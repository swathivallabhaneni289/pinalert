// lib.js: the DOM builders for sketch 004, the Activity page. Mockup only, classic script, no network, no markup sinks:
// every string reaches the DOM through textContent or a text node, the same rule the app follows.
//
// The trust block, the state label, the reporter words and the Reopen control are not redrawn here. Each row asks sketch
// 003's builder (variant H, the owner's choice) for its Activity row and moves those pieces into the new row layout, so
// the part of the row that carries the trust signals is the code the redesign already specifies.
window.A4 = window.A4 || {};
(function (A4) {
  'use strict';

  var S3 = window.S3;
  var el = S3.el;

  // ---- Icon masks -------------------------------------------------------------------------------------------
  // The app masks its icons through root relative urls such as /static/icons/arrow-left.svg, which do not resolve when
  // this page is served from the repo root, so the sketch redeclares the two icons it adds as data URIs, after the app
  // stylesheets, as sketch 003 does for the rest. The Lucide paths are the ones in web/static/icons. The chevron right is
  // new: the app has no chevron-right.svg yet.
  var ICON = {
    chevronRight: '<path d="m9 18 6-6-6-6"/>',
    arrowLeft: '<path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>'
  };

  function maskUrl(inner) {
    var svg = "<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' " +
      "stroke-width='2' stroke-linecap='round' stroke-linejoin='round'>" + inner + '</svg>';
    return 'url("data:image/svg+xml,' + encodeURIComponent(svg) + '")';
  }

  A4.injectMasks = function () {
    if (document.getElementById('a4-masks')) { return; }
    var chevron = maskUrl(ICON.chevronRight);
    var arrow = maskUrl(ICON.arrowLeft);
    var style = document.createElement('style');
    style.id = 'a4-masks';
    style.textContent =
      '.act-row__chevron{-webkit-mask-image:' + chevron + ';mask-image:' + chevron + ';}\n' +
      '.auth-icon--arrow-left{-webkit-mask-image:' + arrow + ';mask-image:' + arrow + ';}\n';
    document.head.appendChild(style);
  };

  // ---- One row ----------------------------------------------------------------------------------------------
  // Order, top to bottom: the source label with the severity label on the right, the title with the arrow, the meta line,
  // the state line (state label and the reporter words, variant H), the trust block, and Reopen when it applies. The
  // first two sit inside one link so the link is one big target and one name for a screen reader.
  function buildRow(ev, toast) {
    var state = A4.stateFor(ev);
    var ctx = { variant: S3.variants.h, scores: false, wording: 'current', statepos: 'today' };
    var built = S3.buildSurface('activity', state, ctx);
    var src = built.scope;
    var badge = src.querySelector('.icon-badge');
    var severity = src.querySelector('.severity-label');
    var stateline = src.querySelector('.report-row__stateline');
    var block = src.querySelector('.trust-block');
    var reopen = src.querySelector('.vote-controls');
    var reopenError = src.querySelector('.vote-error');

    var li = el('li', 'report-row act-row ' + S3.stateClasses(state) + (ev.later ? ' is-later' : ''));
    li.setAttribute('data-kind', ev.kind);
    li.setAttribute('data-report', String(state.reportId));
    li.setAttribute('data-bucket', A4.bucket(state));
    if (ev.later) { li.setAttribute('data-later', 'phase-3.2'); }

    var body = el('div', 'report-row__body');

    var link = el('a', 'act-row__link');
    link.href = '/?report=' + state.reportId;
    var head = el('span', 'act-row__head');
    head.appendChild(el('span', 'act-row__source', A4.SOURCE_LABELS[ev.kind]));
    head.appendChild(severity);
    var titleline = el('span', 'act-row__titleline');
    titleline.appendChild(el('span', 'report-row__title', state.description));
    var chevron = el('span', 'act-row__chevron');
    chevron.setAttribute('aria-hidden', 'true');
    titleline.appendChild(chevron);
    link.appendChild(head);
    link.appendChild(titleline);
    body.appendChild(link);

    body.appendChild(el('p', 'report-row__meta', A4.metaFor(ev, state)));
    if (stateline) { body.appendChild(stateline); }
    body.appendChild(block);
    if (reopen) {
      reopen.addEventListener('click', function (evt) {
        evt.stopPropagation();
        toast('Reopen sends the report back to Live, as it does today.');
      });
      body.appendChild(reopen);
      if (reopenError) { body.appendChild(reopenError); }
    }

    li.appendChild(badge);
    li.appendChild(body);

    // A pointer or a finger can press anywhere on the card. The link is the way in for a keyboard and a screen reader,
    // and the trust block and Reopen keep their own clicks.
    li.addEventListener('click', function (evt) {
      if (evt.target.closest('a, button, .trust-block, .vote-controls')) { return; }
      link.click();
    });
    return li;
  }

  // ---- The page ---------------------------------------------------------------------------------------------
  var TABS = [
    { key: 'all', label: 'All' },
    { key: 'posts', label: 'My posts' },
    { key: 'votes', label: 'My votes' }
  ];
  var FILTERS = [
    { key: 'all', label: 'All' },
    { key: 'active', label: 'Active' },
    { key: 'resolved', label: 'Resolved' }
  ];
  var EMPTY = {
    all: ['No activity yet', 'Reports you post and votes you cast will appear here.'],
    posts: ['No reports yet', 'Reports you submit will appear here.'],
    votes: ['No votes yet', 'Reports you confirm or dispute will appear here.'],
    filtered: ['Nothing in this view', 'None of your reports match this filter.']
  };

  // cfg: { tab, filter, counts, data }. The returned api holds the page, its scope for the rules check, and a render()
  // that redraws the list for the current cfg.
  A4.buildPage = function (cfg) {
    var events = cfg.data === 'none' ? [] : A4.EVENTS.slice();
    var counts = A4.counts(events);
    var api = { cfg: cfg, onChange: null };

    // The app's own toast element and styles, so a press on a row says where it would go.
    var toastEl = document.getElementById('toast');
    if (!toastEl) {
      toastEl = el('div');
      toastEl.id = 'toast';
      toastEl.setAttribute('role', 'status');
      toastEl.setAttribute('aria-live', 'polite');
      toastEl.hidden = true;
      document.body.appendChild(toastEl);
    }
    var toastTimer = 0;
    function toast(message) {
      toastEl.textContent = message;
      toastEl.hidden = false;
      window.clearTimeout(toastTimer);
      toastTimer = window.setTimeout(function () { toastEl.hidden = true; }, 3000);
    }
    api.toast = toast;

    var page = el('main', 'profile-page');

    var back = el('a', 'profile-back-link');
    back.href = '/';
    back.setAttribute('aria-label', 'Back to map');
    back.title = 'Back to map';
    var arrow = el('span', 'auth-icon auth-icon--arrow-left');
    arrow.setAttribute('aria-hidden', 'true');
    back.appendChild(arrow);
    page.appendChild(back);

    var section = el('section', 'profile-activity act');
    section.appendChild(el('h2', null, 'Activity'));
    section.appendChild(el('p', 'act-intro', 'What is happening with your reports.'));

    // Tabs. Automatic activation, roving tabindex, arrow keys, Home and End.
    var tablist = el('div', 'act-tabs');
    tablist.setAttribute('role', 'tablist');
    tablist.setAttribute('aria-label', 'Activity views');
    var tabButtons = {};
    TABS.forEach(function (t) {
      var btn = el('button', 'act-tab');
      btn.type = 'button';
      btn.id = 'act-tab-' + t.key;
      btn.setAttribute('role', 'tab');
      btn.setAttribute('aria-controls', 'act-panel');
      var label = el('span', 'act-tab__label', t.label);
      label.setAttribute('data-label', t.label);
      btn.appendChild(label);
      var n = t.key === 'posts' ? counts.posts : (t.key === 'votes' ? counts.votes : 0);
      if (n > 0) { btn.appendChild(el('span', 'act-count', String(n))); }
      btn.addEventListener('click', function () { select(t.key, false); });
      tabButtons[t.key] = btn;
      tablist.appendChild(btn);
    });
    tablist.addEventListener('keydown', function (evt) {
      var order = TABS.map(function (t) { return t.key; });
      var i = order.indexOf(cfg.tab);
      var next = null;
      if (evt.key === 'ArrowRight') { next = order[(i + 1) % order.length]; }
      else if (evt.key === 'ArrowLeft') { next = order[(i + order.length - 1) % order.length]; }
      else if (evt.key === 'Home') { next = order[0]; }
      else if (evt.key === 'End') { next = order[order.length - 1]; }
      if (next) {
        evt.preventDefault();
        select(next, true);
      }
    });
    section.appendChild(tablist);

    // The Active and Resolved filter, only inside My posts.
    var filter = el('div', 'act-filter');
    filter.setAttribute('role', 'group');
    filter.setAttribute('aria-label', 'Show my reports that are');
    var filterButtons = {};
    FILTERS.forEach(function (f) {
      var btn = el('button', 'act-filter__btn', f.label);
      btn.type = 'button';
      btn.addEventListener('click', function () {
        cfg.filter = f.key;
        render();
        changed();
      });
      filterButtons[f.key] = btn;
      filter.appendChild(btn);
    });
    section.appendChild(filter);

    var panel = el('div');
    panel.id = 'act-panel';
    panel.setAttribute('role', 'tabpanel');
    var status = el('p', 'visually-hidden');
    status.setAttribute('role', 'status');
    status.setAttribute('aria-live', 'polite');
    var list = el('ol', 'profile-report-list act-list');
    var empty = el('div', 'empty-state profile-empty-state');
    var emptyTitle = el('h3');
    var emptyText = el('p');
    empty.appendChild(emptyTitle);
    empty.appendChild(emptyText);
    var note = el('p', 'act-note',
      'The dashed rows show where Phase 3.2 updates would sit. Every other row uses data the app already has.');
    panel.appendChild(status);
    panel.appendChild(list);
    panel.appendChild(empty);
    panel.appendChild(note);
    section.appendChild(panel);
    page.appendChild(section);

    // Page level: a link in the sketch never leaves the sketch.
    page.addEventListener('click', function (evt) {
      var a = evt.target.closest('a[href]');
      if (!a) { return; }
      evt.preventDefault();
      if (a.classList.contains('profile-back-link')) { toast('Back to the map.'); return; }
      toast('Opens ' + a.getAttribute('href') + ' on the map. Phase 3.1 adds this link.');
    });

    function changed() { if (typeof api.onChange === 'function') { api.onChange(); } }

    function select(key, focus) {
      cfg.tab = key;
      render();
      if (focus) { tabButtons[key].focus(); }
      changed();
    }

    function eventsFor() {
      var rows = events.filter(function (ev) {
        if (cfg.tab === 'posts') { return ev.kind === 'post'; }
        if (cfg.tab === 'votes') { return ev.kind === 'vote' || ev.kind === 'dispute'; }
        return true;
      });
      if (cfg.tab === 'posts' && cfg.filter !== 'all') {
        rows = rows.filter(function (ev) { return A4.bucket(A4.stateFor(ev)) === cfg.filter; });
      }
      rows.sort(function (a, b) { return a.minutes - b.minutes; });
      return rows;
    }

    function render() {
      var rows = eventsFor();

      TABS.forEach(function (t) {
        var on = t.key === cfg.tab;
        tabButtons[t.key].setAttribute('aria-selected', on ? 'true' : 'false');
        tabButtons[t.key].tabIndex = on ? 0 : -1;
      });
      panel.setAttribute('aria-labelledby', 'act-tab-' + cfg.tab);

      var showFilter = cfg.tab === 'posts' && counts.posts > 0;
      filter.hidden = !showFilter;
      FILTERS.forEach(function (f) {
        filterButtons[f.key].setAttribute('aria-pressed', f.key === cfg.filter ? 'true' : 'false');
      });

      list.textContent = '';
      rows.forEach(function (ev) { list.appendChild(buildRow(ev, toast)); });
      list.hidden = rows.length === 0;

      var key = cfg.tab;
      if (rows.length === 0 && cfg.tab === 'posts' && counts.posts > 0) { key = 'filtered'; }
      emptyTitle.textContent = EMPTY[key][0];
      emptyText.textContent = EMPTY[key][1];
      empty.hidden = rows.length > 0;

      note.hidden = !(cfg.tab === 'all' && rows.some(function (ev) { return ev.later; }));
      status.textContent = rows.length === 1 ? 'Showing 1 item.' : 'Showing ' + rows.length + ' items.';
    }

    api.root = page;
    api.scope = page;
    api.render = render;
    render();
    return api;
  };
}(window.A4));
