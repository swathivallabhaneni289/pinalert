// stage.js: renders one frame of sketch 004 (the Activity page, one tab, one theme) and runs the rules check on what the
// browser painted. Classic script, no network. Query parameters:
//   theme   light | dark
//   tab     all | posts | votes
//   filter  all | active | resolved (only used on the My posts tab)
//   counts  1 keeps the confirmed counts, the state label and the reporter words on each row, 0 draws it like the picture
//   data    sample | none (none shows the empty states)
//   text    100 | 150 | 175 | 200 (a percentage that scales the four type size tokens, which is what a larger text size
//           does to this app)
//   id      echoed back to the board so it can route the result
(function () {
  'use strict';

  var S3 = window.S3;
  var A4 = window.A4;
  var q = new URLSearchParams(location.search);

  function pick(name, allowed, fallback) {
    var v = q.get(name);
    return allowed.indexOf(v) === -1 ? fallback : v;
  }

  var cfg = {
    theme: pick('theme', ['light', 'dark'], 'light'),
    tab: pick('tab', ['all', 'posts', 'votes'], 'all'),
    filter: pick('filter', ['all', 'active', 'resolved'], 'all'),
    counts: pick('counts', ['1', '0'], '1'),
    data: pick('data', ['sample', 'none'], 'sample'),
    text: ['100', '150', '175', '200'].indexOf(q.get('text')) === -1 ? 100 : parseInt(q.get('text'), 10),
    id: q.get('id') || ''
  };

  // The trust block is variant H of sketch 003, the owner's choice, so the same attributes switch its CSS on.
  var root = document.documentElement;
  var spec = S3.variants.h;
  root.setAttribute('data-variant', 'h');
  root.setAttribute('data-redesign', 'true');
  root.setAttribute('data-header', spec.header);
  root.setAttribute('data-button', spec.button);
  root.setAttribute('data-counts', cfg.counts);

  S3.injectMasks();
  A4.injectMasks();

  if (cfg.text !== 100) {
    var k = cfg.text / 100;
    var big = document.createElement('style');
    big.id = 'a4-text-scale';
    big.textContent = ':root{--font-size-label:' + 14 * k + 'px;--font-size-body:' + 16 * k + 'px;--font-size-heading:' + 18 * k +
      'px;--font-size-display:' + 24 * k + 'px;}';
    document.head.appendChild(big);
  }

  var page = A4.buildPage(cfg);
  document.getElementById('stage').appendChild(page.root);

  // ---- Reporting to the board ---------------------------------------------------------------------------------
  function post(message) {
    try { window.parent.postMessage(message, '*'); } catch (e) { /* standalone */ }
  }

  // The body's own height, not the document's scroll height: that one never drops below the frame's current height, so a
  // frame could only ever grow on the board.
  function postSize() {
    post({ type: 'size', id: cfg.id, height: Math.ceil(document.body.getBoundingClientRect().height) });
  }

  function runRules() {
    var result = window.A4Rules.run({ scope: page.scope, cfg: cfg });
    window.__rules = result;
    window.__ready = true;
    post({ type: 'rules', id: cfg.id, result: result });
    postSize();
  }

  var timer = 0;
  function schedule() {
    window.clearTimeout(timer);
    timer = window.setTimeout(runRules, 120);
  }

  page.onChange = schedule;
  window.__a4 = { cfg: cfg, page: page, rerun: runRules };

  document.getElementById('stage').addEventListener('click', schedule);
  if (typeof ResizeObserver === 'function') { new ResizeObserver(postSize).observe(document.body); }

  // Timers, not animation frames: a frame that is off screen on the board gets no animation frames in Safari, so a check
  // that waited for one would never run until the frame was scrolled into view. Reading the layout forces it anyway.
  function start() {
    var ready = document.fonts && document.fonts.ready ? document.fonts.ready : Promise.resolve();
    ready.then(function () { window.setTimeout(runRules, 60); });
    window.setTimeout(function () { if (!window.__ready) { runRules(); } }, 800);
  }
  if (document.readyState === 'complete') { start(); } else { window.addEventListener('load', start); }
}());
