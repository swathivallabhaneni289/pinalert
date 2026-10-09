// rules.js: the live rules check for sketch 004. Classic script, no network.
//
// It reads computed styles, so it judges what the browser painted. It runs the shared check from
// ../shared/rules-check.js (type sizes, weights, spacing, contrast, borders, shadows, gradients, touch targets, overflow,
// long dashes, emoji) and adds the checks this sketch exists for:
//   tabs, count chips, the filter and the row link are 8px or less (no pills)
//   the three tabs follow the tab pattern and fit without scrolling
//   every row has one link of at least 44px, a severity label, and the trust pieces when the board asks for them
//   Reopen shows only on the poster's own resolved row
//   the source label and the severity label share one line
//   nothing scrolls sideways
//
// The shared check reads colour tokens, but a list of reports mixes severities, ages and states, so it runs with
// allowMixed (the colour rules are then left to the trust block's own contract tests, as sketch 003 does for non fresh
// states). The two 12px labels the shipped design already has are exempt from the radius rule.
window.A4Rules = (function () {
  'use strict';

  var SCALE = [0, 4, 8, 16, 24, 32, 48, 64];

  function describe(node) {
    var cls = node.className && typeof node.className === 'string' ? '.' + node.className.trim().split(/\s+/).join('.') : '';
    return node.tagName.toLowerCase() + cls;
  }

  function shown(node) {
    return getComputedStyle(node).display !== 'none' && node.getClientRects().length > 0;
  }

  // ctx: { scope, cfg }
  function run(ctx) {
    var scope = ctx.scope;
    var cfg = ctx.cfg;
    var results = [];
    var violations = [];

    function record(name, details) {
      var ok = details.length === 0;
      results.push({ name: name, ok: ok, expected: false, details: details });
      if (!ok) {
        details.forEach(function (d) { violations.push({ rule: name, detail: d, expected: false }); });
      }
    }

    // 1. The shared design rules.
    var base = window.SketchRules.check(scope, {
      maxRadius: 12,
      radiusExempt: '.severity-label, .visibility-tag',
      allowMixed: true
    });
    var baseDetails = base.violations.filter(function (v) {
      if (cfg.text !== 100 && v.rule === 'type-size') { return false; }
      // At a larger text size the tab strip scrolls sideways by design, so its tabs sit past the page edge inside it.
      if (cfg.text !== 100 && v.rule === 'overflow' && /^(button\.act-tab|span\.act-count|span\.act-tab__label)/.test(v.element)) { return false; }
      // A negative offset that is a scale step (calc(-1 * var(--space-xs))) is on the scale.
      var neg = v.rule === 'spacing' && /margin\w+ -(\d+)px is not on the scale/.exec(v.detail);
      if (neg && SCALE.indexOf(parseInt(neg[1], 10)) !== -1) { return false; }
      return true;
    }).map(function (v) { return v.rule + ': ' + v.element + ' (' + v.detail + ')'; });
    record('Design rules (sizes, spacing, contrast, borders, targets, copy)', baseDetails);

    // 2. No pills: tabs, chips, the filter and the row link stay at 8px or less on every corner.
    var corners = [];
    Array.prototype.forEach.call(scope.querySelectorAll('.act-tab, .act-count, .act-filter__btn, .act-row__link'), function (n) {
      if (!shown(n)) { return; }
      var cs = getComputedStyle(n);
      ['TopLeft', 'TopRight', 'BottomLeft', 'BottomRight'].forEach(function (c) {
        var r = parseFloat(cs['border' + c + 'Radius']);
        if (r > 8) { corners.push(describe(n) + ' ' + c + ' radius ' + r + 'px is over 8px'); }
      });
    });
    record('Tabs, count chips, filter buttons and the row link have corners of 8px or less', corners);

    // 3. The tab pattern.
    var tabIssues = [];
    var tablist = scope.querySelector('[role="tablist"]');
    var tabs = tablist ? Array.prototype.slice.call(tablist.querySelectorAll('[role="tab"]')) : [];
    if (!tablist) { tabIssues.push('no tablist'); }
    if (tabs.length !== 3) { tabIssues.push(tabs.length + ' tabs, want 3'); }
    var selected = tabs.filter(function (t) { return t.getAttribute('aria-selected') === 'true'; });
    if (selected.length !== 1) { tabIssues.push(selected.length + ' selected tabs, want 1'); }
    var focusable = tabs.filter(function (t) { return t.tabIndex === 0; });
    if (focusable.length !== 1 || (selected[0] && focusable[0] !== selected[0])) {
      tabIssues.push('focus is not rolling: want exactly the selected tab at tabindex 0');
    }
    var panel = document.getElementById('act-panel');
    tabs.forEach(function (t) {
      if (t.getAttribute('aria-controls') !== 'act-panel' || !panel || panel.getAttribute('role') !== 'tabpanel') {
        tabIssues.push(describe(t) + ' does not control the tab panel');
      }
    });
    if (panel && selected[0] && panel.getAttribute('aria-labelledby') !== selected[0].id) {
      tabIssues.push('the panel is not labelled by the selected tab');
    }
    // Selection is never colour alone: the selected tab is heavier and underlined.
    if (selected[0]) {
      var selCs = getComputedStyle(selected[0]);
      tabs.filter(function (t) { return t !== selected[0]; }).forEach(function (t) {
        var cs = getComputedStyle(t);
        if (cs.fontWeight === selCs.fontWeight) { tabIssues.push(describe(t) + ' is not lighter than the selected tab'); }
        if (cs.borderBottomColor === selCs.borderBottomColor) { tabIssues.push(describe(t) + ' has the same underline as the selected tab'); }
      });
    }
    record('Tabs follow the tab pattern, selection is weight and underline as well as colour', tabIssues);

    // 4. The three tabs fit at the normal text size. A larger text size scrolls the strip sideways by design.
    var fit = [];
    if (cfg.text === 100 && tablist && tablist.scrollWidth > tablist.clientWidth + 1) {
      fit.push('the tab strip scrolls sideways (' + tablist.scrollWidth + ' > ' + tablist.clientWidth + ')');
    }
    record('The three tabs fit without scrolling at the normal text size', fit);

    // 5. Rows.
    var rowIssues = [];
    var headIssues = [];
    var rows = Array.prototype.slice.call(scope.querySelectorAll('.act-row')).filter(shown);
    rows.forEach(function (row) {
      var name = row.getAttribute('data-report') + ' ' + row.getAttribute('data-kind');
      var links = row.querySelectorAll('a[href]');
      if (links.length !== 1) {
        rowIssues.push(name + ' has ' + links.length + ' links, want 1');
      } else {
        var lr = links[0].getBoundingClientRect();
        if (lr.height < 43.9 || lr.width < 43.9) { rowIssues.push(name + ' link is ' + Math.round(lr.width) + ' x ' + Math.round(lr.height)); }
        var chev = links[0].querySelector('.act-row__chevron');
        if (!chev || chev.getAttribute('aria-hidden') !== 'true') { rowIssues.push(name + ' arrow is missing or not aria-hidden'); }
      }
      var source = row.querySelector('.act-row__source');
      var severity = row.querySelector('.severity-label');
      if (!source) { rowIssues.push(name + ' has no source label'); }
      if (!severity) { rowIssues.push(name + ' has no severity label'); }
      // At a larger text size the two labels wrap onto two lines by design, so this only holds at the normal size.
      if (source && severity && cfg.text === 100) {
        var a = source.getBoundingClientRect();
        var b = severity.getBoundingClientRect();
        if (b.top >= a.bottom - 0.5) { headIssues.push(name + ': the severity label wraps under the source label'); }
      }

      var block = row.querySelector('.trust-block');
      if (cfg.counts === '1') {
        if (!block || !shown(block)) {
          rowIssues.push(name + ' has no trust block');
        } else if (block.querySelectorAll('a[href], button').length !== 1) {
          rowIssues.push(name + ' trust block does not have exactly one tab stop');
        }
      } else if (block && shown(block)) {
        rowIssues.push(name + ' still shows the trust block with counts dropped');
      }

      var reopen = row.querySelector('.vote-btn--reopen');
      var wantReopen = row.getAttribute('data-kind') === 'post' && row.classList.contains('vis-retracted');
      if (!!reopen !== wantReopen) {
        rowIssues.push(name + (wantReopen ? ' should show Reopen' : ' should not show Reopen'));
      }
    });
    record('Every row: one 44px link, a source label, a severity label, the trust pieces as asked, Reopen only where it applies', rowIssues);
    record('The source label and the severity label share one line', headIssues);

    // 6. Nothing scrolls sideways.
    var scroll = [];
    var docEl = document.documentElement;
    if (docEl.scrollWidth > docEl.clientWidth + 1) { scroll.push('the page scrolls sideways (' + docEl.scrollWidth + ' > ' + docEl.clientWidth + ')'); }
    record('Nothing scrolls sideways', scroll);

    return {
      violations: violations,
      results: results,
      stats: { elements: base.stats.elements, checks: results.length, passed: results.filter(function (r) { return r.ok; }).length },
      width: base.width,
      height: base.height
    };
  }

  return { run: run };
}());
