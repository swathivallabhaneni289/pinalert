// feed.js — the report list and its linkage to the map.
//
// Renders window.Pinalert.state.reports into #report-list. Performs no
// fetch of its own: the map (map.js) and this list read from the exact
// same array the shared store (app.js) fetches, so the two panels cannot
// drift apart (D-06).
//
// SECURITY (T-01-03): report descriptions are authored by anonymous
// strangers and rendered in every other visitor's browser. Every piece of
// report-authored data reaches the DOM through Pinalert.setText (a
// text-node assignment) or an equivalent property, never through a markup
// string a browser then parses. html/template protects only the
// server-rendered shell (index.html.tmpl); this file is the whole defence
// for anything built here afterward.
//
// SCOPE (T-01-23, TRUST-02): this list now renders the confirm/dispute
// controls, the viewer's own standing vote, and the resolver's own
// visibility state — in words (the visibility tag) and in the row
// treatment (the state class) — but it also mounts the shared trust block
// through PinalertTrust. That block renders the server's place counts,
// signal words and panel sentences exactly as sent, and this file computes
// none of them: a count here is never typed, capped or bumped by the browser.
(function () {
  'use strict';

  var SEVERITY_RANK = { critical: 3, medium: 2, low: 1 };

  var SVG_NS = 'http://www.w3.org/2000/svg';

  // VIEW_PARAM is the page URL's own name for the tab, deliberately the
  // exact same word app.js's fetchReports sends to GET /api/reports, so the
  // page and the server share one vocabulary for this idea. The address
  // never carries the default tab: Active is the absence of the parameter.
  var VIEW_PARAM = 'view';

  // OLD_BOOKMARK_PARAM is the address parameter of the old Show disputed
  // checkbox. It is read only, so a saved bookmark still opens Disputed, and
  // it is rewritten once to the new parameter (03.1 D-04). Nothing sends it
  // to the server any more.
  var OLD_BOOKMARK_PARAM = 'show_disputed';

  // REPORT_PARAM is the address parameter of a link to one report. It is read
  // once on load and never removed from the address.
  var REPORT_PARAM = 'report';

  var reportListEl = document.getElementById('report-list');
  var skeletonEl = document.getElementById('feed-skeleton');
  var emptyEl = document.getElementById('feed-empty');
  var disputedEmptyEl = document.getElementById('disputed-empty');
  var errorEl = document.getElementById('feed-error');
  var retryBtn = document.getElementById('feed-retry');
  var viewToggleBtn = document.getElementById('view-toggle');
  var appShellEl = document.getElementById('app-shell');
  var tabStripEl = document.querySelector('.view-tabs');
  var feedCountEl = document.getElementById('feed-count');
  var feedStatusEl = document.getElementById('feed-status');
  var feedHeadingTitleEl = document.getElementById('feed-heading-title');
  var feedNoteEl = document.getElementById('feed-note');
  var pastGroupsEl = document.getElementById('past-groups');
  var historyEmptyEl = document.getElementById('feed-empty-history');
  var pastEmptyEl = document.getElementById('past-empty');
  var filterEmptyEl = document.getElementById('filter-empty');
  var filterClearBtn = document.getElementById('filter-empty-clear');
  var seePastBtn = document.getElementById('see-past-reports');

  // The heading and the one line note under it, per tab (C3 to C7). Active
  // has no note. The strings are fixed here, never read from the server.
  var VIEW_HEADINGS = { active: 'Nearby now', disputed: 'Disputed reports', past: 'Past reports' };
  var VIEW_NOTES = {
    active: '',
    disputed: 'Nearby people disputed these. You can still confirm or dispute them.',
    past: 'Resolved and expired reports from the last 7 days.'
  };

  // lastAnnouncedKey is the tab, place and chip the status line last
  // announced, and statusStarted is false until the first answer lands. The
  // first answer is not announced: a reader who just opened the page hears
  // the list itself, not a status line over it.
  var lastAnnouncedKey = '';
  var statusStarted = false;

  // PHONE_QUERY is the same 899.98px complement of the 900px query that
  // main.css, trust.css and phone.css use. Below it the list is its own scroll
  // box under a fixed height map (phone.css), so moving the list never moves
  // the map.
  var PHONE_QUERY = '(max-width: 899.98px)';

  // rowsById reconciles rendered <li> nodes by report id across polls,
  // rather than clearing and rebuilding the list every 30 seconds, so
  // scroll position and focus survive a background refresh.
  var rowsById = {};

  function severityRank(report) {
    return SEVERITY_RANK[report && report.severity] || 0;
  }

  // sortReports returns a new array — D-08 ordering (critical, then
  // medium, then low; newest first within a band) must never mutate the
  // shared store's array, since the map renders from that same reference.
  function sortReports(reports) {
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

  // setEmptyState is the sole owner of the five empty-state elements. render
  // is documented as the one place that decides which of the list, skeleton,
  // empty state and error state is showing, and more elements would
  // otherwise turn four explicit branches into many. Every setHidden call
  // targeting an empty-state element lives in this function and nowhere
  // else, and at most one of them shows. kind is one of 'none' (none shown),
  // 'general' (the Phase 1 empty state), 'history' (nothing active but
  // recent history, with the See past reports button), 'disputed' (the
  // Disputed tab's own empty state), 'past' (the Past tab's) or 'filter' (a
  // chip emptied the list, with the Clear filter button).
  function setEmptyState(kind) {
    setHidden(emptyEl, kind !== 'general');
    setHidden(historyEmptyEl, kind !== 'history');
    setHidden(disputedEmptyEl, kind !== 'disputed');
    setHidden(pastEmptyEl, kind !== 'past');
    setHidden(filterEmptyEl, kind !== 'filter');
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

  function buildMetaText(report) {
    var categoryLabel = Pinalert.categoryLabel(report.category);
    var text = categoryLabel + ' · ' + Pinalert.relativeTime(report.created_at);

    var shelterLine = Pinalert.formatShelterLine(report);
    if (shelterLine) {
      text += ' · ' + shelterLine;
    }

    return text;
  }

  function activateRow(id) {
    Pinalert.select(id, 'list');
    applySelectionHighlight(id);
    if (window.PinalertMap && typeof PinalertMap.flyTo === 'function') {
      PinalertMap.flyTo(id, { opener: 'list' });
    }
    if (appShellEl && appShellEl.getAttribute('data-view') === 'list') {
      switchView('map');
    }
  }

  function createRow(id) {
    var li = document.createElement('li');
    li.className = 'report-row';
    li.setAttribute('tabindex', '0');
    li.setAttribute('role', 'listitem');
    li.dataset.reportId = String(id);

    var badge = document.createElement('span');
    badge.className = 'icon-badge icon-badge--sm';
    var glyph = document.createElement('span');
    glyph.className = 'icon-glyph';
    glyph.setAttribute('aria-hidden', 'true');
    badge.appendChild(glyph);

    var body = document.createElement('div');
    body.className = 'report-row__body';

    var title = document.createElement('p');
    title.className = 'report-row__title';

    var meta = document.createElement('p');
    meta.className = 'report-row__meta';

    // The distance line, drawn on phones only (phone.css hides it at 900px
    // and up). It is filled by updateRow and stays hidden when the feed is
    // not around the reader's own position (A94).
    var away = document.createElement('p');
    away.className = 'report-row__away';
    away.hidden = true;

    // A26, A45: the visible severity word is the first text in the row body,
    // read before the title. The shipped row had no severity word, so nothing
    // is removed.
    var severityLabel = PinalertTrust.createSeverityLabel();
    body.appendChild(severityLabel);
    body.appendChild(title);
    body.appendChild(meta);
    body.appendChild(away);

    // The state label is built here and placed, with the reporter words, on
    // the state line under the meta line (A75, A81). The tag ships hidden from
    // its own builder.
    var tag = PinalertVisibility.createVisibilityTag();

    // The shared trust block (D-20), built with no options so the row shows
    // no numbers beyond the two counters (D-09). It sits between the state
    // line and the vote controls.
    var trust = PinalertTrust.createTrustBlock();
    body.appendChild(PinalertTrust.createStateLine('report-row__stateline', tag, trust));
    body.appendChild(trust.el);

    var block = PinalertVotes.createVoteBlock(id);
    body.appendChild(block.controls);
    // The too far sentence slot sits between the buttons and the error and
    // is owned by votes.js.
    body.appendChild(block.note);
    body.appendChild(block.error);

    li.appendChild(badge);
    li.appendChild(body);

    li.addEventListener('click', function () {
      activateRow(id);
    });
    li.addEventListener('keydown', function (evt) {
      if (evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar') {
        evt.preventDefault();
        activateRow(id);
      }
    });

    return { el: li, glyph: glyph, severity: severityLabel, title: title, meta: meta, away: away, votes: block, tag: tag, trust: trust };
  }

  // updateRow applies severity/age classes and text content only — it never
  // touches layout, so a background refresh cannot disturb scroll position
  // on rows whose identity (id) is unchanged.
  function updateRow(row, report) {
    replacePrefixedClass(row.el, 'sev-', Pinalert.severityClass(report));
    PinalertTrust.updateSeverityLabel(row.severity, report);
    replacePrefixedClass(row.el, 'age-', 'age-' + PinalertTrust.fadeStage(report));

    // The visibility state class goes on the row element (inherits down to
    // its badge), not the badge itself — main.css's own .icon-badge
    // comment is the authority for this split. refreshAgeAndTime below is
    // deliberately left untouched: it re-applies only the age class, which
    // strips nothing this class added, and the cascade order that makes
    // the Provisional treatment win is a property of stylesheet load
    // order rather than of class-list order — so a 60-second age refresh
    // cannot un-dim a provisional row.
    PinalertVisibility.applyVisibilityClass(row.el, report);

    // Pinalert.iconClass returns the base "icon-glyph" class AND the
    // category-specific class as one space-joined string; only the
    // category-specific (last) field is passed to replacePrefixedClass so
    // the base class — already set once in createRow — is never re-pushed
    // on every poll, which would otherwise accumulate a duplicate copy.
    var glyphClass = Pinalert.iconClass(report.category).split(/\s+/).pop();
    replacePrefixedClass(row.glyph, 'icon-glyph--', glyphClass);

    Pinalert.setText(row.title, report.description);
    Pinalert.setText(row.meta, buildMetaText(report));

    // The distance is the reader's own only when the feed is around the
    // reader's position, the same rule the popup follows (A94).
    var away = Pinalert.feedIsAroundReader() ? Pinalert.formatDistance(report.distance_km) : '';
    Pinalert.setText(row.away, away);
    row.away.hidden = !away;

    PinalertVisibility.updateVisibilityTag(row.tag, report);
    PinalertTrust.updateTrustBlock(row.trust, report.trust);

    var selectedId = Pinalert.state.selectedId;
    var isSelected = selectedId !== null && selectedId !== undefined &&
      String(selectedId) === String(report.id);
    row.el.classList.toggle('report-row--selected', isSelected);

    // Same create-once / update-on-poll split this function already
    // follows for everything else: the block's nodes were built once in
    // createRow, and only their state is written here, so a background
    // refresh cannot disturb focus on a button the reader is about to
    // press.
    PinalertVotes.updateVoteBlock(row.votes, report);
  }

  // elementIsShown is true for a connected element that takes part in layout,
  // so a row in a hidden list pane is never a focus candidate.
  function elementIsShown(el) {
    return !!el && el.isConnected && el.getClientRects().length > 0;
  }

  // currentTabLink is the link of the selected tab, or null without a strip.
  function currentTabLink() {
    return tabStripEl ? tabStripEl.querySelector('.view-tab[aria-current="page"]') : null;
  }

  // focusIsLost is true when focus is on the page body or on a node inside a
  // popup that is no longer open. map.js sets data-popup-open when a popup
  // opens and clears it synchronously when it closes, which the poll's render
  // does for a removed report. Leaflet then keeps the closing node attached
  // for a 200 ms fade, so a focused node there is not live although it is
  // attached and is not the body. A popup without the mark is treated as
  // closed. Reading the mark needs no timer, animation event or style.
  function focusIsLost(active) {
    if (!active || active === document.body) {
      return true;
    }
    var popup = active.closest('.leaflet-popup');
    if (popup !== null && popup.getAttribute('data-popup-open') !== 'true') {
      return true;
    }
    return false;
  }

  // takeMapPopupFocus asks the map whether a poll just removed the popup that
  // held focus (A67). The map reconciles first, so the answer is ready.
  function takeMapPopupFocus() {
    return !!(window.PinalertMap && typeof PinalertMap.takeRemovedPopupFocus === 'function' &&
      PinalertMap.takeRemovedPopupFocus());
  }

  // focusAfterRemoval is the one place focus is chosen after a poll removed
  // the report that owned it (A67, in the A52 order): the first displayed
  // row among the candidates, else the map, else the current tab link, else
  // the view toggle, else the empty state. It acts when focus fell to the page
  // body or sits in a popup that map.js already marked closed, and the browser
  // decides only where focus goes, never which reports exist.
  function focusAfterRemoval(candidates) {
    var active = document.activeElement;
    if (!focusIsLost(active)) {
      return;
    }
    for (var i = 0; i < candidates.length; i++) {
      var candidate = candidates[i];
      if (candidate && candidate.classList && candidate.classList.contains('report-row') &&
          elementIsShown(candidate)) {
        candidate.focus();
        return;
      }
    }
    var mapEl = document.getElementById('map');
    if (elementIsShown(mapEl)) {
      mapEl.focus();
      return;
    }
    var tabLink = currentTabLink();
    if (elementIsShown(tabLink)) {
      tabLink.focus();
      return;
    }
    if (elementIsShown(viewToggleBtn)) {
      viewToggleBtn.focus();
      return;
    }
    var empties = [emptyEl, historyEmptyEl, disputedEmptyEl, pastEmptyEl, filterEmptyEl];
    for (var k = 0; k < empties.length; k++) {
      if (elementIsShown(empties[k])) {
        empties[k].setAttribute('tabindex', '-1');
        empties[k].focus();
        return;
      }
    }
  }

  // renderRows keeps focus on a real control through a poll (A67, A71).
  // Departed rows are removed before the rest are placed. A row moves only
  // when its position changed, with moveBefore where the browser has it (it
  // keeps focus and an open panel), and a moved row's focused element is
  // focused again afterwards.
  function renderRows(reports) {
    var sorted = sortReports(reports);
    var seen = {};
    for (var s = 0; s < sorted.length; s++) {
      seen[String(sorted[s].id)] = true;
    }

    var active = document.activeElement;
    var nexts = [];
    var prevs = [];
    var focusNext = null;
    var focusPrev = null;
    var rowHadFocus = false;
    var existingIds = Object.keys(rowsById);
    for (var j = 0; j < existingIds.length; j++) {
      var rid = existingIds[j];
      if (seen[rid]) {
        continue;
      }
      var gone = rowsById[rid].el;
      nexts.push(gone.nextElementSibling);
      prevs.push(gone.previousElementSibling);
      if (active && gone.contains(active)) {
        focusNext = gone.nextElementSibling;
        focusPrev = gone.previousElementSibling;
        rowHadFocus = true;
      }
      gone.remove();
      delete rowsById[rid];
    }

    var popupHadFocus = takeMapPopupFocus();

    var activeBefore = document.activeElement;
    var movedFocus = false;
    for (var i = 0; i < sorted.length; i++) {
      var report = sorted[i];
      var idKey = String(report.id);

      var row = rowsById[idKey];
      if (!row) {
        row = createRow(report.id);
        rowsById[idKey] = row;
      }
      updateRow(row, report);

      var expected = reportListEl.children[i];
      if (expected !== row.el) {
        if (row.el.parentNode === reportListEl && typeof reportListEl.moveBefore === 'function') {
          reportListEl.moveBefore(row.el, expected || null);
        } else {
          if (row.el.parentNode === reportListEl && row.el.contains(activeBefore)) {
            movedFocus = true;
          }
          reportListEl.insertBefore(row.el, expected || null);
        }
      }
    }

    if (movedFocus && activeBefore && activeBefore.isConnected &&
        (!document.activeElement || document.activeElement === document.body)) {
      activeBefore.focus();
    }
    if (rowHadFocus || popupHadFocus) {
      focusAfterRemoval([focusNext, focusPrev].concat(nexts, prevs));
    }
  }

  function clearRows() {
    var ids = Object.keys(rowsById);
    var active = document.activeElement;
    var rowHadFocus = false;
    for (var i = 0; i < ids.length; i++) {
      if (active && rowsById[ids[i]].el.contains(active)) {
        rowHadFocus = true;
      }
    }
    var popupHadFocus = takeMapPopupFocus();
    for (var j = 0; j < ids.length; j++) {
      rowsById[ids[j]].el.remove();
      delete rowsById[ids[j]];
    }
    if (rowHadFocus || popupHadFocus) {
      focusAfterRemoval([]);
    }
  }

  // formatRadius writes the feed radius as a whole number when it is whole and
  // with one decimal otherwise. The number is the one the page carries, never
  // a fixed one (no invented figures).
  function formatRadius(km) {
    if (typeof km !== 'number' || !isFinite(km)) {
      return '';
    }
    return Math.round(km) === km ? String(km) : km.toFixed(1);
  }

  // updateFeedCount writes the count line under the list heading, for
  // example "5 active · 10 km", "2 disputed · 10 km" or
  // "3 reports · 10 km" ("1 report"). The number is the count of
  // rendered reports and the radius is the one the feed was asked for. It is
  // empty while the feed is loading and on an error with no rows to count. It
  // goes through Pinalert.setText, a text node assignment (T-01-03).
  function updateFeedCount(state) {
    if (!feedCountEl) {
      return;
    }
    var text = '';
    var counted = state.status === 'error' ? state.reports.length > 0
      : state.status !== 'idle' && state.status !== 'loading';
    var radius = formatRadius(Pinalert.config.defaultRadiusKm);
    if (counted && radius !== '') {
      var count = state.reports.length;
      var view = Pinalert.state.view;
      var word = view === 'disputed' ? 'disputed' : 'active';
      if (view === 'past') {
        word = count === 1 ? 'report' : 'reports';
      }
      text = count + ' ' + word + ' \u00b7 ' + radius + ' km';
    }
    Pinalert.setText(feedCountEl, text);
  }

  // updateFeedHeading writes the heading and the one line note of the tab
  // (C3 to C7) and marks the shell with the tab, so the laptop heading, which
  // is hidden on Active, shows for Disputed and Past (feed.css). Every string
  // is fixed in this file and goes through Pinalert.setText.
  function updateFeedHeading(view) {
    var heading = VIEW_HEADINGS[view] || VIEW_HEADINGS.active;
    var note = VIEW_NOTES[view] || '';
    if (feedHeadingTitleEl) {
      Pinalert.setText(feedHeadingTitleEl, heading);
    }
    if (feedNoteEl) {
      Pinalert.setText(feedNoteEl, note);
      setHidden(feedNoteEl, note === '');
    }
    if (appShellEl) {
      appShellEl.setAttribute('data-feed-view', view);
    }
  }

  // updateFeedStatus writes the one status sentence of C11 into the visually
  // hidden live region, for example "3 active reports. Near Kochi. Filtered
  // by Flood." It speaks once per tab, place or chip change, when the answer
  // for that change has landed (so the count is the real one). A poll that
  // only changes the count never speaks again, and the first answer of a page
  // load is recorded without speaking. The numbers and words are the
  // store's, nothing is invented.
  function updateFeedStatus(state) {
    if (!feedStatusEl || state.status !== 'ok') {
      return;
    }
    var store = Pinalert.state;
    var placeName = store.place ? store.place.name : '';
    var key = store.view + '|' + placeName + '|' + store.category;
    if (key === lastAnnouncedKey) {
      return;
    }
    lastAnnouncedKey = key;
    if (!statusStarted) {
      statusStarted = true;
      return;
    }
    var count = state.reports.length;
    var text = count + ' ' + store.view + (count === 1 ? ' report.' : ' reports.');
    if (placeName !== '') {
      text += ' Near ' + placeName + '.';
    }
    if (store.category !== '') {
      text += ' Filtered by ' + (Pinalert.CHIP_LABELS[store.category] || Pinalert.categoryLabel(store.category)) + '.';
    }
    Pinalert.setText(feedStatusEl, text);
  }

  // renderPastTab draws the Past tab's list area. The Past tab is two groups
  // of cards, never the live list, so #report-list is hidden and its rows are
  // removed; the group cards themselves are built by the Past cards plan,
  // which fills #past-groups. The groups container shows only when there are
  // reports; with none, the Past empty block (or the filter block when a chip
  // is on) shows instead.
  function renderPastTab(state) {
    setHidden(reportListEl, true);
    clearRows();
    if (state.reports.length === 0) {
      setHidden(pastGroupsEl, true);
      setEmptyState(Pinalert.state.category !== '' ? 'filter' : 'past');
      return;
    }
    setEmptyState('none');
    setHidden(pastGroupsEl, false);
  }

  // render is the single state machine driving the list, the skeleton, the
  // empty state, and the error state — driven entirely by state.status, so
  // there is exactly one place that decides which of the four is showing.
  function render(state) {
    updateFeedCount(state);
    updateFeedHeading(Pinalert.state.view);

    if (state.status === 'idle' || state.status === 'loading') {
      setHidden(skeletonEl, false);
      setHidden(reportListEl, true);
      setHidden(pastGroupsEl, true);
      setEmptyState('none');
      setHidden(errorEl, true);
      return;
    }

    setHidden(skeletonEl, true);

    if (state.status === 'error') {
      // A fetch failure never clears the rows or the map's existing pins:
      // stale data is more useful than a blank panel to someone who just lost
      // signal (WR-08). While there are rows to show, the list stays visible
      // and the error panel appears alongside it with its Retry button. Only
      // a failure with nothing to show hides the list, and then the rows still
      // in the page are removed through clearRows, which puts focus back on a
      // control that is still shown instead of dropping it to the page body.
      // A later poll or Retry with rows on screen never reaches the loading
      // branch below (app.js treats it as a background refresh), so the list a
      // reader is scrolling or has focus in is not hidden again. Rows fetched
      // for another tab were already dropped by the store, so a failure never
      // leaves one tab's rows under another tab.
      var hasRows = state.reports.length > 0;
      var onPast = Pinalert.state.view === 'past';
      setHidden(errorEl, false);
      setHidden(reportListEl, !hasRows);
      setHidden(pastGroupsEl, !hasRows || !onPast);
      if (!hasRows) {
        clearRows();
      }
      if (onPast) {
        // The Past tab never shows the live list, whatever rows it holds.
        setHidden(reportListEl, true);
      }
      setEmptyState('none');
      return;
    }

    setHidden(errorEl, true);
    updateFeedStatus(state);

    if (Pinalert.state.view === 'past') {
      renderPastTab(state);
      return;
    }
    setHidden(pastGroupsEl, true);

    if (state.reports.length === 0) {
      // An empty list shows exactly one empty block, instead of the list. A
      // chip that emptied it says so, with the button that clears it: the
      // reader's own filter is the reason, never "no reports" (D-18). With no
      // chip on, Disputed has its own words and Active the general ones. The
      // history kind (recent counts and See past reports) is chosen by the
      // Past cards plan, which owns those numbers. Read from the store's tab
      // rather than the pressed link: the store's value describes the data
      // actually in state.reports, while the link describes an intent whose
      // fetch may still be in flight.
      var kind = 'general';
      if (Pinalert.state.category !== '') {
        kind = 'filter';
      } else if (Pinalert.state.view === 'disputed') {
        kind = 'disputed';
      }
      setEmptyState(kind);
      setHidden(reportListEl, true);
      clearRows();
      return;
    }

    setEmptyState('none');
    setHidden(reportListEl, false);
    renderRows(state.reports);
  }

  // findReportById looks the report back up from the shared store rather
  // than caching a stale copy, so the periodic age/time refresh below is
  // always working from the current fetched data.
  function findReportById(idKey) {
    var reports = Pinalert.state.reports;
    for (var i = 0; i < reports.length; i++) {
      if (String(reports[i].id) === idKey) {
        return reports[i];
      }
    }
    return null;
  }

  // refreshAgeAndTime re-evaluates the age stage and the relative-time text
  // on a light interval (elapsed time, not a server push, drives both), by
  // updating classes/text on existing rows rather than re-rendering the
  // list — a report visibly ages even in a tab nobody has touched.
  function refreshAgeAndTime() {
    var ids = Object.keys(rowsById);
    for (var i = 0; i < ids.length; i++) {
      var idKey = ids[i];
      var report = findReportById(idKey);
      if (!report) {
        continue;
      }
      var row = rowsById[idKey];
      replacePrefixedClass(row.el, 'age-', 'age-' + PinalertTrust.fadeStage(report));
      Pinalert.setText(row.meta, buildMetaText(report));
    }
  }

  // applySelectionHighlight is the shared implementation for both selection
  // directions: it marks the row of the selected report and clears the mark
  // from every other row, and it never scrolls the page or the list. On a
  // wide screen the document itself scrolls, so bringing the row into view
  // would carry the map away from the reader who just clicked a pin (A87).
  // A reader who wants the list goes there by hand.
  function applySelectionHighlight(id) {
    var idKey = id === null || id === undefined ? null : String(id);
    var ids = Object.keys(rowsById);
    for (var i = 0; i < ids.length; i++) {
      var rid = ids[i];
      var el = rowsById[rid].el;
      if (idKey !== null && rid === idKey) {
        el.classList.add('report-row--selected');
      } else {
        el.classList.remove('report-row--selected');
      }
    }
  }

  // revealRowInListPane brings the row of a pin the reader just picked into the
  // list pane's own box, on a phone only. Below 900px the list is a scroll box
  // under a fixed height map (phone.css), so moving the list never moves the
  // map, which was the reason A87 gave for leaving the list alone. It sets the
  // PANE's scroll position and nothing else: the page is never scrolled, and
  // applySelectionHighlight above stays free of any scroll.
  function revealRowInListPane(id) {
    if (!window.matchMedia || !window.matchMedia(PHONE_QUERY).matches) {
      return;
    }
    var row = rowsById[String(id)];
    var pane = reportListEl ? reportListEl.closest('.pane--list') : null;
    if (!row || !pane || pane.scrollHeight <= pane.clientHeight) {
      return;
    }
    var gap = parseFloat(window.getComputedStyle(document.documentElement).getPropertyValue('--space-sm'));
    if (isNaN(gap)) {
      gap = 0;
    }
    var paneBox = pane.getBoundingClientRect();
    var rowBox = row.el.getBoundingClientRect();
    var top = rowBox.top - paneBox.top + pane.scrollTop - gap;
    var calm = !!window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    pane.scrollTo({ top: Math.max(0, top), behavior: calm ? 'auto' : 'smooth' });
  }

  // createToggleIcon builds a minimal inline glyph for the view-toggle
  // button. UI-SPEC's Icon Mapping only covers the 9 category icons; no
  // dedicated map/list asset exists, and adding new SVG files under
  // /static/icons (e.g. new-icon.svg) is outside this plan's two-file
  // ownership. Built via the SVG DOM API (never a markup string),
  // aria-hidden since the button's text label
  // already carries the meaning.
  function createToggleIcon(kind) {
    var svg = document.createElementNS(SVG_NS, 'svg');
    svg.setAttribute('viewBox', '0 0 24 24');
    svg.setAttribute('width', '16');
    svg.setAttribute('height', '16');
    svg.setAttribute('aria-hidden', 'true');
    svg.setAttribute('focusable', 'false');
    svg.setAttribute('class', 'view-toggle__icon');

    var path = document.createElementNS(SVG_NS, 'path');
    path.setAttribute('fill', 'none');
    path.setAttribute('stroke', 'currentColor');
    path.setAttribute('stroke-width', '2');
    path.setAttribute('stroke-linecap', 'round');
    path.setAttribute('stroke-linejoin', 'round');

    if (kind === 'list') {
      path.setAttribute('d', 'M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01');
    } else {
      path.setAttribute('d', 'M3 6l6-2 6 2 6-2v14l-6 2-6-2-6 2z M9 4v14 M15 6v14');
    }

    svg.appendChild(path);
    return svg;
  }

  // updateToggleLabel labels the button by its destination (Copywriting
  // Contract): it reads "List" while the map is showing and "Map" while
  // the list is showing.
  function updateToggleLabel() {
    if (!viewToggleBtn || !appShellEl) {
      return;
    }
    var showingList = appShellEl.getAttribute('data-view') === 'list';
    var destination = showingList ? 'Map' : 'List';

    while (viewToggleBtn.firstChild) {
      viewToggleBtn.removeChild(viewToggleBtn.firstChild);
    }
    viewToggleBtn.appendChild(createToggleIcon(showingList ? 'map' : 'list'));

    var label = document.createElement('span');
    Pinalert.setText(label, destination);
    viewToggleBtn.appendChild(label);

    viewToggleBtn.setAttribute('aria-label', 'Switch to ' + destination.toLowerCase() + ' view');
    viewToggleBtn.setAttribute('aria-pressed', showingList ? 'true' : 'false');
  }

  function moveFocusIntoView(view) {
    var target;
    if (view === 'map') {
      // An open popup card takes focus first (A51).
      if (window.PinalertMap && typeof PinalertMap.focusOpenPopup === 'function' && PinalertMap.focusOpenPopup()) {
        return;
      }
      target = document.getElementById('fab-report');
    } else {
      target = reportListEl && reportListEl.querySelector('.report-row');
    }
    if (target) {
      target.focus();
    } else if (viewToggleBtn) {
      viewToggleBtn.focus();
    }
  }

  // switchView flips the shell's data-view attribute; main.css owns every
  // show/hide rule keyed off it. Order matters for the Leaflet fix below:
  // the pane must be un-hidden before the resize event fires, or Leaflet
  // reads a zero-size container.
  function switchView(view) {
    if (!appShellEl) {
      return;
    }
    appShellEl.setAttribute('data-view', view);
    updateToggleLabel();

    if (view === 'map') {
      // map.js keeps its Leaflet map instance private (its returned API is
      // only {flyTo, highlight}), so this file cannot call invalidateSize()
      // directly. Dispatching a window resize event is the standard way to
      // reach it anyway: Leaflet's default trackResize option listens for
      // exactly this event and calls the map's own invalidateSize() in
      // response, which is what a map hidden-then-shown via display:none
      // needs to stop rendering with a stale container size.
      window.dispatchEvent(new Event('resize'));
    }

    moveFocusIntoView(view);
  }

  if (viewToggleBtn) {
    // #view-toggle ships nested inside .pane--list in index.html.tmpl.
    // main.css's narrow-screen rules set .pane--list to display:none while
    // data-view="map" — an ancestor's display:none removes every
    // descendant from rendering regardless of the descendant's own
    // position/display values, so the position:fixed toggle button
    // vanishes with it, stranding a narrow-screen visitor on the map with
    // no way back to the list. Reparenting the button under #app-shell
    // (a sibling of both panes) keeps main.css's breakpoint rules the sole,
    // untouched authority — .view-toggle's own rules are all element-
    // scoped, so nothing about its appearance or positioning depends on
    // which element contains it.
    if (appShellEl && viewToggleBtn.parentNode !== appShellEl) {
      appShellEl.appendChild(viewToggleBtn);
    }

    viewToggleBtn.addEventListener('click', function () {
      var current = appShellEl ? appShellEl.getAttribute('data-view') : 'map';
      switchView(current === 'list' ? 'map' : 'list');
    });
  }

  if (retryBtn) {
    retryBtn.addEventListener('click', function () {
      Pinalert.fetchReports();
    });
  }

  // readViewFromURL reads the tab out of the page address (03.1 D-04, D-05).
  // Parsed with URLSearchParams and compared against a closed list of words,
  // never interpolated anywhere (T-01-17), so the page and the server cannot
  // disagree about what an address means. An absent, misspelled or hostile
  // value opens Active, which is D-10's closed default: the fail-safe
  // direction, since guessing wrong here would show a reader reports they
  // did not ask for. view=resolved is a read only alias of Past, and an old
  // bookmark that carries the old checkbox parameter set to true and no view
  // opens Disputed. The answer says
  // whether the address holds a spelling this script no longer writes, so the
  // caller tidies it once.
  function readViewFromURL() {
    var params = new URLSearchParams(window.location.search);
    var word = params.get(VIEW_PARAM);
    var hasOld = params.has(OLD_BOOKMARK_PARAM);
    if (word === 'disputed' || word === 'past') {
      return { view: word, rewrite: hasOld };
    }
    if (word === 'resolved') {
      return { view: 'past', rewrite: hasOld };
    }
    if (word === null && params.get(OLD_BOOKMARK_PARAM) === 'true') {
      return { view: 'disputed', rewrite: true };
    }
    return { view: 'active', rewrite: word !== null || hasOld };
  }

  // syncViewURL keeps the address bar in step with the tab: the parameter is
  // set for Disputed and Past and deleted for Active, so the default tab is
  // never written (a main page address with a query string would be taken for
  // a different page by the low data view, which would break a remembered
  // choice). It applies the update with history.replaceState, the History
  // API's in-place variant, so a tab press adds no Back button step: Back
  // would otherwise walk the reader through their own tab history instead of
  // leaving the page. Every other parameter (report, full, resend) is kept as
  // it is, and the old bookmark parameter is dropped.
  function syncViewURL(view) {
    var url = new URL(window.location.href);
    if (view === 'disputed' || view === 'past') {
      url.searchParams.set(VIEW_PARAM, view);
    } else {
      url.searchParams.delete(VIEW_PARAM);
    }
    url.searchParams.delete(OLD_BOOKMARK_PARAM);
    window.history.replaceState(null, '', url);
  }

  // updateTabStrip marks the current tab: aria-current on the link of the
  // view, none on the others. The current tab also differs in weight and
  // border (feed.css), so colour is never the only cue.
  function updateTabStrip(view) {
    if (!tabStripEl) {
      return;
    }
    var links = tabStripEl.querySelectorAll('.view-tab');
    for (var i = 0; i < links.length; i++) {
      if (links[i].getAttribute('data-feed-view') === view) {
        links[i].setAttribute('aria-current', 'page');
      } else {
        links[i].removeAttribute('aria-current');
      }
    }
  }

  // showTab brings everything that names the tab in step with the store.
  function showTab(view) {
    updateTabStrip(view);
    updateFeedHeading(view);
  }

  // switchTab is the one place a tab press lands. It sets the store, marks the
  // link, writes the address and refetches through the one list request in
  // app.js. Refetching rather than filtering state.reports in place is
  // deliberate: the server decides which reports exist for a tab (02-04), and
  // a client side filter would be a second visibility decision, the resolver
  // bypass T-02-04 names. Rows fetched for the other tab stay only until the
  // answer lands; a failed fetch drops them (app.js).
  function switchTab(view) {
    if (Pinalert.VIEWS.indexOf(view) === -1 || view === Pinalert.state.view) {
      return;
    }
    Pinalert.setView(view);
    showTab(view);
    syncViewURL(view);
    Pinalert.fetchReports();
  }

  if (tabStripEl) {
    // The tabs are real links (/, /?view=disputed, /?view=past), so a middle
    // click, a modified click and "open in new tab" follow the link as they
    // are. A script only makes the plain left click instant.
    tabStripEl.addEventListener('click', function (evt) {
      var link = evt.target && evt.target.closest ? evt.target.closest('.view-tab') : null;
      if (!link || evt.defaultPrevented || evt.button !== 0 ||
          evt.metaKey || evt.ctrlKey || evt.shiftKey || evt.altKey) {
        return;
      }
      evt.preventDefault();
      switchTab(link.getAttribute('data-feed-view'));
    });
  }

  // focusCurrentTab puts focus on the link of the selected tab. The two empty
  // block buttons hide themselves when their press succeeds, which would drop
  // focus to the page body, so they hand it to the strip first.
  function focusCurrentTab() {
    var link = currentTabLink();
    if (link) {
      link.focus();
    }
  }

  if (filterClearBtn) {
    // Clear filter: the chip goes back to All and the list is read again. The
    // browser removes nothing itself; the server answers for the whole set.
    filterClearBtn.addEventListener('click', function () {
      Pinalert.setCategory('');
      Pinalert.fetchReports();
      focusCurrentTab();
    });
  }

  if (seePastBtn) {
    // See past reports: the one button that moves the reader between tabs.
    seePastBtn.addEventListener('click', function () {
      switchTab('past');
      focusCurrentTab();
    });
  }

  // Restore step, not a synchronisation step: the address is the source of
  // truth and the store is driven FROM it. Read it, hand the tab to the store,
  // mark the link, and keep the report id of a ?report=ID link for the first
  // list request (app.js sends it once). The address is rewritten only when it
  // held a spelling this script no longer writes, and the report parameter is
  // never touched, so a link sent by a moderator stays as it was sent (D-05).
  //
  // No fetch is issued here because it does not need to be: the first
  // report fetch fires from map.js's centerOnVisitor, inside an async
  // geolocation success/failure callback, and every deferred module
  // body, including this one, runs to completion before any async
  // callback can execute. This restore is therefore guaranteed to
  // precede the first fetch, so the first render is never briefly
  // on the wrong tab. If a future change ever moves the initial fetch into a
  // deferred module body ABOVE this one in the template's script order,
  // that guarantee breaks and this restore would need its own fetch.
  // Adding a fetch unconditionally would issue a duplicate request on every
  // single page load.
  var initialAddress = readViewFromURL();
  Pinalert.setView(initialAddress.view);
  showTab(initialAddress.view);
  Pinalert.setFocusId(new URLSearchParams(window.location.search).get(REPORT_PARAM));
  if (initialAddress.rewrite) {
    syncViewURL(initialAddress.view);
  }

  Pinalert.onSelect(function (id, source) {
    // A selection that starts from the map applies the row highlight only
    // and never scrolls the page or the list (A87). Guard on source so a
    // list-originated selection, which activateRow already highlighted, is
    // not highlighted a second time.
    if (source === 'map') {
      applySelectionHighlight(id);
      revealRowInListPane(id);
    }
  });

  Pinalert.subscribe(render);
  render(Pinalert.state);
  updateToggleLabel();

  window.setInterval(refreshAgeAndTime, 60000);

  // pageshow / back-forward-cache refetch (02-UAT.md gap 3, "major").
  //
  // handlers.Page already sets the document's own Cache-Control: no-store
  // (DEC-Q) on every response. Chromium and Firefox honour that by
  // excluding the page from the back-forward cache entirely, so a Back
  // navigation on those browsers always re-runs this file from scratch.
  // Safari's WebKit page cache does NOT reliably honour that header for
  // bfcache eligibility, though — a Safari Back can restore a frozen
  // pre-vote DOM snapshot with no JavaScript re-run at all, which is
  // exactly what the UAT reporter saw after tapping Reopen on Activity and
  // pressing Back: nothing refetched, so the feed kept showing the
  // pre-reopen state. This listener is a defensive addition on top of the
  // no-store header, not a replacement for it — it is what makes the
  // restore itself correct, rather than trying to fight for bfcache
  // exclusion on a browser that does not reliably grant it.
  //
  // The guard on the event's persisted property is what stops an ordinary
  // first load or a normal forward navigation from firing a duplicate
  // fetch: pageshow also fires on those, with persisted false. Only a true
  // bfcache restore sets it to true.
  //
  // The poll timer (app.js's startPolling) is not a substitute for this
  // listener: a restored page's interval timers resume ticking from where
  // they left off, but the reader would still see the stale snapshot for
  // up to a full poll interval (config.pollIntervalMs) before the next
  // tick fires — exactly the delay the UAT reported.
  //
  // Do NOT add an unload or beforeunload listener anywhere in this file,
  // or anywhere else, to "fix" this instead: either one makes the
  // page permanently ineligible for the back-forward cache, which would
  // suppress this symptom by destroying the very restore this listener is
  // making correct — and would also cost the reader the scroll-position
  // and form-state benefits bfcache gives them on every other Back
  // navigation.
  window.addEventListener('pageshow', function (evt) {
    if (!evt.persisted) {
      return;
    }
    // Re-read the address and re-apply it to the strip and the store
    // BEFORE refetching (UAT gap 5's extension into gap 3): a restored frozen
    // DOM can never show one tab over another tab's rows, because the
    // strip is always re-derived from the address rather than trusted as
    // whatever the browser happened to restore.
    var restored = readViewFromURL();
    Pinalert.setView(restored.view);
    showTab(restored.view);
    Pinalert.fetchReports();
  });
}());
