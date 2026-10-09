// rules.js: the live rules check for sketch 003. Classic script, no network.
//
// It reads computed styles, so it judges what the browser painted. It runs the shared check from
// ../shared/rules-check.js (type sizes, weights, spacing, contrast, colour tokens, touch targets, overflow, long
// dashes, emoji) and adds the checks this sketch exists for:
//   no underline anywhere in the block or the header, at rest, hover, press and focus
//   the reporter words are plain regular weight text
//   trust text is the normal text colour
//   nothing clips and the page never scrolls sideways
//   no count splits between its number and its word unless that count alone is wider than the button
//   one tab stop, aria-expanded, a panel that follows it, no live region
//   hover and press are background fills, hover softer than press, both visible, short transitions
//   the panel wording stays in the closed character set, and the caveat appears once in the proposed set
//
// Two limits to know. text-decoration propagates to descendants rather than inheriting, so a child span can read
// "none" while its button paints an underline. The check therefore reads the toggle and every ancestor up to the
// surface, not only the text spans. And hover, press and focus are previewed with classes that share declarations
// with the real pseudo classes, so the in page check proves the CSS, while the real :hover, :active and :focus-visible
// are proven by driving the pointer and keyboard from a test browser.
window.S3Rules = (function () {
  'use strict';

  // ---- Colour helpers ---------------------------------------------------------------------------------------
  function parseColor(str) {
    if (!str) { return null; }
    str = str.trim();
    if (str === 'transparent') { return { r: 0, g: 0, b: 0, a: 0 }; }
    var m = str.match(/^#([0-9a-f]{6})$/i);
    if (m) { return { r: parseInt(m[1].slice(0, 2), 16), g: parseInt(m[1].slice(2, 4), 16), b: parseInt(m[1].slice(4, 6), 16), a: 1 }; }
    m = str.match(/^rgba?\(([^)]+)\)$/);
    if (m) {
      var p = m[1].split(/[\s,\/]+/).filter(Boolean);
      var alpha = 1;
      if (p.length > 3) { alpha = p[3].indexOf('%') > -1 ? parseFloat(p[3]) / 100 : parseFloat(p[3]); }
      return { r: parseFloat(p[0]), g: parseFloat(p[1]), b: parseFloat(p[2]), a: alpha };
    }
    m = str.match(/^color\(srgb ([^)]+)\)$/);
    if (m) {
      var q = m[1].split(/[\s\/]+/).filter(Boolean);
      return { r: parseFloat(q[0]) * 255, g: parseFloat(q[1]) * 255, b: parseFloat(q[2]) * 255, a: q.length > 3 ? parseFloat(q[3]) : 1 };
    }
    return null;
  }
  function composite(top, bottom) {
    var a = top.a;
    return { r: top.r * a + bottom.r * (1 - a), g: top.g * a + bottom.g * (1 - a), b: top.b * a + bottom.b * (1 - a), a: 1 };
  }
  function lin(c) { c = c / 255; return c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4); }
  function lum(c) { return 0.2126 * lin(c.r) + 0.7152 * lin(c.g) + 0.0722 * lin(c.b); }
  function contrast(a, b) {
    var la = lum(a), lb = lum(b);
    return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
  }
  function lab(c) {
    var r = lin(c.r), g = lin(c.g), b = lin(c.b);
    var x = 0.4124564 * r + 0.3575761 * g + 0.1804375 * b;
    var y = 0.2126729 * r + 0.7151522 * g + 0.0721750 * b;
    var z = 0.0193339 * r + 0.1191920 * g + 0.9503041 * b;
    var f = function (t) { return t > 216 / 24389 ? Math.cbrt(t) : (24389 / 27 * t + 16) / 116; };
    var fx = f(x / 0.95047), fy = f(y), fz = f(z / 1.08883);
    return [116 * fy - 16, 500 * (fx - fy), 200 * (fy - fz)];
  }
  function deltaE(a, b) {
    var A = lab(a), B = lab(b);
    return Math.sqrt(Math.pow(A[0] - B[0], 2) + Math.pow(A[1] - B[1], 2) + Math.pow(A[2] - B[2], 2));
  }
  function token(name) {
    return parseColor(getComputedStyle(document.documentElement).getPropertyValue(name));
  }
  function near(a, b) {
    return !!a && !!b && Math.abs(a.r - b.r) <= 1.5 && Math.abs(a.g - b.g) <= 1.5 && Math.abs(a.b - b.b) <= 1.5;
  }
  // The colour a node's own box sits on: its ancestors' backgrounds composited over the page colour.
  function backdrop(node, pageBg) {
    var layers = [];
    for (var n = node; n; n = n.parentElement) {
      var c = parseColor(getComputedStyle(n).backgroundColor);
      if (c && c.a > 0) { layers.push(c); if (c.a >= 1) { break; } }
    }
    var result = pageBg;
    for (var i = layers.length - 1; i >= 0; i--) { result = composite(layers[i], result); }
    return result;
  }

  function describe(node) {
    var cls = node.className && typeof node.className === 'string' ? '.' + node.className.trim().split(/\s+/).join('.') : '';
    return node.tagName.toLowerCase() + cls;
  }
  function seconds(list) {
    return String(list || '0s').split(',').map(function (s) {
      s = s.trim();
      return s.indexOf('ms') > -1 ? parseFloat(s) / 1000 : parseFloat(s);
    });
  }

  // Run a function with transitions switched off, so a forced state reads its final value and not a frame on the way.
  // The styles are flushed once more before the switch comes off, so whatever the function changed lands without a
  // transition, and the page is left at rest and not caught turning a chevron back.
  function withoutTransitions(fn) {
    var style = document.createElement('style');
    style.textContent = '*,*::before,*::after{transition:none !important;animation:none !important;}';
    document.head.appendChild(style);
    void document.documentElement.offsetWidth;
    try {
      var result = fn();
      void document.documentElement.offsetWidth;
      return result;
    } finally {
      style.remove();
      void document.documentElement.offsetWidth;
    }
  }

  var PREVIEWS = ['is-hover', 'is-press', 'is-focus'];
  function withPreview(toggle, name, fn) {
    var had = PREVIEWS.filter(function (c) { return toggle.classList.contains(c); });
    PREVIEWS.forEach(function (c) { toggle.classList.remove(c); });
    if (name) { toggle.classList.add('is-' + name); }
    void toggle.offsetWidth;
    try { return fn(); } finally {
      PREVIEWS.forEach(function (c) { toggle.classList.remove(c); });
      had.forEach(function (c) { toggle.classList.add(c); });
      void toggle.offsetWidth;
    }
  }

  // Resolved fill of the toggle in one state, over whatever sits behind it.
  function fillIn(toggle, name, pageBg) {
    return withPreview(toggle, name, function () {
      var under = backdrop(toggle.parentElement, pageBg);
      var own = parseColor(getComputedStyle(toggle).backgroundColor) || { r: 0, g: 0, b: 0, a: 0 };
      return composite(own, under);
    });
  }

  // For the README table and the tests: the numbers behind the hover and press fills of the first toggle on the page.
  function measureFills() {
    var toggle = document.querySelector('.trust-block__toggle');
    if (!toggle) { return null; }
    return withoutTransitions(function () {
      var pageBg = token('--color-bg');
      var rest = fillIn(toggle, null, pageBg);
      var hover = fillIn(toggle, 'hover', pageBg);
      var press = fillIn(toggle, 'press', pageBg);
      var border = token('--color-border');
      var round = function (c) { return [Math.round(c.r), Math.round(c.g), Math.round(c.b)]; };
      return {
        rest: round(rest), hover: round(hover), press: round(press),
        restToHover: +deltaE(rest, hover).toFixed(1),
        hoverToPress: +deltaE(hover, press).toFixed(1),
        restToPress: +deltaE(rest, press).toFixed(1),
        pressIsBorderToken: near(press, border)
      };
    });
  }

  var CLOSED_SET = /^[A-Za-z0-9 ,.'+]+$/;

  // ---- The check --------------------------------------------------------------------------------------------
  // ctx: { scope, surface, state, variant, cfg, closeButton, cardWidth }
  function run(ctx) {
    var scope = ctx.scope;
    var state = ctx.state;
    var variant = ctx.variant;
    var isBaseline = !variant.redesign;
    var results = [];       // { name, ok, details: [] , expected }
    var violations = [];

    function record(name, details, expectedWhenBaseline) {
      var ok = details.length === 0;
      var expected = isBaseline && !!expectedWhenBaseline;
      results.push({ name: name, ok: ok, expected: expected, details: details });
      if (!ok) {
        details.forEach(function (d) { violations.push({ rule: name, detail: d, expected: expected }); });
      }
    }

    var pageBg = token('--color-bg');
    var textTok = token('--color-text');
    var borderTok = token('--color-border');
    var toggle = scope.querySelector('.trust-block__toggle');
    var block = scope.querySelector('.trust-block');
    var all = [scope].concat(Array.prototype.slice.call(scope.querySelectorAll('*')));
    var visible = all.filter(function (n) { return !(n instanceof SVGElement) && getComputedStyle(n).display !== 'none' && (n === scope || n.getClientRects().length); });

    // 1. The shared design rules. The preview classes would add a fill, so the page is read at rest, and put back after,
    //    all with transitions off so a previewed fill is never caught half way to its colour.
    var base;
    withoutTransitions(function () {
      var held = toggle ? PREVIEWS.filter(function (c) { return toggle.classList.contains(c); }) : [];
      held.forEach(function (c) { toggle.classList.remove(c); });
      base = window.SketchRules.check(scope, {
        report: { severity: state.severity },
        extra: ctx.closeButton ? [ctx.closeButton] : [],
        expectWidth: ctx.cardWidth ? [ctx.cardWidth - 4, ctx.cardWidth + 6] : null,
        maxRadius: 12,
        radiusExempt: '.severity-label, .visibility-tag',
        allowMixed: state.age !== 'fresh' || state.visibility !== 'live'
      });
      held.forEach(function (c) { toggle.classList.add(c); });
    });
    var SCALE = [0, 4, 8, 16, 24, 32, 48, 64];
    var baseDetails = base.violations.filter(function (v) {
      if (ctx.cfg.text !== 100 && v.rule === 'type-size') { return false; }
      // A negative offset that is a scale step (calc(-1 * var(--space-xs))) is on the scale, as 03-UI-SPEC A53 writes it.
      var neg = v.rule === 'spacing' && /margin\w+ -(\d+)px is not on the scale/.exec(v.detail);
      if (neg && SCALE.indexOf(parseInt(neg[1], 10)) !== -1) { return false; }
      return true;
    }).map(function (v) { return v.rule + ': ' + v.element + ' (' + v.detail + ')'; });
    record('Design rules (sizes, spacing, contrast, colour tokens, targets)', baseDetails, false);

    // 2. No underline, at rest and in every previewed state. Read the toggle and every ancestor up to the
    //    surface as well as every descendant, because a decoration propagates rather than inherits.
    function underlineIssues(label) {
      var issues = [];
      var chain = [];
      for (var n = toggle; n && n !== scope.parentElement; n = n.parentElement) { chain.push(n); }
      var set = visible.concat(chain.filter(function (c) { return visible.indexOf(c) === -1; }));
      set.forEach(function (node) {
        var cs = getComputedStyle(node);
        if (cs.textDecorationLine && cs.textDecorationLine !== 'none') {
          issues.push(label + ': ' + describe(node) + ' paints ' + cs.textDecorationLine);
        }
        ['::before', '::after'].forEach(function (pe) {
          var ps = getComputedStyle(node, pe);
          if (ps.content && ps.content !== 'none' && ps.content !== 'normal') {
            var w = parseFloat(ps.width), h = parseFloat(ps.height);
            if (ps.position === 'absolute' && w >= 8 && h > 0 && h <= 2) {
              issues.push(label + ': ' + describe(node) + pe + ' draws a ' + Math.round(w) + 'px wide, ' + h + 'px tall line');
            }
          }
        });
      });
      if (block) {
        Array.prototype.forEach.call(block.querySelectorAll('.trust-block__toggle *, .trust-block__toggle'), function (node) {
          var cs = getComputedStyle(node);
          if (parseFloat(cs.borderBottomWidth) >= 1 && cs.borderBottomStyle !== 'none') {
            issues.push(label + ': ' + describe(node) + ' has a bottom border that reads as an underline');
          }
        });
      }
      return issues;
    }
    var h1 = underlineIssues('rest');
    if (toggle) {
      withoutTransitions(function () {
        ['hover', 'press', 'focus'].forEach(function (name) {
          withPreview(toggle, name, function () { h1 = h1.concat(underlineIssues(name)); });
        });
      });
    }
    record('No underline at rest, on hover, on press or on focus', h1, true);

    // 3. Reporter words: the reporter words are plain text in the header zone, not in the button.
    var h2 = [];
    var reporter = scope.querySelector('.trust-reporter');
    if (state.reporter) {
      if (isBaseline || !reporter) {
        var inButton = toggle && toggle.querySelector('.trust-signal--second');
        h2.push(inButton ? 'the reporter words sit inside the trust button, under the counts, with a user icon' : 'the reporter words are missing');
      } else {
        if (toggle && toggle.contains(reporter)) { h2.push('the reporter words are inside the button'); }
        var nodes = [reporter].concat(Array.prototype.slice.call(reporter.querySelectorAll('*')));
        nodes.forEach(function (n) {
          var cs = getComputedStyle(n);
          if (n.closest('.visually-hidden') && n !== reporter) { return; }
          if (parseInt(cs.fontWeight, 10) !== 400) { h2.push(describe(n) + ' weight ' + cs.fontWeight + ' is not 400'); }
          ['Top', 'Right', 'Bottom', 'Left'].forEach(function (s) {
            if (parseFloat(cs['border' + s + 'Width']) > 0 && cs['border' + s + 'Style'] !== 'none') { h2.push(describe(n) + ' has a border'); }
          });
          var bg = parseColor(cs.backgroundColor);
          if (bg && bg.a > 0) { h2.push(describe(n) + ' has a background'); }
          if (cs.boxShadow !== 'none') { h2.push(describe(n) + ' has a shadow'); }
          if (parseFloat(cs.borderTopLeftRadius) > 0) { h2.push(describe(n) + ' has rounded corners'); }
          if (n.matches('[class*="icon"], [class*="chip"], [class*="badge"], [class*="pill"]')) { h2.push(describe(n) + ' looks like an icon, chip, badge or pill'); }
          ['::before', '::after'].forEach(function (pe) {
            var ps = getComputedStyle(n, pe);
            if (ps.content && ps.content !== 'none' && ps.content !== 'normal') { h2.push(describe(n) + pe + ' adds generated content'); }
          });
          var fg = parseColor(cs.color);
          if (n.childNodes.length && !near(fg, textTok)) { h2.push(describe(n) + ' is not in the normal text colour'); }
        });
      }
    } else if (reporter) {
      h2.push('a reporter element is drawn for a report with no reporter signal');
    }
    record('Reporter words are plain text', h2, true);

    // 4. Trust text colour: trust text is the normal text colour. Colour is for severity only.
    var h3 = [];
    var trustTextNodes = scope.querySelectorAll('.trust-block *, .trust-reporter, .trust-reporter *');
    Array.prototype.forEach.call(trustTextNodes, function (n) {
      if (n.closest('.visually-hidden')) { return; }
      var own = false;
      for (var i = 0; i < n.childNodes.length; i++) { if (n.childNodes[i].nodeType === 3 && /\S/.test(n.childNodes[i].nodeValue)) { own = true; } }
      if (!own) { return; }
      if (!near(parseColor(getComputedStyle(n).color), textTok)) { h3.push(describe(n) + ' is ' + getComputedStyle(n).color + ', not the text colour'); }
    });
    record('Trust text is the normal text colour', h3, false);

    // 5. Nothing clips, nothing scrolls sideways.
    var h4 = [];
    var docEl = document.documentElement;
    if (docEl.scrollWidth > docEl.clientWidth + 1) { h4.push('the page scrolls sideways (' + docEl.scrollWidth + ' > ' + docEl.clientWidth + ')'); }
    visible.forEach(function (n) {
      if (n.closest('.visually-hidden')) { return; }
      var cs = getComputedStyle(n);
      var hidesX = /hidden|clip|auto|scroll/.test(cs.overflowX);
      var hidesY = /hidden|clip/.test(cs.overflowY);
      if (hidesX && n.scrollWidth > n.clientWidth + 1 && n.clientWidth > 0) { h4.push(describe(n) + ' clips sideways (' + n.scrollWidth + ' > ' + n.clientWidth + ')'); }
      if (hidesY && n.scrollHeight > n.clientHeight + 1 && n.clientHeight > 0 && !n.classList.contains('popup-card__title')) {
        h4.push(describe(n) + ' clips vertically (' + n.scrollHeight + ' > ' + n.clientHeight + ')');
      }
    });
    record('Nothing clips and nothing scrolls sideways', h4, false);

    // 5b. A count never splits between its number and its word ("7" on one line, "confirmed" on the next) unless that
    //     count on its own is wider than the room the button gives its text. A split that a line of its own would have
    //     avoided is a layout fault, the one the chevron column caused at large text sizes before the repair.
    var h4b = [];
    Array.prototype.forEach.call(scope.querySelectorAll('.trust-block__count'), function (c) {
      if (!c.getClientRects().length) { return; }
      var range = document.createRange();
      range.selectNodeContents(c);
      var tops = {};
      Array.prototype.forEach.call(range.getClientRects(), function (r) { if (r.width > 0) { tops[Math.round(r.top)] = true; } });
      if (Object.keys(tops).length < 2) { return; }
      var held = c.style.whiteSpace;
      c.style.whiteSpace = 'nowrap';
      var whole = document.createRange();
      whole.selectNodeContents(c);
      var natural = whole.getBoundingClientRect().width;
      c.style.whiteSpace = held;
      var host = c.closest('.trust-block__toggle');
      var hcs = getComputedStyle(host);
      var room = host.clientWidth - parseFloat(hcs.paddingLeft) - parseFloat(hcs.paddingRight);
      if (natural <= room + 0.5) {
        h4b.push('"' + c.textContent + '" splits between its number and its word, although it is ' + Math.round(natural) +
          'px wide and the button has ' + Math.round(room) + 'px');
      }
    });
    record('Counts keep their number and word together', h4b, false);

    // 6. The disclosure.
    var h5 = [];
    if (!toggle) {
      h5.push('no trust button found');
    } else {
      var stops = block.querySelectorAll('a[href], button, input, select, textarea, [tabindex]:not([tabindex="-1"])');
      if (stops.length !== 1) { h5.push('the block has ' + stops.length + ' tab stops, want 1'); }
      var expanded = toggle.getAttribute('aria-expanded');
      if (expanded !== 'true' && expanded !== 'false') { h5.push('aria-expanded is "' + expanded + '"'); }
      var panel = document.getElementById(toggle.getAttribute('aria-controls'));
      if (!panel || !block.contains(panel)) { h5.push('aria-controls does not point at the panel in the block'); }
      else if (panel.hidden !== (expanded !== 'true')) { h5.push('the panel and aria-expanded disagree'); }
      if (block.querySelector('[aria-live], [role="status"], [role="alert"]')) { h5.push('the block holds a live region'); }
      Array.prototype.forEach.call(toggle.querySelectorAll('*'), function (n) {
        if (!/^(SPAN)$/.test(n.tagName)) { h5.push('the button holds a ' + n.tagName.toLowerCase() + ', a button may hold phrasing content only'); }
      });
      if (toggle.type !== 'button') { h5.push('the toggle is not type="button"'); }
    }
    record('One tab stop, aria-expanded, no live region', h5, false);

    // 7. Hover and press are background fills, hover softer than press, both visible, short transitions.
    var h6 = [];
    var fills = null;
    if (toggle) {
      fills = withoutTransitions(function () {
        var rest = fillIn(toggle, null, pageBg);
        var hover = fillIn(toggle, 'hover', pageBg);
        var press = fillIn(toggle, 'press', pageBg);
        return { rest: rest, hover: hover, press: press };
      });
      var rh = deltaE(fills.rest, fills.hover);
      var hp = deltaE(fills.hover, fills.press);
      var rp = deltaE(fills.rest, fills.press);
      if (!near(fills.press, composite(borderTok, backdrop(toggle.parentElement, pageBg)))) { h6.push('the press fill is not var(--color-border)'); }
      if (rh < 2.3) { h6.push('hover is not visible, it moves the fill ' + rh.toFixed(1) + ' delta E from rest (want 2.3 or more)'); }
      if (hp < 1.5) { h6.push('hover and press are not distinct, ' + hp.toFixed(1) + ' delta E apart (want 1.5 or more)'); }
      if (!(rh < rp)) { h6.push('hover is not softer than press'); }
      // Text must stay readable on top of both fills.
      var sample = toggle.querySelector('.trust-block__counts') || toggle;
      var fg = parseColor(getComputedStyle(sample).color);
      [['hover', fills.hover], ['press', fills.press]].forEach(function (pair) {
        var c = contrast(fg, pair[1]);
        if (c < 4.5) { h6.push('text on the ' + pair[0] + ' fill is ' + c.toFixed(2) + ':1'); }
      });
      var longest = 0;
      [toggle].concat(Array.prototype.slice.call(toggle.querySelectorAll('*'))).forEach(function (n) {
        seconds(getComputedStyle(n).transitionDuration).forEach(function (s) { if (s > longest) { longest = s; } });
      });
      if (longest > 0.1501) { h6.push('a transition lasts ' + Math.round(longest * 1000) + 'ms, the limit is 150ms'); }
      if (isBaseline) {
        // Today's design has no hover fill at all: it underlines instead.
        if (rh < 0.5) { h6.push('hover shows no fill, it underlines the text instead'); }
      }
    } else {
      h6.push('no trust button found');
    }
    record('Hover and press are background fills, hover softer, both visible', h6, true);

    // 8. The disclosure cue and the open chevron.
    var cue = [];
    if (toggle) {
      var chevron = toggle.querySelector('.trust-block__chevron');
      if (!chevron) {
        cue.push('no visible disclosure cue that is not an underline');
      } else {
        if (chevron.getAttribute('aria-hidden') !== 'true') { cue.push('the chevron is not aria-hidden'); }
        var cs = getComputedStyle(chevron);
        if (!(cs.maskImage && cs.maskImage !== 'none') && !(cs.webkitMaskImage && cs.webkitMaskImage !== 'none')) { cue.push('the chevron has no mask image'); }
        if (!near(parseColor(cs.backgroundColor), textTok)) { cue.push('the chevron is not the normal text colour'); }
        var was = toggle.getAttribute('aria-expanded');
        var shut = withoutTransitions(function () { toggle.setAttribute('aria-expanded', 'false'); return getComputedStyle(chevron).transform; });
        var open = withoutTransitions(function () { toggle.setAttribute('aria-expanded', 'true'); return getComputedStyle(chevron).transform; });
        withoutTransitions(function () { toggle.setAttribute('aria-expanded', was); });
        if (shut === open) { cue.push('the chevron does not turn when the panel opens'); }
      }
    }
    record('Disclosure cue (chevron that turns)', cue, true);

    // 9. W: the panel wording.
    var w = [];
    var lines = Array.prototype.map.call(scope.querySelectorAll('.trust-block__line'), function (p) { return p.textContent; });
    lines.forEach(function (t) {
      var plain = t.replace(/re-confirming/g, 'reconfirming');
      if (!CLOSED_SET.test(plain)) { w.push('"' + t + '" leaves the closed character set'); }
      if (/first report|fades unless/i.test(t)) { w.push('"' + t + '" uses banned wording'); }
      if (/^ | $|  /.test(t)) { w.push('"' + t + '" has a leading, trailing or doubled space'); }
    });
    var caveat = lines.filter(function (t) { return /Counts (separate )?places, not votes\./.test(t); }).length;
    if (ctx.cfg.wording === 'proposed' && caveat !== 1) { w.push('the caveat appears ' + caveat + ' times in the panel, want once'); }
    record('Wording stays in the closed set, caveat once when proposed', w, false);

    return {
      violations: violations,
      results: results,
      stats: { elements: base.stats.elements, checks: results.length, passed: results.filter(function (r) { return r.ok; }).length },
      width: base.width,
      height: base.height,
      fills: fills ? {
        restToHover: +deltaE(fills.rest, fills.hover).toFixed(1),
        hoverToPress: +deltaE(fills.hover, fills.press).toFixed(1),
        restToPress: +deltaE(fills.rest, fills.press).toFixed(1)
      } : null
    };
  }

  return { run: run, measureFills: measureFills, deltaE: deltaE, parseColor: parseColor };
}());
