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
// SCOPE (T-01-23, TRUST-02, 03.1 D-21): a list row is compact text with no
// control in it. It shows the severity word, the resolver's own visibility
// state (in words, the visibility tag, and in the row treatment, the state
// class), the distance, the title and the plain text trust line from
// PinalertTrust. Confirm, Dispute and Mark resolved live in the opened
// report only. The trust line renders the server's counters and words
// exactly as sent, and this file computes none of them: a count here is never
// typed, capped or bumped by the browser.
(function () {
  'use strict';

  var SEVERITY_RANK = { critical: 3, medium: 2, low: 1 };

  // REPORT_PARAM is the address parameter of a link to one report. It is read
  // once on load and never removed from the address.
  var REPORT_PARAM = 'report';

  // The page My activity opens on Ended nearby (Phase 3.4, D-18). A link to a
  // finished report leaves Nearby for it with the report id appended.
  var ENDED_NEARBY_PATH = '/profile?part=ended';

  // Ends the address of that redirect (code review part 2, WR-05). location.replace
  // keeps the page it left as the referrer but removes it from history, so the
  // phone back arrow (app-bar.js) cannot trust the referrer on the page it opens:
  // it sees this mark and follows its own address instead of going back, which
  // could leave the site. The server ignores the parameter.
  var REDIRECT_MARK = '&from=link';

  // The only shape of a report id taken from the store into an address.
  var REPORT_ID_PATTERN = /^[1-9][0-9]{0,17}$/;

  // The words of the disputed cues (UI-SPEC 2e, P34-C8). The state word
  // "Disputed" itself comes from PinalertVisibility, the one source of state
  // words. These are fixed strings, never read from the server.
  var DISPUTED_LINE = 'Disputed: may not be true';
  var DISPUTED_LEAD = 'Disputed, may not be true.';
  var PROVISIONAL_LEAD = 'Not confirmed yet.';

  // The words of the Show choices in the polite status line (P34-C7).
  var SHOW_STATUS_WORDS = { confirmed: 'Show Confirmed', disputed: 'Show Disputed' };

  var reportListEl = document.getElementById('report-list');
  var skeletonEl = document.getElementById('feed-skeleton');
  var emptyEl = document.getElementById('feed-empty');
  var disputedEmptyEl = document.getElementById('disputed-empty');
  var confirmedEmptyEl = document.getElementById('confirmed-empty');
  var errorEl = document.getElementById('feed-error');
  var retryBtn = document.getElementById('feed-retry');
  var appShellEl = document.getElementById('app-shell');
  var queueStripEl = document.getElementById('queue-strip');
  var showButtonEl = document.getElementById('show-button');
  var feedCountEl = document.getElementById('feed-count');
  var feedStatusEl = document.getElementById('feed-status');
  var historyEmptyEl = document.getElementById('feed-empty-history');
  var filterEmptyEl = document.getElementById('filter-empty');
  var filterClearBtn = document.getElementById('filter-empty-clear');
  var historyEmptyBodyEl = document.getElementById('feed-empty-history-body');
  var staleEl = document.getElementById('feed-stale');
  var staleTextEl = document.getElementById('feed-stale-text');
  var staleRetryBtn = document.getElementById('feed-stale-retry');
  var noticeLiveEl = document.getElementById('notice-live');
  var noticeCardEl = document.getElementById('notice-card');
  var noticeSlowEl = document.getElementById('notice-slow');
  var noticeStaleEl = document.getElementById('notice-stale');
  var noticeStaleTextEl = document.getElementById('notice-stale-text');
  var noticeQueueEl = document.getElementById('notice-queue');
  var noticeQueueTextEl = document.getElementById('notice-queue-text');
  var noticeQueueBtn = document.getElementById('notice-queue-show');
  // The number of saved reports of this account, set by the queue's count
  // event. The Queue row of the notice card shows it on a phone in map view.
  var queueCount = 0;
  var QUEUE_CARD_SUFFIX = ' waiting to send.';
  var narrowQuery = window.matchMedia ? window.matchMedia('(max-width: 899.98px)') : null;

  // The words of the stale line and its two announcements. The strings are
  // fixed here and only the time phrase is computed, never read from the
  // server. staleShown is true while the stale line is showing, so each
  // announcement is made once, on the change, not on every poll.
  var STALE_ENTER_PREFIX = 'Connection lost. Showing reports from ';
  var STALE_LEAVE_MESSAGE = 'Back online. The list is up to date.';
  var staleShown = false;

  // The sentence end of the empty All view's body (P34-C10), after the counts.
  var REPORT_PROMPT = "Post a report to share what you're seeing.";

  // lastAnnouncedKey is the Show choice, place and chip the status line last
  // announced, and statusStarted is false until the first answer lands. The
  // first answer is not announced: a reader who just opened the page hears
  // the list itself, not a status line over it.
  var lastAnnouncedKey = '';
  var statusStarted = false;

  // rowsById reconciles rendered <li> nodes by report id across polls,
  // rather than clearing and rebuilding the list every 30 seconds, so
  // scroll position and focus survive a background refresh.
  var rowsById = {};

  // handledFocus is the first focus object (the server's answer to a report
  // link) that handleFocus acted on, or null before one has landed. Every
  // answer carries a new focus object while the link is held (CR-01), so the
  // toast and the redirect of a finished report are made once per link, never
  // again on a poll, a vote or a Show change.
  var handledFocus = null;

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
  // recent history, with the See ended reports link), 'confirmed' (Show
  // Confirmed has nothing to show), 'disputed' (Show Disputed has nothing to
  // show) or 'filter' (a chip emptied the list, with the Clear filter
  // button).
  function setEmptyState(kind) {
    setHidden(emptyEl, kind !== 'general');
    setHidden(historyEmptyEl, kind !== 'history');
    setHidden(confirmedEmptyEl, kind !== 'confirmed');
    setHidden(disputedEmptyEl, kind !== 'disputed');
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

  function updateWrittenLine(el, report) {
    // The second meta fact of a late report, "Written 4 hours ago". The server
    // sends written_at only past 30 minutes, so most rows leave this hidden.
    // It is display only: the first meta line, the fade and the order still
    // count from upload (D-19).
    var line = Pinalert.writtenLine(report);
    Pinalert.setText(el, line);
    el.hidden = !line;
  }

  // activateRow chooses a report from the list (D-17): the row takes the soft
  // fill, its pin grows (map.js, from the same selection), the map brings the
  // pin into view and the report opens as built (the popup on a laptop, the
  // detail sheet on a phone). On a phone the list sheet then drops to short
  // beneath the detail sheet and remembers where it was; map.js hands it back
  // when the detail closes. The sheet moves last, after the report has opened,
  // because opening a report closes a previous one and that close restores the
  // sheet. PinalertSheet does nothing on a laptop.
  function activateRow(id) {
    Pinalert.select(id, 'list');
    applySelectionHighlight(id);
    var opened = true;
    if (window.PinalertMap && typeof PinalertMap.flyTo === 'function') {
      opened = PinalertMap.flyTo(id, { opener: 'list' }) !== false;
    }
    if (opened && window.PinalertSheet) {
      window.PinalertSheet.remember();
      window.PinalertSheet.setSnap('short');
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

    // The Written line of a late report sits right after the meta line, with
    // the same look, and stays hidden until the report carries written_at.
    var written = document.createElement('p');
    written.className = 'report-row__meta';
    written.hidden = true;

    // The distance segment, drawn at every width. It is filled by updateRow
    // from the store's one distance sentence and stays hidden when the feed
    // is centred on neither the reader nor a searched place (A94, D-14).
    var away = document.createElement('p');
    away.className = 'report-row__away';
    away.hidden = true;

    // The second plain fact after the distance, shown only when the reader's
    // cached fix puts the reader beyond the vote radius (D-14). It is text,
    // not a control, and the opened report explains it.
    var far = document.createElement('span');
    far.className = 'report-row__far';
    Pinalert.setText(far, 'Too far to vote');
    far.hidden = true;

    // The words a screen reader ends the row with. The row is one tab stop
    // and opens the report on Enter or Space.
    var cue = document.createElement('span');
    cue.className = 'report-row__open-cue visually-hidden';
    Pinalert.setText(cue, 'Open report');

    // The state a screen reader hears first (UI-SPEC 2e): "Disputed, may not
    // be true." on a disputed row, "Not confirmed yet." on a provisional one,
    // and nothing on the others. It is the first text of the row and is never
    // drawn. It is text inside the row rather than an aria-label on the row,
    // so the row's own text and counters are still read after it.
    var lead = document.createElement('span');
    lead.className = 'report-row__lead visually-hidden';
    lead.hidden = true;

    // A26, A45: the visible severity word is the first text in the row body,
    // read before the title. It is the first child appended; the lead is put
    // in front of it afterwards, so the state is the first thing heard.
    var severityLabel = PinalertTrust.createSeverityLabel();
    body.appendChild(severityLabel);
    body.insertBefore(lead, severityLabel);

    // The bold line of a disputed row (cue 3): "Disputed: may not be true",
    // above the counters that carry the server's dispute figures. Hidden on
    // every other row.
    var disputedLine = document.createElement('p');
    disputedLine.className = 'report-row__disputed-line';
    Pinalert.setText(disputedLine, DISPUTED_LINE);
    disputedLine.hidden = true;

    // The state label is built here and placed, with the reporter words, on
    // the state line (A75, A81). The tag ships hidden from its own builder.
    // The state line gives its children to the body's flex order (R3).
    var tag = PinalertVisibility.createVisibilityTag();

    // The plain text trust line (D-20, D-21): the two counters and the
    // currency word, no toggle and no panel. Its reporter words ride in the
    // state line.
    var trust = PinalertTrust.createTrustLine();
    body.appendChild(PinalertTrust.createStateLine('report-row__stateline', tag, trust));
    body.appendChild(meta);
    body.appendChild(written);
    body.appendChild(away);
    body.appendChild(far);
    body.appendChild(title);
    body.appendChild(disputedLine);
    body.appendChild(trust.el);
    body.appendChild(cue);

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

    return {
      el: li, glyph: glyph, severity: severityLabel, title: title, meta: meta, written: written,
      away: away, far: far, tag: tag, trust: trust, lead: lead, disputedLine: disputedLine
    };
  }

  // isDisputed is the one test of the disputed cues, the group and the pin: the
  // server's own view word on the report. The browser never decides that a
  // report is disputed from counts (D-14, TRUST-02).
  function isDisputed(report) {
    return !!report && report.view === 'disputed';
  }

  // leadText is the state words a screen reader hears before the row's text.
  function leadText(report) {
    if (isDisputed(report)) {
      return DISPUTED_LEAD;
    }
    if (report && PinalertVisibility.displayState(report) === 'provisional') {
      return PROVISIONAL_LEAD;
    }
    return '';
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
    updateWrittenLine(row.written, report);

    // The distance is the store's one sentence: the reader's own centre, a
    // searched place, or nothing for the fallback centre (A94, D-14). The
    // far fact reads the cached fix only and never asks for a location.
    var away = Pinalert.distanceText(report);
    Pinalert.setText(row.away, away);
    row.away.hidden = !away;
    row.far.hidden = !PinalertVotes.farFromCachedFix(report);

    PinalertVisibility.updateVisibilityTag(row.tag, report);
    PinalertTrust.updateTrustLine(row.trust, report.trust);

    // The disputed cues (D-14): the grey category glyph shown, the bold line,
    // the screen reader lead. Full contrast and never faded (D-07): no rule
    // for this class sets opacity.
    var disputed = isDisputed(report);
    row.el.classList.toggle('report-row--disputed', disputed);
    row.disputedLine.hidden = !disputed;
    var lead = leadText(report);
    Pinalert.setText(row.lead, lead);
    row.lead.hidden = lead === '';

    var selectedId = Pinalert.state.selectedId;
    var isSelected = selectedId !== null && selectedId !== undefined &&
      String(selectedId) === String(report.id);
    row.el.classList.toggle('report-row--selected', isSelected);
  }

  // elementIsShown is true for a connected element that takes part in layout,
  // so a row in a hidden list pane is never a focus candidate.
  function elementIsShown(el) {
    return !!el && el.isConnected && el.getClientRects().length > 0;
  }

  // reportOpener is the control that opens the report window at the current
  // size: the Report item of the bottom bar on a phone, the Post a report link
  // of the top bar on a laptop (Phase 3.4, D-10). Each size hides the other
  // with display none, so the one with a layout box is the visible one. It is
  // found by the data-open-report attribute, not by an id, and it is null when
  // neither takes part in layout.
  function reportOpener() {
    var openers = document.querySelectorAll('[data-open-report]');
    for (var i = 0; i < openers.length; i += 1) {
      if (elementIsShown(openers[i])) {
        return openers[i];
      }
    }
    return null;
  }

  // showButton is the Show button of the list head, or null without one. It
  // is where focus lands when a control that held it leaves the page.
  function showButton() {
    return showButtonEl;
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
  // row among the candidates, else the map, else the Show button, else
  // the sheet handle, else the empty state. It acts when focus fell to the page
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
    var showEl = showButton();
    if (elementIsShown(showEl)) {
      showEl.focus();
      return;
    }
    var handle = document.getElementById('sheet-handle');
    if (elementIsShown(handle)) {
      handle.focus();
      return;
    }
    var empties = [emptyEl, historyEmptyEl, confirmedEmptyEl, disputedEmptyEl, filterEmptyEl];
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
  //
  // Grouping (D-14): the rows are sorted in the feed's own order, then split
  // into two groups by the server's view word and nothing else: every report
  // that is not disputed first, every disputed report last, each group in the
  // sorted order. The last group carries no heading of its own: each disputed
  // row says "Disputed: may not be true" itself (UI-SPEC 2e cue 3), and the
  // words are not repeated (D-02).
  function renderRows(reports) {
    var sorted = Pinalert.disputedLast(sortReports(reports));
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
    var slot = 0;
    for (var i = 0; i < sorted.length; i++) {
      var report = sorted[i];
      var idKey = String(report.id);

      var row = rowsById[idKey];
      if (!row) {
        row = createRow(report.id);
        rowsById[idKey] = row;
      }
      updateRow(row, report);

      var expected = reportListEl.children[slot];
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
      slot += 1;
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

  // updateFeedCount writes the count line of the list head (P34-C6): the
  // number is the count of the cards shown under the current Show choice (the
  // one visible set the rows and the pins draw, the disputed group included in
  // Show All), the radius is the one the feed was asked for, and the line reads
  // "5 reports, 5 km" or "1 report, 5 km". It is
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
      var count = Pinalert.visibleReports().length;
      text = count + (count === 1 ? ' report, ' : ' reports, ') + radius + ' km';
    }
    Pinalert.setText(feedCountEl, text);
  }

  // recentSentence is the counts sentence {A} of the empty All view (C39,
  // HIST-04) from the server's recent object. Only the parts whose count is
  // above zero are written, and the numbers are the server's, never typed
  // here. It returns an empty string when nothing happened lately, which is
  // how render knows to show the plain empty block instead.
  function recentSentence(recent) {
    if (!recent) {
      return '';
    }
    var resolved = recent.resolved_24h;
    var expired = recent.expired_24h;
    var parts = [];
    if (Number.isInteger(resolved) && resolved > 0) {
      parts.push(resolved + (resolved === 1 ? ' was resolved' : ' were resolved'));
    }
    if (Number.isInteger(expired) && expired > 0) {
      parts.push(expired + ' faded out');
    }
    if (parts.length === 0) {
      return '';
    }
    return parts.join(' and ') + ' in the last 24 hours.';
  }

  // updateFeedStatus writes the one status sentence (UI-SPEC 2i) into the
  // visually hidden live region, for example "3 reports, Show Confirmed. Near
  // Kochi. Filtered by Flood." It speaks once per Show, place or chip change,
  // when the answer for that change has landed (so the count is the real one).
  // The Show words are named only when the choice is not All. A poll that only
  // changes the count never speaks again, and the first answer of a page load
  // is recorded without speaking. The numbers and words are the store's,
  // nothing is invented.
  function updateFeedStatus(state) {
    if (!feedStatusEl || state.status !== 'ok') {
      return;
    }
    var store = Pinalert.state;
    var placeName = store.place ? store.place.name : '';
    var key = store.show + '|' + placeName + '|' + store.category;
    if (key === lastAnnouncedKey) {
      return;
    }
    lastAnnouncedKey = key;
    if (!statusStarted) {
      statusStarted = true;
      return;
    }
    var count = Pinalert.visibleReports().length;
    var text = count + (count === 1 ? ' report' : ' reports');
    if (SHOW_STATUS_WORDS[store.show]) {
      text += ', ' + SHOW_STATUS_WORDS[store.show];
    }
    text += '.';
    if (placeName !== '') {
      text += ' Near ' + placeName + '.';
    }
    if (store.category !== '') {
      text += ' Filtered by ' + (Pinalert.CHIP_LABELS[store.category] || Pinalert.categoryLabel(store.category)) + '.';
    }
    Pinalert.setText(feedStatusEl, text);
  }

  // announce speaks one sentence through the page level live region. It does
  // nothing on a page without it.
  function announce(text) {
    if (!noticeLiveEl) {
      return;
    }
    Pinalert.setText(noticeLiveEl, text);
  }

  // listPaneHidden is true when the list pane, and so the queue strip and the
  // stale line, is not displayed. It asks the layout, not data-view: the
  // phone layout shows the map and the list together whatever data-view says
  // (phone.css), so the attribute alone would hide the list on every phone
  // and the notice card would repeat, over the list, what the list already
  // shows (review finding WR-W02). A pane with no layout box is not shown.
  function listPaneHidden() {
    var pane = appShellEl ? appShellEl.querySelector('.pane--list') : null;
    return !!pane && pane.getClientRects().length === 0;
  }

  // openSavedReport opens the report form for a saved report (Edit and
  // resend), prefilled from the saved row, under the Not sent note (D-20). Its
  // post names the account the row was saved under (WR-02).
  function openSavedReport(record, opener, note) {
    if (!window.PinalertReportForm) {
      return;
    }
    window.PinalertReportForm.open({
      opener: opener || reportOpener(),
      prefill: record.payload,
      note: note,
      requestId: record.request_id,
      accountId: record.account_id
    });
  }

  // queueFocusFallback is where focus goes when the last saved report leaves:
  // the first report row, else the opener of the report window.
  function queueFocusFallback() {
    var firstRow = reportListEl ? reportListEl.querySelector('.report-row') : null;
    return firstRow || reportOpener();
  }

  // syncNoticeCard sets the Stale row of the notice card (shown only while the
  // stale line is active and the list pane is not displayed) and then hides
  // the whole card exactly when none of its rows is visible. Every script that
  // changes a row follows this rule.
  function syncNoticeCard() {
    if (!noticeCardEl) {
      return;
    }
    if (staleShown) {
      Pinalert.setText(noticeStaleTextEl, 'Last updated ' + Pinalert.updatedAgo(Pinalert.state.lastUpdatedAt) + '.');
    }
    setHidden(noticeStaleEl, !(staleShown && listPaneHidden()));
    if (noticeQueueEl) {
      if (queueCount > 0) {
        Pinalert.setText(noticeQueueTextEl, queueCount + QUEUE_CARD_SUFFIX);
      }
      setHidden(noticeQueueEl, !(queueCount > 0 && listPaneHidden()));
    }
    setHidden(noticeCardEl, noticeCardEl.querySelector('.notice-card__row:not([hidden])') === null);
  }

  // markFeedLoaded tells the head script that the list has loaded, and takes
  // the Slow row away without any announcement.
  function markFeedLoaded() {
    document.documentElement.setAttribute('data-feed-state', 'ok');
    setHidden(noticeSlowEl, true);
    syncNoticeCard();
  }

  // staleLineText is the visible sentence of the stale line.
  function staleLineText() {
    return 'Last updated ' + Pinalert.updatedAgo(Pinalert.state.lastUpdatedAt) + '. Trying again.';
  }

  // renderFirstLoad draws the page before any list has loaded: the skeleton
  // while the first request is out, the Couldn't load reports panel when it
  // failed. Nothing else shows the skeleton or the panel, so a poll that fails
  // later can never put the reader back on them.
  function renderFirstLoad(state) {
    setHidden(staleEl, true);
    staleShown = false;
    if (state.status === 'error') {
      setHidden(skeletonEl, true);
      setHidden(errorEl, false);
    } else {
      setHidden(skeletonEl, false);
      setHidden(errorEl, true);
    }
    setHidden(reportListEl, true);
    setEmptyState('none');
    syncNoticeCard();
  }

  // renderStale shows the stale line while the last poll failed after a list
  // had loaded, and takes it away when a later poll succeeds. It announces
  // once when the line appears and once when it goes. Focus on Retry now moves
  // to the first row before the line hides, so it is not dropped on the page.
  function renderStale(state) {
    var stale = state.status === 'error';
    if (stale) {
      Pinalert.setText(staleTextEl, staleLineText());
      setHidden(staleEl, false);
      if (staleRetryBtn) {
        staleRetryBtn.disabled = false;
      }
      if (!staleShown) {
        announce(STALE_ENTER_PREFIX + Pinalert.updatedAgo(Pinalert.state.lastUpdatedAt) + '.');
      }
    } else if (staleShown && state.status === 'ok') {
      if (staleRetryBtn && document.activeElement === staleRetryBtn && reportListEl) {
        var firstRow = reportListEl.querySelector('.report-row');
        if (firstRow) {
          firstRow.focus();
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
    syncNoticeCard();
  }

  // refreshStaleLine renews the minute figure of the stale line on a timer.
  // It never announces.
  function refreshStaleLine() {
    if (staleShown) {
      Pinalert.setText(staleTextEl, staleLineText());
    }
    syncNoticeCard();
  }

  // render is the single state machine for the list, the first load skeleton,
  // the first load error panel, the stale line and the empty states. Before
  // any list has loaded, renderFirstLoad decides. After one has, the list
  // path runs for every status: a failed or slow poll keeps the rows and the
  // map pins and shows the stale line, so the skeleton and the error panel
  // belong to the first load only (D-20).
  function render(state) {
    updateFeedCount(state);

    // The one set of reports under the Show choice. The rows, the count line,
    // the status line and the empty states all read it, and map.js draws its
    // pins from the same call, so the list and the map cannot disagree.
    var visible = Pinalert.visibleReports();

    var listShown = state.hasLoaded === true || state.reports.length > 0;
    if (!listShown) {
      renderFirstLoad(state);
      return;
    }

    setHidden(skeletonEl, true);
    setHidden(errorEl, true);
    if (state.status === 'ok') {
      markFeedLoaded();
    }
    renderStale(state);

    if (state.status === 'error') {
      // A fetch failure never clears the rows or the map's existing pins:
      // stale data is more useful than a blank panel to someone who just lost
      // signal (WR-08). While there are rows to show, the list stays visible
      // under the stale line. Only a failure with nothing to show hides the
      // list, and then the rows still in the page are removed through
      // clearRows, which puts focus back on a control that is still shown
      // instead of dropping it to the page body. A failure draws no empty
      // state, because the page cannot say the list is empty after a failed
      // answer. A later poll or Retry never reaches the first load branch (the
      // store treats it as a background refresh), so the list a reader is
      // scrolling or has focus in is not hidden again. Rows fetched for
      // another Show choice were already dropped by the store, so a failure
      // never leaves one choice's rows under another.
      var hasRows = visible.length > 0;
      setHidden(reportListEl, !hasRows);
      if (!hasRows) {
        clearRows();
      }
      setEmptyState('none');
      return;
    }

    setHidden(errorEl, true);
    updateFeedStatus(state);

    if (visible.length === 0) {
      // An empty list shows exactly one empty block, instead of the list. A
      // chip that emptied it says so, with the button that clears it: the
      // reader's own filter is the reason, never "no reports" (D-18). With no
      // chip on, Show Disputed and Show Confirmed have their own words (the
      // Confirmed block also covers an area with reports that nobody has
      // confirmed yet), an unfiltered All with something resolved or faded in
      // the last 24 hours says what happened lately with a link to My
      // activity (HIST-04), and anything else gets the general words. Read from
      // the store's choice rather than the pressed menu item: the store's value
      // describes the data actually in state.reports, while the menu describes
      // an intent whose fetch may still be in flight.
      var kind = 'general';
      if (Pinalert.state.category !== '') {
        kind = 'filter';
      } else if (Pinalert.state.show === 'disputed') {
        kind = 'disputed';
      } else if (Pinalert.state.show === 'confirmed') {
        kind = 'confirmed';
      } else if (recentSentence(Pinalert.state.recent) !== '') {
        kind = 'history';
        Pinalert.setText(historyEmptyBodyEl, recentSentence(Pinalert.state.recent) + ' ' + REPORT_PROMPT);
      }
      setEmptyState(kind);
      setHidden(reportListEl, true);
      clearRows();
      handleFocus(state);
      return;
    }

    setEmptyState('none');
    setHidden(reportListEl, false);
    renderRows(visible);
    handleFocus(state);
  }

  // handleFocus acts on the server's answer to a ?report=ID link (D-04, D-11)
  // once per link: a report too old or not readable gets one plain message,
  // and a report that has finished (resolved or faded out) leaves Nearby for
  // My activity, Ended nearby, with its id (D-18, orchestrator decision 6).
  // An active or disputed report stays where it is: both are drawn in All, the
  // disputed one in the last group. The address is replaced rather than added
  // to, so Back does not return to a page that would redirect again. Later
  // answers for the same link only refresh the report. Opening the pin is the
  // map's own job.
  function handleFocus(state) {
    var focus = state.focus;
    if (!focus || handledFocus !== null) {
      return;
    }
    handledFocus = focus;
    if (focus.status === 'too_old') {
      Pinalert.showToast("That report is older than 7 days and is no longer shown.");
      return;
    }
    if (focus.status === 'unavailable') {
      Pinalert.showToast("That report isn't available.");
      return;
    }
    if (focus.status === 'ok' && focus.report &&
        (focus.report.view === 'resolved' || focus.report.view === 'past')) {
      var finishedId = String(focus.report.id);
      if (REPORT_ID_PATTERN.test(finishedId)) {
        window.location.replace(ENDED_NEARBY_PATH + '&report=' + finishedId + REDIRECT_MARK);
      } else {
        window.location.replace(ENDED_NEARBY_PATH + REDIRECT_MARK);
      }
    }
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
      updateWrittenLine(row.written, report);
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
  // list's own scroll box (D-17). On a phone that box is the sheet
  // (#report-sheet, phone.css), so moving the list never moves the map, which
  // was the reason A87 gave for leaving the list alone. On a laptop (Phase 3.4
  // plan 20, UI-SPEC 2a) the pane holds the head and the strip lines still and
  // only the cards list scrolls, so the box is #report-list; the head is
  // outside it and takes no room in the arithmetic below. The box is chosen by
  // the layout query at the moment of the pick. It sets that ONE box's scroll
  // position and nothing else: the page is never scrolled, and
  // applySelectionHighlight above stays free of any scroll. "Nearest": a row
  // that is already whole under the sticky handle and list head stays where it
  // is; any other row is scrolled to sit just under them. Smooth unless the
  // person asked for reduced motion.
  function revealRowInListPane(id) {
    var row = rowsById[String(id)];
    var sheet = narrowQuery && narrowQuery.matches ? document.getElementById('report-sheet') : reportListEl;
    if (!row || !sheet || sheet.scrollHeight <= sheet.clientHeight) {
      return;
    }
    var gap = parseFloat(window.getComputedStyle(document.documentElement).getPropertyValue('--space-sm'));
    if (isNaN(gap)) {
      gap = 0;
    }
    var sheetBox = sheet.getBoundingClientRect();
    var head = sheet.querySelector('.list-head');
    // The head is inside the box only on a phone (it is stuck under the handle
    // there). On a laptop the box starts at the first card, so nothing is
    // reserved above it and no gap is left either.
    var inside = head && sheet.contains(head);
    var stuck = inside ? head.getBoundingClientRect().bottom - sheetBox.top : 0;
    var lead = inside ? stuck + gap : 0;
    var rowBox = row.el.getBoundingClientRect();
    var rowTop = rowBox.top - sheetBox.top;
    var rowBottom = rowBox.bottom - sheetBox.top;
    if (rowTop >= lead && rowBottom <= sheet.clientHeight) {
      return;
    }
    var top = rowTop + sheet.scrollTop - lead;
    var calm = !!window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    sheet.scrollTo({ top: Math.max(0, top), behavior: calm ? 'auto' : 'smooth' });
  }

  if (retryBtn) {
    retryBtn.addEventListener('click', function () {
      Pinalert.fetchReports();
    });
  }

  // Retry now on the stale line asks the store for a background refresh. The
  // button is disabled while the request is out and enabled again by the
  // render that follows it.
  if (staleRetryBtn) {
    staleRetryBtn.addEventListener('click', function () {
      staleRetryBtn.disabled = true;
      Pinalert.fetchReports();
    });
  }

  // focusShowButton puts focus on the Show button. The Clear filter button
  // hides itself when its press succeeds, which would drop focus to the page
  // body, so it hands focus to the list head first.
  function focusShowButton() {
    var button = showButton();
    if (button) {
      button.focus();
    }
  }

  if (filterClearBtn) {
    // Clear filter: the chip goes back to All and the list is read again. The
    // browser removes nothing itself; the server answers for the whole set.
    // A chip change ends a report link (CR-01).
    filterClearBtn.addEventListener('click', function () {
      Pinalert.dropFocus();
      Pinalert.setCategory('');
      Pinalert.fetchReports();
      focusShowButton();
    });
  }

  // Boot step. The Show choice was handed to the store by show-menu.js, which
  // reads the page address once (the old /?view=disputed link opens Show
  // Disputed) and writes nothing back. Here the store is asked to hold that
  // choice, so its read word is all or disputed even when the menu script is
  // missing, and the report id of a ?report=ID link is kept for the list
  // requests (app.js sends it until the link ends). The report parameter is
  // never touched, so a link sent by a moderator stays as it was sent (D-05).
  //
  // No fetch is issued here because it does not need to be: the first
  // report fetch fires from map.js's centerOnVisitor, inside an async
  // geolocation success/failure callback, and every deferred module
  // body, including this one, runs to completion before any async
  // callback can execute. This restore is therefore guaranteed to
  // precede the first fetch, so the first request carries the right read. If
  // a future change ever moves the initial fetch into a deferred module body
  // ABOVE this one in the template's script order, that guarantee breaks and
  // this step would need its own fetch. Adding a fetch unconditionally would
  // issue a duplicate request on every single page load.
  Pinalert.setShow(Pinalert.state.show);
  Pinalert.setFocusId(new URLSearchParams(window.location.search).get(REPORT_PARAM));

  Pinalert.onSelect(function (id, source) {
    // A selection that starts from the map chooses its row the same way (D-17):
    // the row takes the soft fill, the phone sheet comes up to half so the row
    // can be seen (it is not remembered: the sheet stays at half when the
    // report closes), and the sheet's own scroll box brings the row under the
    // head. It never scrolls the page (A87). Guard on source so a
    // list-originated selection, which activateRow already handled, is not
    // handled a second time.
    if (source === 'map') {
      applySelectionHighlight(id);
      if (window.PinalertSheet && window.PinalertSheet.getSnap() !== 'half') {
        window.PinalertSheet.setSnap('half');
      }
      revealRowInListPane(id);
    }
  });

  Pinalert.subscribe(render);
  render(Pinalert.state);

  window.setInterval(refreshAgeAndTime, 60000);
  window.setInterval(refreshStaleLine, 60000);

  // The Queue and Stale rows of the notice card follow the layout, because
  // they show only while the list pane is not displayed. The breakpoint
  // listener catches the width tier change; the resize listener catches any
  // other layout change that shows or hides the pane.
  if (narrowQuery) {
    if (typeof narrowQuery.addEventListener === 'function') {
      narrowQuery.addEventListener('change', syncNoticeCard);
    } else if (typeof narrowQuery.addListener === 'function') {
      narrowQuery.addListener(syncNoticeCard);
    }
  }
  window.addEventListener('resize', syncNoticeCard);

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
    // Re-apply the store's Show choice to the store and the menu BEFORE
    // refetching (UAT gap 5's extension into gap 3): a restored frozen DOM can
    // never show one choice over another choice's rows, because the read word
    // and the menu are always re-derived from the one Show value the store
    // holds. The address carries no Show choice, so there is nothing to re-read.
    Pinalert.setShow(Pinalert.state.show);
    if (window.PinalertShowMenu && typeof PinalertShowMenu.sync === 'function') {
      PinalertShowMenu.sync();
    }
    Pinalert.fetchReports();
  });

  // The device queue mounts last. The report form was mounted by modal.js, which
  // loads before this file, so a saved report can be opened for editing at once.
  if (queueStripEl) {
    queueStripEl.addEventListener('pinalert-queue-count', function (evt) {
      queueCount = (evt && evt.detail && evt.detail.count) || 0;
      syncNoticeCard();
    });
  }
  if (noticeQueueBtn) {
    noticeQueueBtn.addEventListener('click', function () {
      // The list is always on the page now (a sheet on the phone, a column on
      // the laptop), so the button only has to bring the waiting strip into
      // view: a short sheet grows to half first.
      if (window.PinalertSheet && window.PinalertSheet.getSnap() === 'short') {
        window.PinalertSheet.setSnap('half');
      }
      var heading = document.getElementById('queue-strip-heading');
      if (heading) {
        heading.focus();
      }
    });
  }
  if (window.PinalertQueue && queueStripEl && appShellEl) {
    window.PinalertQueue.mount({
      strip: queueStripEl,
      accountId: appShellEl.getAttribute('data-account-id'),
      headingLevel: 2,
      onEdit: openSavedReport,
      focusFallback: queueFocusFallback
    });
  }
}());
