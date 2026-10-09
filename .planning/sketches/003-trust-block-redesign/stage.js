// stage.js: renders one frame of sketch 003 (one variant, one surface, one report state, one theme) and runs the rules
// check on what the browser painted. Classic script, no network. Query parameters:
//   theme    light | dark
//   variant  current | e | f | g
//   surface  popup | row | activity
//   state    1 to 9
//   open     1 opens the panel
//   preview  rest | hover | press | focus (a still picture of the state)
//   text     100 | 150 | 175 | 200 (a percentage that scales the four type size tokens, which is what a larger text size does to this app)
//   wording  proposed | current
//   statepos today | top (rows only: the state label stays under the meta, or joins the severity label)
//   fit      1 frames the whole popup card, 0 keeps the shipped phone height scroll
//   id       echoed back to the board so it can route the result
(function () {
  'use strict';

  var S3 = window.S3;
  var q = new URLSearchParams(location.search);

  function pick(name, allowed, fallback) {
    var v = q.get(name);
    return allowed.indexOf(v) === -1 ? fallback : v;
  }

  var variantKey = pick('variant', Object.keys(S3.variants), 'f');
  var cfg = {
    theme: pick('theme', ['light', 'dark'], 'light'),
    variant: variantKey,
    surface: pick('surface', ['popup', 'row', 'activity'], 'popup'),
    state: q.get('state') || '1',
    open: q.get('open') === '1',
    preview: pick('preview', ['rest', 'hover', 'press', 'focus'], 'rest'),
    text: ['100', '150', '175', '200'].indexOf(q.get('text')) === -1 ? 100 : parseInt(q.get('text'), 10),
    wording: pick('wording', ['proposed', 'current'], S3.variants[variantKey].redesign ? 'proposed' : 'current'),
    statepos: pick('statepos', ['today', 'top'], 'today'),
    fit: q.get('fit') !== '0',
    id: q.get('id') || ''
  };

  var spec = S3.variants[cfg.variant];
  var state = S3.getState(cfg.state);
  var root = document.documentElement;

  root.setAttribute('data-variant', cfg.variant);
  root.setAttribute('data-surface', cfg.surface);
  root.setAttribute('data-fit', cfg.fit ? '1' : '0');
  if (spec.redesign) {
    root.setAttribute('data-redesign', 'true');
    root.setAttribute('data-header', spec.header);
    root.setAttribute('data-button', spec.button);
  }

  S3.injectMasks();

  if (cfg.text !== 100) {
    var k = cfg.text / 100;
    var big = document.createElement('style');
    big.id = 's3-text-scale';
    big.textContent = ':root{--font-size-label:' + 14 * k + 'px;--font-size-body:' + 16 * k + 'px;--font-size-heading:' + 18 * k +
      'px;--font-size-display:' + 24 * k + 'px;}';
    document.head.appendChild(big);
  }

  var ctx = { variant: spec, scores: cfg.surface === 'popup', wording: cfg.wording, statepos: cfg.statepos };
  var built = S3.buildSurface(cfg.surface, state, ctx);
  var stage = document.getElementById('stage');
  stage.appendChild(built.root);

  if (state.activityOnly && cfg.surface !== 'activity') {
    stage.appendChild(S3.el('p', 'stage-note', 'In the app this state shows on the Activity page only.'));
  }

  var toggles = Array.prototype.slice.call(document.querySelectorAll('.trust-block__toggle'));
  toggles.forEach(function (toggle) {
    var panel = document.getElementById(toggle.getAttribute('aria-controls'));
    if (cfg.open) { S3.setOpen(toggle, panel, true); }
    if (cfg.preview !== 'rest') { toggle.classList.add('is-' + cfg.preview); }
  });

  // Escape closes an open panel first, and on the popup a second press closes the popup. A document level listener in
  // the capture phase does it, because Safari does not focus a button on click, so after a mouse click the key would
  // never reach the block through focus.
  document.addEventListener('keydown', function (evt) {
    if (evt.key !== 'Escape') { return; }
    var openToggles = toggles.filter(function (t) { return t.getAttribute('aria-expanded') === 'true'; });
    if (!openToggles.length) {
      if (built.popup && !built.popup.hidden) {
        built.setPopupShown(false);
        evt.stopPropagation();
        evt.preventDefault();
      }
      return;
    }
    openToggles.forEach(function (t) { S3.setOpen(t, document.getElementById(t.getAttribute('aria-controls')), false); });
    var active = document.activeElement;
    if (!active || active === document.body || openToggles.indexOf(active) === -1) { openToggles[0].focus(); }
    evt.stopPropagation();
    evt.preventDefault();
    schedule();
  }, true);

  // ---- Reporting to the board ---------------------------------------------------------------------------------
  function post(message) {
    try { window.parent.postMessage(message, '*'); } catch (e) { /* standalone */ }
  }

  function postSize() {
    post({ type: 'size', id: cfg.id, height: Math.ceil(document.documentElement.scrollHeight) });
  }

  function runRules() {
    // A popup closed from its X has nothing to check until it is opened again.
    if (!built.scope.getClientRects().length) { return; }
    var popup = built.popup || null;
    var result = window.S3Rules.run({
      scope: built.scope,
      state: state,
      variant: spec,
      cfg: cfg,
      closeButton: popup ? popup.querySelector('.leaflet-popup-close-button') : null,
      cardWidth: cfg.surface === 'popup' ? Math.min(340, window.innerWidth - 32) : null
    });
    window.__rules = result;
    window.__ready = true;
    post({ type: 'rules', id: cfg.id, meta: { theme: cfg.theme, sketch: '003', id: cfg.id }, result: result });
    postSize();
  }

  var timer = 0;
  function schedule() {
    window.clearTimeout(timer);
    timer = window.setTimeout(runRules, 120);
  }

  window.__s3 = { cfg: cfg, spec: spec, state: state, built: built, rerun: runRules };

  stage.addEventListener('click', schedule);
  if (typeof ResizeObserver === 'function') { new ResizeObserver(postSize).observe(document.body); }

  function start() {
    var ready = document.fonts && document.fonts.ready ? document.fonts.ready : Promise.resolve();
    ready.then(function () {
      window.requestAnimationFrame(function () { window.requestAnimationFrame(runRules); });
    });
  }
  if (document.readyState === 'complete') { start(); } else { window.addEventListener('load', start); }
}());
