// low.js: the Low data view's whole client half (plan 04-18, D-13, D-14, D-16,
// D-20, R-09). It lists Active reports as plain text rows with no map, from the
// last known position stored on this device first, then from the fresh one.
//
// It sends no request of its own. The list comes only from the shared store
// (Pinalert.fetchReports in app.js), so a refetch that votes.js triggers after a
// vote uses the same position this page set. It exports nothing.
//
// SECURITY (T-01-03, T-04-01): every report authored or server authored string
// reaches the DOM through Pinalert.setText or the shared builders, never a
// markup string a browser then parses.
(function () {
  'use strict';

  var listEl = document.getElementById('low-list');
  if (!listEl) {
    // Inert on any page that is not the Low data view.
    return;
  }

  var STORED_POSITION_KEY = 'pinalert.lowLastPosition';
  var LIST_POSITION_TIMEOUT_MS = 8000;
  var TIME_REFRESH_MS = 60000;

  var BASIS_FRESH = 'Near your position.';
  var BASIS_STORED_PENDING = 'Near your last known position. Finding your position.';
  var BASIS_STORED_OFF = 'Near your last known position. Location is off, so it may be out of date.';
  var BASIS_PENDING = 'Finding your position.';
  var BASIS_DEFAULT_AREA = 'Location is off, so this list shows reports near the default area. Turn on location access to see reports near you.';

  var EMPTY_OWN_TITLE = 'No reports near you yet';
  var EMPTY_OWN_TEXT = 'Nothing has been reported in your area recently. Use Report an emergency above to add one.';
  var EMPTY_AREA_TITLE = 'No reports in this area yet';
  var EMPTY_AREA_TEXT = 'Turn on location access to check near you, or use Report an emergency above to add one.';

  var STALE_PREFIX = 'Last updated ';
  var STALE_SUFFIX = '. Trying again.';
  var STALE_ENTER_PREFIX = 'Connection lost. Showing reports from ';
  var STALE_LEAVE_MESSAGE = 'Back online. The list is up to date.';

  var SEVERITY_RANK = { critical: 3, medium: 2, low: 1 };

  // The report form's location block (D-15, D-16). A fix at or under this many
  // metres counts as accurate; a worse one needs the person to accept it. The
  // position request gives up after this many milliseconds.
  var GOOD_ACCURACY_METRES = 100;
  var LOCATION_TIMEOUT_MS = 8000;

  // The eight states of the location block, in the words of the contract.
  // {ACCURACY} and {PLACE} are filled with split and join, so the filled text
  // is never read as a pattern.
  var LOCATION_STATE_ASKING = 'Finding your position. Your browser may ask to share it. Allow it so this report is placed correctly.';
  var LOCATION_STATE_FOUND = 'Position found. Accurate to about {ACCURACY}.';
  var LOCATION_STATE_WEAK = 'Position found, but only accurate to about {ACCURACY}. You can use it, or type a place for a better match.';
  var LOCATION_STATE_ACCEPTED = 'Using this position, accurate to about {ACCURACY}.';
  var LOCATION_STATE_DENIED = 'Location is off for this site. Type a place or address below, or turn on location access in your browser settings and then use Retry position.';
  var LOCATION_STATE_TIMED_OUT = "Couldn't get your position in time. Retry position, or type a place or address below.";
  var LOCATION_STATE_TYPED = 'Using {PLACE}. The report will be placed there, not at your position.';
  var LOCATION_STATE_BOTH_FAILED = 'Posting from here needs a location. Open the full version to place your report on the map.';

  // What Post report says when the form has no usable location yet.
  var LOW_LOCATION_MISSING_MESSAGE = 'Set a location by allowing location access or typing a place.';
  var LOW_LOCATION_WEAK_MESSAGE = 'Choose to use this position, or type a place.';
  var LOW_LOCATION_PENDING_MESSAGE = 'Still finding your position. Wait a moment, or type a place.';
  var LOW_SEARCH_UNAVAILABLE_MESSAGE = 'Search unavailable. Try your position again, or open the full version.';

  var basisEl = document.getElementById('low-basis');
  var retryPositionBtn = document.getElementById('low-retry-position');
  var staleEl = document.getElementById('low-stale');
  var staleTextEl = document.getElementById('low-stale-text');
  var staleRetryBtn = document.getElementById('low-stale-retry');
  var skeletonEl = document.getElementById('low-skeleton');
  var emptyEl = document.getElementById('low-empty');
  var emptyTitleEl = document.getElementById('low-empty-title');
  var emptyTextEl = document.getElementById('low-empty-text');
  var errorEl = document.getElementById('low-error');
  var retryBtn = document.getElementById('low-retry');
  var fullLink = document.getElementById('low-full');
  var noticeLiveEl = document.getElementById('notice-live');

  // rowsById maps a report id to the row object createLowRow returned.
  var rowsById = {};
  // lowLocation is the report form's location object, built in boot when the
  // page has the form.
  var lowLocation = null;
  var basisKind = '';
  var haveStored = false;
  var staleShown = false;
  // listChain orders this page's own list requests: each one starts after the
  // previous one settles, so a slow answer for the stored position can never
  // land after the answer for the fresh position.
  var listChain = Promise.resolve();

  function readStoredPosition() {
    var raw = null;
    try {
      raw = window.localStorage.getItem(STORED_POSITION_KEY);
    } catch (err) {
      // Private browsing can throw. An unreadable store counts as absent.
      return null;
    }
    if (!raw) {
      return null;
    }
    var parsed;
    try {
      parsed = JSON.parse(raw);
    } catch (err) {
      return null;
    }
    if (!parsed || typeof parsed.lat !== 'number' || typeof parsed.lon !== 'number' ||
        !isFinite(parsed.lat) || !isFinite(parsed.lon) ||
        Math.abs(parsed.lat) > 90 || Math.abs(parsed.lon) > 180) {
      return null;
    }
    return { lat: parsed.lat, lon: parsed.lon };
  }

  function storePosition(lat, lon) {
    try {
      window.localStorage.setItem(STORED_POSITION_KEY, JSON.stringify({ lat: lat, lon: lon, savedAt: Date.now() }));
    } catch (err) {
      // Nothing to do: the list still works without a remembered position.
    }
  }

  function setHidden(el, hidden) {
    if (!el) {
      return;
    }
    if (hidden) {
      el.setAttribute('hidden', '');
    } else {
      el.removeAttribute('hidden');
    }
  }

  function announce(text) {
    if (!noticeLiveEl) {
      return;
    }
    Pinalert.setText(noticeLiveEl, text);
  }

  function setBasis(kind) {
    basisKind = kind;
    var text = '';
    if (kind === 'fresh') {
      text = BASIS_FRESH;
    } else if (kind === 'stored') {
      text = BASIS_STORED_PENDING;
    } else if (kind === 'stored-off') {
      text = BASIS_STORED_OFF;
    } else if (kind === 'pending') {
      text = BASIS_PENDING;
    } else if (kind === 'default') {
      text = BASIS_DEFAULT_AREA;
    }
    // The polite region announces only when its text changes.
    if (basisEl && basisEl.textContent !== text) {
      Pinalert.setText(basisEl, text);
    }
    setHidden(retryPositionBtn, kind !== 'default');
  }

  function showListFor(lat, lon, own) {
    Pinalert.setCenter(lat, lon, own);
    listChain = listChain.then(function () {
      return Pinalert.fetchReports();
    });
    Pinalert.startPolling();
  }

  function onPositionFound(pos) {
    if (retryPositionBtn) {
      retryPositionBtn.disabled = false;
    }
    var coords = pos && pos.coords;
    if (!coords || !isFinite(coords.latitude) || !isFinite(coords.longitude)) {
      onPositionFailed();
      return;
    }
    storePosition(coords.latitude, coords.longitude);
    setBasis('fresh');
    showListFor(coords.latitude, coords.longitude, true);
  }

  function onPositionFailed() {
    if (retryPositionBtn) {
      retryPositionBtn.disabled = false;
    }
    if (haveStored) {
      setBasis('stored-off');
      return;
    }
    setBasis('default');
    showListFor(Pinalert.config.fallbackLat, Pinalert.config.fallbackLon, false);
  }

  function requestPosition() {
    if (!navigator.geolocation) {
      onPositionFailed();
      return;
    }
    navigator.geolocation.getCurrentPosition(onPositionFound, onPositionFailed, {
      timeout: LIST_POSITION_TIMEOUT_MS
    });
  }

  function severityRank(report) {
    return SEVERITY_RANK[report && report.severity] || 0;
  }

  // sortLowReports returns a new array: severity first, newest first within a
  // band. It never changes the store's own array.
  function sortLowReports(reports) {
    var copy = reports.slice();
    copy.sort(function (a, b) {
      var rankDiff = severityRank(b) - severityRank(a);
      if (rankDiff !== 0) {
        return rankDiff;
      }
      return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
    });
    return copy;
  }

  function buildLowMetaText(report) {
    var text = Pinalert.categoryLabel(report.category) + ' · ' + Pinalert.relativeTime(report.created_at);
    var shelterLine = Pinalert.formatShelterLine(report);
    if (shelterLine) {
      text += ' · ' + shelterLine;
    }
    return text;
  }

  function replacePrefixedClass(el, prefix, newClass) {
    var kept = [];
    var current = el.className ? el.className.split(/\s+/) : [];
    for (var i = 0; i < current.length; i++) {
      if (current[i] && current[i].indexOf(prefix) !== 0) {
        kept.push(current[i]);
      }
    }
    kept.push(newClass);
    el.className = kept.join(' ');
  }

  // createLowRow builds one row from the shared builders. The row is plain
  // text: it takes no focus and answers no tap. Its one control is the trust
  // block's own toggle.
  function createLowRow(report) {
    var li = document.createElement('li');
    li.className = 'report-row report-row--low';
    li.setAttribute('role', 'listitem');
    li.dataset.reportId = String(report.id);

    var badge = document.createElement('span');
    badge.className = 'icon-badge icon-badge--sm';
    var glyph = document.createElement('span');
    glyph.className = 'icon-glyph';
    glyph.setAttribute('aria-hidden', 'true');
    badge.appendChild(glyph);

    var body = document.createElement('div');
    body.className = 'report-row__body';

    var severityLabel = PinalertTrust.createSeverityLabel();
    body.appendChild(severityLabel);

    var title = document.createElement('p');
    title.className = 'report-row__title';
    body.appendChild(title);

    var meta = document.createElement('p');
    meta.className = 'report-row__meta';
    body.appendChild(meta);

    // The Written line shows only when the server sent a written time; it
    // ships hidden and updateLowWrittenLine fills it.
    var written = document.createElement('p');
    written.className = 'report-row__meta';
    written.hidden = true;
    body.appendChild(written);

    var tag = PinalertVisibility.createVisibilityTag();
    var trust = PinalertTrust.createTrustBlock();
    body.appendChild(PinalertTrust.createStateLine('report-row__stateline', tag, trust));
    body.appendChild(trust.el);

    // Confirm and Dispute come from the shared vote block, built without the
    // resolve button and its question. The block owns its own listeners and sits under the trust
    // block in the order the opened report uses: controls, the too far note,
    // then the vote error.
    var voteBlock = PinalertVotes.createVoteBlock(report.id, { resolve: false });
    body.appendChild(voteBlock.controls);
    body.appendChild(voteBlock.note);
    body.appendChild(voteBlock.error);

    li.appendChild(badge);
    li.appendChild(body);

    return {
      el: li,
      glyph: glyph,
      severity: severityLabel,
      title: title,
      meta: meta,
      written: written,
      tag: tag,
      trust: trust,
      voteBlock: voteBlock
    };
  }

  // updateLowWrittenLine fills the Written line from the shared formatter, which
  // is the only reader of the server's written time, and hides it when empty.
  function updateLowWrittenLine(row, report) {
    var line = Pinalert.writtenLine(report);
    Pinalert.setText(row.written, line);
    row.written.hidden = !line;
  }

  function updateLowRow(row, report) {
    replacePrefixedClass(row.el, 'sev-', Pinalert.severityClass(report));
    PinalertTrust.updateSeverityLabel(row.severity, report);
    replacePrefixedClass(row.el, 'age-', 'age-' + PinalertTrust.fadeStage(report));
    PinalertVisibility.applyVisibilityClass(row.el, report);

    // Pinalert.iconClass returns the base class and the category class joined;
    // only the category class (the last field) is swapped, so the base class
    // set once in createLowRow is never added twice.
    var glyphClass = Pinalert.iconClass(report.category).split(/\s+/).pop();
    replacePrefixedClass(row.glyph, 'icon-glyph--', glyphClass);

    Pinalert.setText(row.title, report.description);
    Pinalert.setText(row.meta, buildLowMetaText(report));
    updateLowWrittenLine(row, report);

    PinalertVisibility.updateVisibilityTag(row.tag, report);
    PinalertTrust.updateTrustBlock(row.trust, report.trust);
    PinalertVotes.updateVoteBlock(row.voteBlock, report);
  }

  // focusAfterRemoval puts focus on a real control when the row that held it
  // is gone: the trust toggle of the row now at that index, else of the row
  // before it, else the Report button when the page has one, else the Open
  // full version link.
  function focusAfterRemoval(index) {
    var rows = listEl.children;
    if (index >= 0 && index < rows.length && PinalertTrust.focusToggle(rows[index])) {
      return;
    }
    if (index - 1 >= 0 && index - 1 < rows.length && PinalertTrust.focusToggle(rows[index - 1])) {
      return;
    }
    var reportBtn = document.getElementById('low-report');
    if (reportBtn) {
      reportBtn.focus();
      return;
    }
    if (fullLink) {
      fullLink.focus();
    }
  }

  function renderLowRows(reports) {
    var sorted = sortLowReports(reports);
    var seen = {};
    for (var s = 0; s < sorted.length; s++) {
      seen[String(sorted[s].id)] = true;
    }

    var active = document.activeElement;
    var removedFocusIndex = -1;
    var existingIds = Object.keys(rowsById);
    for (var j = 0; j < existingIds.length; j++) {
      var rid = existingIds[j];
      if (seen[rid]) {
        continue;
      }
      var gone = rowsById[rid].el;
      if (active && gone.contains(active)) {
        removedFocusIndex = Array.prototype.indexOf.call(listEl.children, gone);
      }
      gone.remove();
      delete rowsById[rid];
    }

    var activeBefore = document.activeElement;
    var movedFocus = false;
    for (var i = 0; i < sorted.length; i++) {
      var report = sorted[i];
      var idKey = String(report.id);
      var row = rowsById[idKey];
      if (!row) {
        row = createLowRow(report);
        rowsById[idKey] = row;
      }
      updateLowRow(row, report);

      var expected = listEl.children[i];
      if (expected !== row.el) {
        if (row.el.parentNode === listEl && typeof listEl.moveBefore === 'function') {
          listEl.moveBefore(row.el, expected || null);
        } else {
          if (row.el.parentNode === listEl && row.el.contains(activeBefore)) {
            movedFocus = true;
          }
          listEl.insertBefore(row.el, expected || null);
        }
      }
    }

    if (movedFocus && activeBefore && activeBefore.isConnected &&
        (!document.activeElement || document.activeElement === document.body)) {
      activeBefore.focus();
    }
    if (removedFocusIndex !== -1) {
      focusAfterRemoval(removedFocusIndex);
    }
  }

  function clearLowRows() {
    var hadFocus = listEl.contains(document.activeElement);
    var ids = Object.keys(rowsById);
    for (var i = 0; i < ids.length; i++) {
      rowsById[ids[i]].el.remove();
      delete rowsById[ids[i]];
    }
    if (hadFocus) {
      focusAfterRemoval(0);
    }
  }

  function setEmpty(show) {
    setHidden(emptyEl, !show);
    if (!show) {
      return;
    }
    if (basisKind === 'fresh') {
      Pinalert.setText(emptyTitleEl, EMPTY_OWN_TITLE);
      Pinalert.setText(emptyTextEl, EMPTY_OWN_TEXT);
    } else {
      Pinalert.setText(emptyTitleEl, EMPTY_AREA_TITLE);
      Pinalert.setText(emptyTextEl, EMPTY_AREA_TEXT);
    }
  }

  function staleLineText(state) {
    return STALE_PREFIX + Pinalert.updatedAgo(state.lastUpdatedAt) + STALE_SUFFIX;
  }

  // renderStale shows the stale line while the last poll failed after a list
  // had loaded, and takes it away when a later poll succeeds. It announces once
  // when the line appears and once when it goes. The line itself is passive
  // text; the page level region speaks.
  function renderStale(state) {
    var stale = state.hasLoaded === true && state.status === 'error';
    if (stale) {
      Pinalert.setText(staleTextEl, staleLineText(state));
      setHidden(staleEl, false);
      if (staleRetryBtn) {
        staleRetryBtn.disabled = false;
      }
      if (!staleShown) {
        announce(STALE_ENTER_PREFIX + Pinalert.updatedAgo(state.lastUpdatedAt) + '.');
      }
    } else if (staleShown && state.status === 'ok') {
      if (staleRetryBtn && document.activeElement === staleRetryBtn) {
        var firstRow = listEl.querySelector('.report-row');
        if (!firstRow || !PinalertTrust.focusToggle(firstRow)) {
          if (fullLink) {
            fullLink.focus();
          }
        }
      }
      setHidden(staleEl, true);
      if (staleRetryBtn) {
        staleRetryBtn.disabled = false;
      }
      announce(STALE_LEAVE_MESSAGE);
    } else {
      setHidden(staleEl, true);
    }
    staleShown = stale;
  }

  // renderLow is the one place that decides which of the list, the skeleton,
  // the empty block, the error panel and the stale line shows. The skeleton and
  // the error panel belong to the first load only: a failed poll after a list
  // has loaded keeps the rows and adds the stale line (D-20).
  function renderLow(state) {
    if (state.hasLoaded !== true) {
      setHidden(staleEl, true);
      staleShown = false;
      setHidden(emptyEl, true);
      setHidden(listEl, true);
      if (state.status === 'error') {
        setHidden(skeletonEl, true);
        setHidden(errorEl, false);
        listEl.setAttribute('aria-busy', 'false');
      } else {
        setHidden(skeletonEl, false);
        setHidden(errorEl, true);
        listEl.setAttribute('aria-busy', 'true');
      }
      return;
    }

    setHidden(skeletonEl, true);
    setHidden(errorEl, true);
    listEl.setAttribute('aria-busy', 'false');
    renderStale(state);

    if (state.reports.length === 0) {
      clearLowRows();
      setHidden(listEl, true);
      setEmpty(true);
      return;
    }
    setEmpty(false);
    setHidden(listEl, false);
    renderLowRows(state.reports);
  }

  function findReportById(idKey) {
    var reports = Pinalert.state.reports;
    for (var i = 0; i < reports.length; i++) {
      if (String(reports[i].id) === idKey) {
        return reports[i];
      }
    }
    return null;
  }

  // refreshLowTimes renews the minute figure of the stale line and each row's
  // time and age class on a timer. It never announces.
  function refreshLowTimes() {
    if (staleShown) {
      Pinalert.setText(staleTextEl, staleLineText(Pinalert.state));
    }
    var ids = Object.keys(rowsById);
    for (var i = 0; i < ids.length; i++) {
      var report = findReportById(ids[i]);
      if (!report) {
        continue;
      }
      var row = rowsById[ids[i]];
      replacePrefixedClass(row.el, 'age-', 'age-' + PinalertTrust.fadeStage(report));
      Pinalert.setText(row.meta, buildLowMetaText(report));
      updateLowWrittenLine(row, report);
    }
  }

  // accuracyPhrase words a position accuracy for the status line: metres
  // rounded to the nearest 10 and at least 10, then kilometres with one
  // decimal from 1000 metres up. The tenths come from whole number arithmetic
  // because a fixed decimal conversion of 1.45 can round down.
  function accuracyPhrase(metres) {
    var rounded = Math.max(10, Math.round(metres / 10) * 10);
    if (rounded < 1000) {
      return rounded + ' metres';
    }
    var km = Math.round(rounded / 100) / 10;
    return String(km) + ' km';
  }

  // lowSearchFailureMessage is the location object's answer to a failed place
  // search. It takes no argument on purpose: the server's own text is never
  // shown on this page, only the Low data sentence (U8).
  function lowSearchFailureMessage() {
    return LOW_SEARCH_UNAVAILABLE_MESSAGE;
  }

  // createLowLocation builds the map free location object the shared report
  // form asks for. It holds one of eight states (asking, found, weak,
  // accepted, denied, timedout, typed, bothfailed) and writes the sentence of
  // the state into the status line of the location block. The exact fix is
  // kept in memory for the open form only and is never stored.
  function createLowLocation() {
    var statusEl = document.getElementById('low-location-status');
    var actionsEl = document.querySelector('.low-location__actions');
    var useBtn = document.getElementById('low-location-use');
    var ownBtn = document.getElementById('low-location-own');
    var locationRetryBtn = document.getElementById('low-location-retry');
    var locationFullLink = document.getElementById('low-location-full');
    var submitBtn = document.getElementById('submit-report');

    var state = 'asking';
    var fix = null;
    var typed = null;
    var searchFailed = false;
    var positionSeq = 0;
    var markTouched = null;

    function touch() {
      if (markTouched) {
        markTouched();
      }
    }

    function fillAccuracy(sentence) {
      return fix ? sentence.split('{ACCURACY}').join(accuracyPhrase(fix.accuracy)) : sentence;
    }

    function fillPlace(sentence) {
      return typed ? sentence.split('{PLACE}').join(typed.displayName) : sentence;
    }

    function sentenceFor() {
      if (state === 'asking') {
        return LOCATION_STATE_ASKING;
      }
      if (state === 'found') {
        return fillAccuracy(LOCATION_STATE_FOUND);
      }
      if (state === 'weak') {
        return fillAccuracy(LOCATION_STATE_WEAK);
      }
      if (state === 'accepted') {
        return fillAccuracy(LOCATION_STATE_ACCEPTED);
      }
      if (state === 'denied') {
        return LOCATION_STATE_DENIED;
      }
      if (state === 'timedout') {
        return LOCATION_STATE_TIMED_OUT;
      }
      if (state === 'typed') {
        return fillPlace(LOCATION_STATE_TYPED);
      }
      return LOCATION_STATE_BOTH_FAILED;
    }

    // clearSubmitMark takes the unavailable look off Post report.
    function clearSubmitMark() {
      if (submitBtn) {
        submitBtn.removeAttribute('aria-disabled');
        submitBtn.removeAttribute('aria-describedby');
      }
    }

    function render() {
      var text = sentenceFor();
      // The status line is a live region: writing the same text again could
      // be read out a second time, so only a changed sentence is written.
      if (statusEl && statusEl.textContent !== text) {
        Pinalert.setText(statusEl, text);
      }
      var showUse = state === 'weak';
      var showOwn = state === 'typed' && fix !== null;
      var showRetry = state === 'denied' || state === 'timedout' || state === 'bothfailed';
      var showFull = state === 'bothfailed';
      setHidden(useBtn, !showUse);
      setHidden(ownBtn, !showOwn);
      setHidden(locationRetryBtn, !showRetry);
      setHidden(locationFullLink, !showFull);
      setHidden(actionsEl, !(showUse || showOwn || showRetry || showFull));
      if (submitBtn && state === 'bothfailed') {
        submitBtn.setAttribute('aria-disabled', 'true');
        submitBtn.setAttribute('aria-describedby', 'low-location-status');
      } else {
        clearSubmitMark();
      }
    }

    // A control that was just pressed may have gone away with the state it
    // belonged to. Focus then goes to the category choice, the next thing the
    // form needs, so it never falls to the page behind.
    function keepFocus(pressed) {
      if (!pressed || !pressed.hidden) {
        return;
      }
      var next = document.querySelector('#category-grid .category-tile--selected') ||
        document.querySelector('#category-grid .category-tile');
      if (next) {
        next.focus();
      }
    }

    function onFail(seq, err) {
      if (seq !== positionSeq) {
        return;
      }
      if (state === 'typed') {
        render();
        return;
      }
      fix = null;
      if (searchFailed) {
        state = 'bothfailed';
      } else if (err && err.code === err.PERMISSION_DENIED) {
        state = 'denied';
      } else {
        // The UI-SPEC gives no separate sentence for an unavailable position;
        // Timed out offers the same two ways forward.
        state = 'timedout';
      }
      render();
    }

    function onFix(seq, pos) {
      if (seq !== positionSeq) {
        return;
      }
      var coords = pos && pos.coords;
      if (!coords || !isFinite(coords.latitude) || !isFinite(coords.longitude)) {
        onFail(seq, { code: 2, PERMISSION_DENIED: 1 });
        return;
      }
      var accuracy = isFinite(coords.accuracy) ? coords.accuracy : GOOD_ACCURACY_METRES + 1;
      fix = { latitude: coords.latitude, longitude: coords.longitude, accuracy: accuracy };
      if (state !== 'typed') {
        state = accuracy <= GOOD_ACCURACY_METRES ? 'found' : 'weak';
      }
      render();
    }

    // requestFix asks for the exact position. Each request gets a number and a
    // late answer to an older request is ignored.
    function requestFix() {
      positionSeq += 1;
      var seq = positionSeq;
      if (!navigator.geolocation) {
        onFail(seq, { code: 1, PERMISSION_DENIED: 1 });
        return;
      }
      navigator.geolocation.getCurrentPosition(function (pos) {
        onFix(seq, pos);
      }, function (err) {
        onFail(seq, err);
      }, {
        enableHighAccuracy: true,
        timeout: LOCATION_TIMEOUT_MS,
        maximumAge: 0
      });
    }

    function start(hooks) {
      markTouched = hooks && typeof hooks.markTouched === 'function' ? hooks.markTouched : null;
      state = 'asking';
      fix = null;
      typed = null;
      searchFailed = false;
      render();
      requestFix();
    }

    function reset() {
      positionSeq += 1;
      state = 'asking';
      fix = null;
      typed = null;
      searchFailed = false;
      markTouched = null;
      if (statusEl) {
        Pinalert.setText(statusEl, '');
      }
      setHidden(useBtn, true);
      setHidden(ownBtn, true);
      setHidden(locationRetryBtn, true);
      setHidden(locationFullLink, true);
      setHidden(actionsEl, true);
      clearSubmitMark();
    }

    function current() {
      if (state === 'typed' && typed) {
        return { latitude: typed.latitude, longitude: typed.longitude };
      }
      if ((state === 'found' || state === 'accepted') && fix) {
        return { latitude: fix.latitude, longitude: fix.longitude };
      }
      return null;
    }

    function choose(result) {
      var latitude = Number(result && result.lat);
      var longitude = Number(result && result.lon);
      if (!isFinite(latitude) || !isFinite(longitude)) {
        return;
      }
      typed = { latitude: latitude, longitude: longitude, displayName: String(result.displayName) };
      state = 'typed';
      render();
    }

    function isSet() {
      return current() !== null;
    }

    function missingMessage() {
      if (state === 'asking') {
        return LOW_LOCATION_PENDING_MESSAGE;
      }
      if (state === 'weak') {
        return LOW_LOCATION_WEAK_MESSAGE;
      }
      return LOW_LOCATION_MISSING_MESSAGE;
    }

    // noteSearchOutcome hears how the last place search ended. A failed or
    // empty search while the position is also unavailable means both ways of
    // placing the report have failed.
    function noteSearchOutcome(outcome) {
      if (outcome === 'no_match' || outcome === 'unavailable') {
        searchFailed = true;
        if (state === 'denied' || state === 'timedout') {
          state = 'bothfailed';
          render();
        }
      } else if (outcome === 'match') {
        searchFailed = false;
      }
    }

    function onUse() {
      touch();
      if (state === 'weak') {
        state = 'accepted';
        render();
      }
      keepFocus(useBtn);
    }

    function onOwn() {
      touch();
      typed = null;
      if (fix) {
        state = fix.accuracy <= GOOD_ACCURACY_METRES ? 'found' : 'accepted';
      } else {
        state = 'asking';
      }
      render();
      keepFocus(ownBtn);
    }

    // Retry asks again without clearing a typed place or a failed search.
    function onRetry() {
      touch();
      if (state !== 'typed') {
        state = 'asking';
        fix = null;
        render();
      }
      requestFix();
      keepFocus(locationRetryBtn);
    }

    if (useBtn) {
      useBtn.addEventListener('click', onUse);
    }
    if (ownBtn) {
      ownBtn.addEventListener('click', onOwn);
    }
    if (locationRetryBtn) {
      locationRetryBtn.addEventListener('click', onRetry);
    }

    return {
      current: current,
      choose: choose,
      isSet: isSet,
      missingMessage: missingMessage,
      searchFailureMessage: lowSearchFailureMessage,
      noteSearchOutcome: noteSearchOutcome,
      start: start,
      reset: reset
    };
  }

  function boot() {
    Pinalert.subscribe(renderLow);

    var stored = readStoredPosition();
    if (stored) {
      Pinalert.setCenter(stored.lat, stored.lon, true);
    } else {
      Pinalert.setCenter(Pinalert.config.fallbackLat, Pinalert.config.fallbackLon, false);
    }
    // The Active view is the store's own start state and this page never reads
    // the address, so nothing sets another view here.
    renderLow(Pinalert.state);

    if (stored) {
      haveStored = true;
      setBasis('stored');
      showListFor(stored.lat, stored.lon, true);
    } else {
      setBasis('pending');
    }
    requestPosition();

    if (retryPositionBtn) {
      retryPositionBtn.addEventListener('click', function () {
        retryPositionBtn.disabled = true;
        requestPosition();
      });
    }
    if (staleRetryBtn) {
      staleRetryBtn.addEventListener('click', function () {
        staleRetryBtn.disabled = true;
        Pinalert.fetchReports();
      });
    }
    if (retryBtn) {
      retryBtn.addEventListener('click', function () {
        Pinalert.fetchReports();
      });
    }

    window.setInterval(refreshLowTimes, TIME_REFRESH_MS);

    // A page restored from the back forward cache is a frozen snapshot, so it
    // refetches at once instead of waiting for the next poll.
    window.addEventListener('pageshow', function (evt) {
      if (evt.persisted) {
        Pinalert.fetchReports();
      }
    });

    // The report form is the shared one, mounted once with the map free
    // location object. A page without the form or without the shared script
    // keeps a working list and an inert Report button.
    var reportModal = document.getElementById('report-modal');
    var reportBtn = document.getElementById('low-report');
    if (reportModal && reportBtn && window.PinalertReportForm) {
      lowLocation = createLowLocation();
      PinalertReportForm.mount({
        backdrop: reportModal,
        form: reportModal.querySelector('.modal-panel'),
        location: lowLocation,
        variant: 'low'
      });
      reportBtn.addEventListener('click', function () {
        PinalertReportForm.open({ opener: reportBtn });
      });
    }
  }

  boot();
}());
