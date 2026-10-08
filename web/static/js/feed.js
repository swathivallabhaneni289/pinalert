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

  // DISPUTED_PARAM is the page URL's own name for the "Show disputed"
  // filter flag — deliberately the exact same literal app.js's own
  // fetchReports appends to the GET /api/reports URL (see app.js's
  // fetchReports comment), so the page and the server share one
  // vocabulary for this idea rather than two spellings of it.
  var DISPUTED_PARAM = 'show_disputed';

  var reportListEl = document.getElementById('report-list');
  var skeletonEl = document.getElementById('feed-skeleton');
  var emptyEl = document.getElementById('feed-empty');
  var disputedEmptyEl = document.getElementById('disputed-empty');
  var errorEl = document.getElementById('feed-error');
  var retryBtn = document.getElementById('feed-retry');
  var viewToggleBtn = document.getElementById('view-toggle');
  var appShellEl = document.getElementById('app-shell');
  var showDisputedToggleEl = document.getElementById('show-disputed-toggle');

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

  // setEmptyState is the sole owner of both empty-state elements. render is
  // documented as the one place that decides which of the list, skeleton,
  // empty state and error state is showing, and a fifth element would
  // otherwise turn four explicit branches into eight — every setHidden call
  // targeting either empty-state element lives in this function and
  // nowhere else. kind is one of 'none' (neither shown), 'general' (the
  // Phase 1 empty state) or 'disputed' (the shared query parameter's own
  // empty state).
  function setEmptyState(kind) {
    setHidden(emptyEl, kind !== 'general');
    setHidden(disputedEmptyEl, kind !== 'disputed');
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
    var categoryLabel = Pinalert.CATEGORY_LABELS[report.category] || report.category;
    var text = categoryLabel + ' · ' + Pinalert.relativeTime(report.created_at);

    var shelterLine = Pinalert.formatShelterLine(report);
    if (shelterLine) {
      text += ' · ' + shelterLine;
    }

    return text;
  }

  function activateRow(id) {
    Pinalert.select(id, 'list');
    applySelectionHighlight(id, false);
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

    // A26, A45: the visible severity word is the first text in the row body,
    // read before the title. The shipped row had no severity word, so nothing
    // is removed.
    var severityLabel = PinalertTrust.createSeverityLabel();
    body.appendChild(severityLabel);
    body.appendChild(title);
    body.appendChild(meta);

    // This is the insertion point 02-05's artifact table reserved for the
    // visibility tag: between the meta line and the vote controls. The tag
    // ships hidden from its own builder, so a Live report's row is
    // byte-identical to what Phase 1 rendered.
    var tag = PinalertVisibility.createVisibilityTag();
    body.appendChild(tag);

    // The shared trust block (D-20), built with no options so the row shows
    // no numbers beyond the two counters (D-09). It sits between the
    // visibility tag and the vote controls.
    var trust = PinalertTrust.createTrustBlock();
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

    return { el: li, glyph: glyph, severity: severityLabel, title: title, meta: meta, votes: block, tag: tag, trust: trust };
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

  // takeMapPopupFocus asks the map whether a poll just removed the popup that
  // held focus (A67). The map reconciles first, so the answer is ready.
  function takeMapPopupFocus() {
    return !!(window.PinalertMap && typeof PinalertMap.takeRemovedPopupFocus === 'function' &&
      PinalertMap.takeRemovedPopupFocus());
  }

  // focusAfterRemoval is the one place focus is chosen after a poll removed
  // the report that owned it (A67, in the A52 order): the first displayed
  // row among the candidates, else the map, else the view toggle, else the
  // empty state. It acts only when focus fell to the page body, and the
  // browser decides only where focus goes, never which reports exist.
  function focusAfterRemoval(candidates) {
    var active = document.activeElement;
    if (active && active !== document.body) {
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
    if (elementIsShown(viewToggleBtn)) {
      viewToggleBtn.focus();
      return;
    }
    var empties = [emptyEl, disputedEmptyEl];
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

  // render is the single state machine driving the list, the skeleton, the
  // empty state, and the error state — driven entirely by state.status, so
  // there is exactly one place that decides which of the four is showing.
  function render(state) {
    if (state.status === 'idle' || state.status === 'loading') {
      setHidden(skeletonEl, false);
      setHidden(reportListEl, true);
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
      // reader is scrolling or has focus in is not hidden again.
      var hasRows = state.reports.length > 0;
      setHidden(errorEl, false);
      setHidden(reportListEl, !hasRows);
      if (!hasRows) {
        clearRows();
      }
      setEmptyState('none');
      return;
    }

    setHidden(errorEl, true);

    if (state.reports.length === 0) {
      // The disputed empty state replaces the general one rather than
      // stacking above a populated list, because in this codebase this
      // component means "shown instead of the list" — and the checked box
      // sitting directly above it is the context that makes the narrower
      // message the right one. Read from the store's flag rather than the
      // checkbox: the store's value describes the data actually in
      // state.reports, while the checkbox describes an intent whose fetch
      // may still be in flight.
      setEmptyState(Pinalert.state.showDisputed ? 'disputed' : 'general');
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
  // directions: a list click applies it without scrolling (the row is
  // already in view — the user just activated it), while a map-originated
  // selection scrolls the matching row into view.
  function applySelectionHighlight(id, scroll) {
    var idKey = id === null || id === undefined ? null : String(id);
    var ids = Object.keys(rowsById);
    for (var i = 0; i < ids.length; i++) {
      var rid = ids[i];
      var el = rowsById[rid].el;
      if (idKey !== null && rid === idKey) {
        el.classList.add('report-row--selected');
        if (scroll) {
          el.scrollIntoView({ block: 'nearest' });
        }
      } else {
        el.classList.remove('report-row--selected');
      }
    }
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

  // readDisputedFromURL reports whether the current address bar asks for
  // the disputed view (UAT gap 5, a user-requested scope reversal dated
  // 2026-09-17 — see 02-UI-SPEC.md's dated amendment on the "Show disputed
  // filter (D-10, D-11)" section; the original spec's ephemeral,
  // non-persisting checkbox was deliberate and shipped as specified, this
  // is not a bug fix). Parsed with URLSearchParams and compared against
  // the one literal app.js's own fetchReports appends ('true'), never
  // interpolated anywhere (T-01-17) — so the page and the server can never
  // disagree about what a given URL means. An absent, misspelled or
  // hostile value collapses to false, which is D-10's closed default: the
  // fail-safe direction, since the failure mode of guessing wrong here is
  // showing a reader disputed content they did not ask for.
  function readDisputedFromURL() {
    var params = new URLSearchParams(window.location.search);
    return params.get(DISPUTED_PARAM) === 'true';
  }

  // syncDisputedURL keeps the address bar's DISPUTED_PARAM in step with
  // checked: added when true, deleted when false. Applies the update via
  // history.replaceState, the History API's in-place-update variant —
  // never its variant that adds a new Back-button-history entry — because
  // a filter toggle must not create a Back-button step, or Back would walk
  // the reader through their own filter history instead of leaving the
  // page, which is a behaviour change nobody asked for.
  function syncDisputedURL(checked) {
    var url = new URL(window.location.href);
    if (checked) {
      url.searchParams.set(DISPUTED_PARAM, 'true');
    } else {
      url.searchParams.delete(DISPUTED_PARAM);
    }
    window.history.replaceState(null, '', url);
  }

  if (showDisputedToggleEl) {
    showDisputedToggleEl.addEventListener('change', function () {
      // Refetching rather than filtering state.reports in place is
      // deliberate: the server decides which reports exist under the
      // shared query parameter (02-04), and a client-side filter would be
      // a second visibility decision — the resolver bypass T-02-04 names.
      Pinalert.setShowDisputed(showDisputedToggleEl.checked);
      Pinalert.fetchReports();
      syncDisputedURL(showDisputedToggleEl.checked);
    });

    // Restore step, not a synchronisation step — direction of trust
    // reversed as of the 2026-09-17 scope reversal above: the URL is now
    // the source of truth, and the checkbox is driven FROM it. Read the
    // URL, write the result onto the checkbox's checked property, then
    // hand the same value to the shared store's setter — never the other
    // way around, and never a fetch here. The template's own autocomplete
    // attribute is now genuinely belt-and-braces (a browser that restores
    // a checked box across a reload is immediately overwritten by this
    // read), rather than the load-bearing defence the superseded comment
    // this block replaces used to claim it was.
    //
    // No fetch is issued here because it does not need to be: the first
    // report fetch fires from map.js's centerOnVisitor, inside an async
    // geolocation success/failure callback, and every deferred module
    // body — including this one — runs to completion before any async
    // callback can execute. This restore is therefore guaranteed to
    // precede the first fetch, so the first render is never briefly
    // unfiltered. If a future change ever moves the initial fetch into a
    // deferred module body ABOVE this one in the template's script order,
    // that guarantee breaks and this restore would need its own fetch —
    // flagging that here since it is the one thing that would invalidate
    // this reasoning. Adding a fetch unconditionally would issue a
    // duplicate request on every single page load.
    var initialDisputed = readDisputedFromURL();
    showDisputedToggleEl.checked = initialDisputed;
    Pinalert.setShowDisputed(initialDisputed);
  }

  Pinalert.onSelect(function (id, source) {
    // Guard on source so a list-originated selection does not bounce back
    // and re-scroll the list it came from.
    if (source === 'map') {
      applySelectionHighlight(id, true);
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
    // Re-read the URL and re-apply it to the checkbox and the store
    // BEFORE refetching (UAT gap 5's extension into gap 3): this is what
    // finally makes true the claim the pre-reversal comment on the
    // disputed-toggle block made and could not keep — a restored frozen
    // DOM can no longer show a checked box over unfiltered data, because
    // the checked box is now always re-derived from the URL rather than
    // trusted as whatever the browser happened to restore.
    var disputed = readDisputedFromURL();
    if (showDisputedToggleEl) {
      showDisputedToggleEl.checked = disputed;
    }
    Pinalert.setShowDisputed(disputed);
    Pinalert.fetchReports();
  });
}());
