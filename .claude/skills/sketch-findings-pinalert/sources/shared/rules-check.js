// rules-check.js: runs the owner's design rules against a rendered mockup and reports violations.
// It reads computed styles, so it judges what the browser actually paints, not what the CSS says.
window.SketchRules = (function () {
  'use strict';

  var SIZES = [14, 16, 18, 24];
  var WEIGHTS = [400, 600];
  var SPACING = [0, 4, 8, 16, 24, 32, 48, 64];
  var MAX_RADIUS = 8;
  var SPACING_PROPS = ['paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft',
    'marginTop', 'marginRight', 'marginBottom', 'marginLeft', 'rowGap', 'columnGap'];
  var TOKEN_NAMES = ['--color-bg', '--color-surface', '--color-border', '--color-text', '--color-text-muted',
    '--color-severity-low', '--color-severity-medium', '--color-severity-critical',
    '--color-severity-low-bg', '--color-severity-medium-bg', '--color-severity-critical-bg', '--color-age-stale'];

  function parseColor(str) {
    if (!str) { return null; }
    str = str.trim();
    if (str === 'transparent') { return { r: 0, g: 0, b: 0, a: 0 }; }
    var m = str.match(/^#([0-9a-f]{6})$/i);
    if (m) {
      return { r: parseInt(m[1].slice(0, 2), 16), g: parseInt(m[1].slice(2, 4), 16), b: parseInt(m[1].slice(4, 6), 16), a: 1 };
    }
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

  function channel(v) {
    v = v / 255;
    return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
  }

  function luminance(c) {
    return 0.2126 * channel(c.r) + 0.7152 * channel(c.g) + 0.0722 * channel(c.b);
  }

  function contrast(a, b) {
    var la = luminance(a);
    var lb = luminance(b);
    return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
  }

  function composite(top, bottom) {
    var a = top.a;
    return {
      r: top.r * a + bottom.r * (1 - a),
      g: top.g * a + bottom.g * (1 - a),
      b: top.b * a + bottom.b * (1 - a),
      a: 1
    };
  }

  function readTokens() {
    var cs = getComputedStyle(document.documentElement);
    var out = {};
    TOKEN_NAMES.forEach(function (name) { out[name] = parseColor(cs.getPropertyValue(name)); });
    return out;
  }

  function effectiveBackground(node, pageBg) {
    var layers = [];
    for (var n = node; n; n = n.parentElement) {
      var c = parseColor(getComputedStyle(n).backgroundColor);
      if (c && c.a > 0) {
        layers.push(c);
        if (c.a >= 1) { break; }
      }
    }
    var result = pageBg;
    for (var i = layers.length - 1; i >= 0; i--) { result = composite(layers[i], result); }
    return result;
  }

  function close(a, b) {
    return Math.abs(a.r - b.r) <= 1.5 && Math.abs(a.g - b.g) <= 1.5 && Math.abs(a.b - b.b) <= 1.5;
  }

  function chromatic(c) {
    return Math.max(c.r, c.g, c.b) - Math.min(c.r, c.g, c.b) > 24;
  }

  function px(value) {
    if (value === 'normal' || value === 'auto' || !value) { return 0; }
    return parseFloat(value);
  }

  function ownText(node) {
    for (var i = 0; i < node.childNodes.length; i++) {
      if (node.childNodes[i].nodeType === 3 && /\S/.test(node.childNodes[i].nodeValue)) { return true; }
    }
    return false;
  }

  function visibleText(node) {
    var clone = node.cloneNode(true);
    var hidden = clone.querySelectorAll('.visually-hidden');
    for (var i = 0; i < hidden.length; i++) { hidden[i].remove(); }
    return clone.textContent.replace(/\s+/g, ' ').trim();
  }

  function describe(node) {
    var cls = node.className && typeof node.className === 'string' ? '.' + node.className.trim().split(/\s+/).join('.') : '';
    var text = visibleText(node).slice(0, 28);
    return node.tagName.toLowerCase() + cls + (text ? ' "' + text + '"' : '');
  }

  var DASH = /[–—―]/;
  var EMOJI = /\p{Extended_Pictographic}/u;

  // scope: the element to lint. opts.extra: more elements to lint (the Leaflet close button).
  // opts.report: the sample report, opts.allowMixed: age and provisional states legitimately mix tokens.
  // opts.expectedOrder: strings that must appear once each, top to bottom. opts.expectWidth: [min, max].
  function check(scope, opts) {
    opts = opts || {};
    var violations = [];
    var tokens = readTokens();
    var pageBg = tokens['--color-bg'];
    var severity = opts.report ? opts.report.severity : null;
    var nodes = [scope].concat(Array.prototype.slice.call(scope.querySelectorAll('*')));
    (opts.extra || []).forEach(function (extra) { if (extra) { nodes.push(extra); } });
    var stats = { elements: 0, sizes: {}, weights: {}, spacing: {}, colors: {} };

    function fail(rule, node, detail) {
      violations.push({ rule: rule, element: describe(node), detail: detail });
    }

    var scopeRect = scope.getBoundingClientRect();

    nodes.forEach(function (node) {
      if (node instanceof SVGElement) { return; }
      var cs = getComputedStyle(node);
      if (cs.display === 'none') { return; }
      if (node !== scope && !node.getClientRects().length) { return; }
      stats.elements += 1;
      var hiddenUtility = node.classList.contains('visually-hidden');
      var rect = node.getBoundingClientRect();

      // Type: only four sizes, only two weights, the system UI stack.
      // Inactive controls are exempt from the contrast rule (WCAG 1.4.3). The too far reason is shown as its own
      // full contrast sentence beside them.
      var inactive = node.disabled === true;
      if (ownText(node) && !hiddenUtility) {
        var size = px(cs.fontSize);
        var weight = parseInt(cs.fontWeight, 10);
        stats.sizes[size] = (stats.sizes[size] || 0) + 1;
        stats.weights[weight] = (stats.weights[weight] || 0) + 1;
        if (SIZES.indexOf(size) === -1) { fail('type-size', node, size + 'px is not one of 14, 16, 18, 24'); }
        if (WEIGHTS.indexOf(weight) === -1) { fail('type-weight', node, weight + ' is not 400 or 600'); }
        if (cs.fontFamily.indexOf('-apple-system') !== 0) { fail('type-family', node, cs.fontFamily.slice(0, 40)); }

        // Contrast: 4.5 to 1 for every piece of text.
        var fg = parseColor(cs.color);
        var bg = effectiveBackground(node, pageBg);
        if (fg) {
          var text = fg.a < 1 ? composite(fg, bg) : fg;
          var ratio = contrast(text, bg);
          if (ratio < 4.5 && !inactive) { fail('contrast', node, ratio.toFixed(2) + ':1 is under 4.5:1'); }
        }
      }

      // Spacing: only the scale. Leaflet's own popup shell is not part of the design and is skipped.
      if (!hiddenUtility) {
        SPACING_PROPS.forEach(function (prop) {
          // A centred page column has auto side margins, which are layout, not a spacing choice.
          if (node === scope && (prop === 'marginLeft' || prop === 'marginRight')) { return; }
          var v = px(cs[prop]);
          if (SPACING.indexOf(v) === -1) { fail('spacing', node, prop + ' ' + v + 'px is not on the scale'); }
          else { stats.spacing[v] = (stats.spacing[v] || 0) + 1; }
        });
      }

      // Borders: hairlines, plus the 4px severity bar and nothing else.
      ['Top', 'Right', 'Bottom', 'Left'].forEach(function (side) {
        var w = px(cs['border' + side + 'Width']);
        if (cs['border' + side + 'Style'] === 'none') { return; }
        if (w === 0 || w === 1) { return; }
        if (w === 4 && side === 'Left') { return; }
        fail('border-width', node, 'border-' + side.toLowerCase() + ' ' + w + 'px');
      });

      // Corners: modest radius. Circles (badge, back link) are fine.
      var radius = cs.borderTopLeftRadius.split(' ')[0];
      var isCircle = radius.indexOf('%') > -1 ? parseFloat(radius) >= 50 : px(radius) >= Math.min(rect.width, rect.height) / 2 - 0.5;
      var maxR = opts.maxRadius || MAX_RADIUS;
      if (!isCircle && px(radius) > maxR && !(opts.radiusExempt && node.matches(opts.radiusExempt))) { fail('radius', node, radius + ' is over ' + maxR + 'px'); }

      // No gradients, no stray shadows.
      if (/gradient/.test(cs.backgroundImage) || /gradient/.test(cs.borderImageSource)) { fail('gradient', node, cs.backgroundImage.slice(0, 60)); }
      if (cs.boxShadow !== 'none') {
        var blur = (cs.boxShadow.match(/(-?\d+(?:\.\d+)?)px/g) || []).map(parseFloat)[2] || 0;
        var alphaMatch = cs.boxShadow.match(/rgba\([^)]*,\s*([0-9.]+)\)/);
        var shadowAlpha = alphaMatch ? parseFloat(alphaMatch[1]) : 1;
        if (blur > 8 || shadowAlpha > 0.35) { fail('shadow', node, cs.boxShadow.slice(0, 70)); }
      }

      // Touch targets: 44px minimum on anything you can press.
      if (!hiddenUtility && node.matches('a[href], button, input:not([type="hidden"]), textarea, select, [role="button"]')) {
        if (rect.height < 43.9 || rect.width < 43.9) { fail('touch-target', node, Math.round(rect.width) + ' x ' + Math.round(rect.height) + ' is under 44 x 44'); }
      }

      // Colour: every colour is a token, and chromatic colour is the report's own severity only.
      var used = [['color', ownText(node) && !hiddenUtility ? cs.color : null], ['background', cs.backgroundColor]];
      ['Top', 'Right', 'Bottom', 'Left'].forEach(function (side) {
        if (px(cs['border' + side + 'Width']) > 0 && cs['border' + side + 'Style'] !== 'none') { used.push(['border', cs['border' + side + 'Color']]); }
      });
      used.forEach(function (pair) {
        var c = pair[1] ? parseColor(pair[1]) : null;
        if (!c || c.a === 0) { return; }
        var known = null;
        TOKEN_NAMES.forEach(function (name) { if (!known && close(c, tokens[name])) { known = name; } });
        if (known) {
          stats.colors[known] = (stats.colors[known] || 0) + 1;
          if (/severity/.test(known) && severity && known.indexOf('severity-' + severity) === -1) {
            fail('colour-scope', node, known + ' used on a ' + severity + ' report');
          }
          return;
        }
        if (opts.allowMixed) { return; }
        fail(chromatic(c) ? 'colour-scope' : 'colour-token', node, pair[0] + ' ' + pair[1] + ' is not a token');
      });

      // Overflow: nothing pokes outside the component.
      if (!hiddenUtility && node !== scope && rect.width > 0 && (rect.right > scopeRect.right + 0.5 || rect.left < scopeRect.left - 0.5)) {
        if (!node.classList.contains('leaflet-popup-close-button')) { fail('overflow', node, 'outside the component horizontally'); }
      }

      // Copy: no long dashes, no emoji, in text or in attributes.
      var strings = [ownText(node) ? node.textContent : ''];
      ['aria-label', 'title', 'alt', 'placeholder'].forEach(function (a) { strings.push(node.getAttribute(a) || ''); });
      strings.forEach(function (s) {
        if (DASH.test(s)) { fail('dash', node, 'contains an em or en dash'); }
        if (EMOJI.test(s)) { fail('emoji', node, 'contains an emoji'); }
      });
    });

    if (opts.expectWidth) {
      var w = scopeRect.width;
      if (w < opts.expectWidth[0] || w > opts.expectWidth[1]) { fail('width', scope, Math.round(w) + 'px is outside ' + opts.expectWidth.join(' to ') + 'px'); }
    }

    if (opts.expectedOrder) {
      var all = Array.prototype.slice.call(scope.querySelectorAll('*'));
      var prev = null;
      opts.expectedOrder.forEach(function (want) {
        var hits = all.filter(function (n) { return visibleText(n) === want; });
        hits = hits.filter(function (n) { return !hits.some(function (o) { return o !== n && n.contains(o); }); });
        if (hits.length !== 1) { fail('order', scope, '"' + want + '" found ' + hits.length + ' times'); return; }
        var rect = hits[0].getBoundingClientRect();
        // Items that share a row overlap vertically and are fine. Only an item that sits wholly above the
        // one before it breaks the specified top to bottom order.
        if (prev && rect.bottom <= prev.top + 0.5) { fail('order', hits[0], 'sits wholly above the item before it'); }
        prev = rect;
      });
    }

    return { violations: violations, stats: stats, width: Math.round(scopeRect.width), height: Math.round(scopeRect.height) };
  }

  return { check: check };
}());
