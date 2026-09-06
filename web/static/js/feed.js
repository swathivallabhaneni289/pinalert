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
// SCOPE (T-01-23): this list shows no confirmation count and no trust-
// state wording. That data belongs to the Phase 2 Trust Engine and does
// not exist yet — inventing a placeholder would put an unbacked claim in
// front of a reader, which is the exact failure mode this product exists
// to counter.
(function () {
  'use strict';

  var SEVERITY_RANK = { critical: 3, medium: 2, low: 1 };

  var CAPACITY_LABELS = (function () {
    var labels = {};
    var statuses = Pinalert.CAPACITY_STATUSES || [];
    for (var i = 0; i < statuses.length; i++) {
      var key = statuses[i];
      labels[key] = key.charAt(0).toUpperCase() + key.slice(1);
    }
    return labels;
  }());

  var SVG_NS = 'http://www.w3.org/2000/svg';

  var reportListEl = document.getElementById('report-list');
  var skeletonEl = document.getElementById('feed-skeleton');
  var emptyEl = document.getElementById('feed-empty');
  var errorEl = document.getElementById('feed-error');
  var retryBtn = document.getElementById('feed-retry');
  var viewToggleBtn = document.getElementById('view-toggle');
  var appShellEl = document.getElementById('app-shell');

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

    if (report.category === 'shelter_open') {
      var statusLabel = CAPACITY_LABELS[report.shelter_capacity_status];
      if (statusLabel) {
        text += ' · ' + statusLabel;
      }
      var headcount = report.shelter_headcount;
      if (headcount !== null && headcount !== undefined) {
        text += ' · ' + headcount + (headcount === 1 ? ' person' : ' people');
      }
    }

    return text;
  }

  function activateRow(id) {
    Pinalert.select(id, 'list');
    applySelectionHighlight(id, false);
    if (window.PinalertMap && typeof PinalertMap.flyTo === 'function') {
      PinalertMap.flyTo(id);
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
    var img = document.createElement('img');
    img.alt = '';
    badge.appendChild(img);

    var body = document.createElement('div');
    body.className = 'report-row__body';

    var title = document.createElement('p');
    title.className = 'report-row__title';

    var meta = document.createElement('p');
    meta.className = 'report-row__meta';

    body.appendChild(title);
    body.appendChild(meta);

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

    return { el: li, img: img, title: title, meta: meta };
  }

  // updateRow applies severity/age classes and text content only — it never
  // touches layout, so a background refresh cannot disturb scroll position
  // on rows whose identity (id) is unchanged.
  function updateRow(row, report) {
    replacePrefixedClass(row.el, 'sev-', Pinalert.severityClass(report));
    replacePrefixedClass(row.el, 'age-', 'age-' + Pinalert.ageStage(report));

    row.img.src = Pinalert.iconPath(report.category);

    Pinalert.setText(row.title, report.description);
    Pinalert.setText(row.meta, buildMetaText(report));

    var selectedId = Pinalert.state.selectedId;
    var isSelected = selectedId !== null && selectedId !== undefined &&
      String(selectedId) === String(report.id);
    row.el.classList.toggle('report-row--selected', isSelected);
  }

  function renderRows(reports) {
    var sorted = sortReports(reports);
    var seen = {};

    for (var i = 0; i < sorted.length; i++) {
      var report = sorted[i];
      var idKey = String(report.id);
      seen[idKey] = true;

      var row = rowsById[idKey];
      if (!row) {
        row = createRow(report.id);
        rowsById[idKey] = row;
      }
      updateRow(row, report);

      var expected = reportListEl.children[i];
      if (expected !== row.el) {
        reportListEl.insertBefore(row.el, expected || null);
      }
    }

    var existingIds = Object.keys(rowsById);
    for (var j = 0; j < existingIds.length; j++) {
      var rid = existingIds[j];
      if (!seen[rid]) {
        rowsById[rid].el.remove();
        delete rowsById[rid];
      }
    }
  }

  function clearRows() {
    var ids = Object.keys(rowsById);
    for (var i = 0; i < ids.length; i++) {
      rowsById[ids[i]].el.remove();
      delete rowsById[ids[i]];
    }
  }

  // render is the single state machine driving the list, the skeleton, the
  // empty state, and the error state — driven entirely by state.status, so
  // there is exactly one place that decides which of the four is showing.
  function render(state) {
    if (state.status === 'idle' || state.status === 'loading') {
      setHidden(skeletonEl, false);
      setHidden(reportListEl, true);
      setHidden(emptyEl, true);
      setHidden(errorEl, true);
      return;
    }

    setHidden(skeletonEl, true);

    if (state.status === 'error') {
      // A fetch failure hides the list in place of showing it, but never
      // clears the rows or the map's existing pins — stale data is more
      // useful than a blank panel to someone who just lost signal.
      setHidden(errorEl, false);
      setHidden(reportListEl, true);
      setHidden(emptyEl, true);
      return;
    }

    setHidden(errorEl, true);

    if (state.reports.length === 0) {
      setHidden(emptyEl, false);
      setHidden(reportListEl, true);
      clearRows();
      return;
    }

    setHidden(emptyEl, true);
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
      replacePrefixedClass(row.el, 'age-', 'age-' + Pinalert.ageStage(report));
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
  // dedicated map/list asset exists, and adding new /static/icons/*.svg
  // files is outside this plan's two-file ownership. Built via the SVG DOM
  // API (never a markup string), aria-hidden since the button's text label
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
}());
