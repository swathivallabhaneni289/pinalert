// login_globe_test.mjs - runs the real web/static/js/login-globe.js in a Node
// sandbox and checks the turning Earth behind the sign in card (Phase 5 plan 07,
// D-13, UI-SPEC Surface 5, sketch 009 "For the build"):
//
//   - nothing is injected before the page has loaded and gone idle, and the
//     WebGL2 probe runs before any download: with no WebGL2 no link or script
//     element is ever created and no map is built;
//   - only an address under /globe/tiles/ is accepted for the tiles;
//   - the libraries are injected with the pinned address, SRI, crossorigin
//     anonymous and referrerpolicy no-referrer, and one map is built with the
//     raster source capped at zoom 3, globe projection, no interaction and no
//     attribution control, India (longitude 80, latitude 18) in the middle;
//   - the first full frame shows the globe and its credit (is-ready on both);
//   - under reduce motion the globe is one still frame and no animation frame
//     loop runs; the setting is followed live;
//   - a hidden tab stops the loop and a visible one resumes it;
//   - a library load error, a lost WebGL context, a map error before the first
//     frame and nothing drawn 10 seconds after the download started (time while
//     hidden not counted) all give up: the map is removed, the credit stays
//     hidden, and a library that arrives late builds no map;
//   - the turn speed is 360 degrees per 120 seconds, west to east;
//   - sizeGlobe sizes the sphere from the sign in area (never a bare viewport
//     height) and tells the map its new zoom.
//
// Run it with: node --test web/login_globe_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what login-globe.js calls: a minimal element model, fake
// timers and animation frames the test moves by hand, and a fake maplibregl.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'login-globe.js'), 'utf8');

const TILES = '/globe/tiles/{z}/{y}/{x}.jpg';
const LIB_JS = 'https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.js';
const LIB_JS_SRI = 'sha256-RamwepGJzlYFTGIKlHzPQeKR5YyV6bYVM7dAqqZe5cs=';
const LIB_CSS = 'https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.css';
const LIB_CSS_SRI = 'sha256-qx5w1Z7EBGW65+cDDaLzzPKBM/1QLmK9WY7vut/XpzI=';

class FakeEl {
  constructor(tag, id) {
    this.tagName = tag.toUpperCase();
    this.id = id || '';
    this.attrs = {};
    this.classes = new Set();
    this.styleVars = {};
    this.children = [];
    this.clientWidth = 0;
    this.offsetTop = 0;
    const self = this;
    this.classList = {
      add(c) { self.classes.add(c); },
      remove(c) { self.classes.delete(c); },
      contains(c) { return self.classes.has(c); }
    };
    this.style = { setProperty(n, v) { self.styleVars[n] = v; } };
  }
  set innerHTML(v) { throw new Error('innerHTML must never be used: ' + v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  appendChild(node) { this.children.push(node); return node; }
}

// boot builds a page, runs login-globe.js in it and hands back the controls.
function boot(opts = {}) {
  const o = {
    webgl: true, reduce: false, readyState: 'complete', idle: false, tiles: TILES,
    elements: true, width: 1440, innerHeight: 900, offsetTop: 0, hidden: false, ...opts
  };
  let now = 0;
  let nextId = 1;
  const timers = new Map();
  const rafs = new Map();
  let nextRaf = 1;
  const created = [];
  const cancelled = [];
  const maps = [];
  const docHandlers = {};
  const winHandlers = {};
  const reduceHandlers = [];
  const byId = {};

  const mk = (tag, id) => { const el = new FakeEl(tag, id); if (id) { byId[id] = el; } return el; };
  const head = new FakeEl('head');
  let scriptTag = null;
  let wrapper = null;
  let credit = null;
  if (o.elements) {
    scriptTag = mk('script', 'login-globe-script');
    scriptTag.attrs['data-tiles'] = o.tiles;
    scriptTag.attrs['data-lib-js'] = LIB_JS;
    scriptTag.attrs['data-lib-js-sri'] = LIB_JS_SRI;
    scriptTag.attrs['data-lib-css'] = LIB_CSS;
    scriptTag.attrs['data-lib-css-sri'] = LIB_CSS_SRI;
    wrapper = mk('div', 'login-globe');
    mk('div', 'login-globe-map');
    credit = mk('p', 'globe-credit');
    const gate = mk('div', 'login-gate');
    gate.clientWidth = o.width;
    gate.offsetTop = o.offsetTop;
  }

  const reduceQuery = {
    matches: o.reduce,
    addEventListener(type, fn) { if (type === 'change') { reduceHandlers.push(fn); } }
  };

  class FakeMap {
    constructor(options) {
      this.options = options;
      this.handlers = {};
      this.jumps = [];
      this.zooms = [];
      this.removed = false;
      maps.push(this);
    }
    on(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); return this; }
    fire(type, e) { (this.handlers[type] || []).forEach((fn) => fn(e)); }
    jumpTo(j) { this.jumps.push(Array.from(j.center)); }
    setZoom(z) { this.zooms.push(z); }
    remove() { this.removed = true; }
  }

  const documentFake = {
    hidden: o.hidden,
    readyState: o.readyState,
    head,
    getElementById(id) { return byId[id] || null; },
    createElement(tag) {
      created.push(tag);
      const el = new FakeEl(tag);
      if (tag === 'canvas') {
        el.getContext = (kind) => {
          if (kind === 'webgl2' && o.webgl) { return { getExtension() { return { loseContext() {} }; } }; }
          return null;
        };
      }
      return el;
    },
    addEventListener(type, fn) { (docHandlers[type] = docHandlers[type] || []).push(fn); },
    removeEventListener() {}
  };

  const sandbox = {
    document: documentFake,
    console,
    Math,
    setTimeout(fn, ms) { const id = nextId++; timers.set(id, { fn, at: now + ms }); return id; },
    clearTimeout(id) { timers.delete(id); },
    requestAnimationFrame(fn) { const id = nextRaf++; rafs.set(id, fn); return id; },
    cancelAnimationFrame(id) { cancelled.push(id); rafs.delete(id); },
    matchMedia(q) {
      assert.equal(q, '(prefers-reduced-motion: reduce)');
      return reduceQuery;
    },
    devicePixelRatio: 3,
    innerHeight: o.innerHeight,
    addEventListener(type, fn) { (winHandlers[type] = winHandlers[type] || []).push(fn); }
  };
  if (o.idle) {
    sandbox.requestIdleCallback = (fn) => { const id = nextId++; timers.set(id, { fn, at: now + 50 }); return id; };
  }
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'login-globe.js' });

  const ctl = {
    sandbox, created, maps, head, wrapper, credit, scriptTag, cancelled, reduceQuery, documentFake,
    // advance moves the fake clock and runs every timer that falls due.
    advance(ms) {
      const end = now + ms;
      for (;;) {
        let nextKey = null;
        for (const [k, t] of timers) {
          if (t.at <= end && (nextKey === null || t.at < timers.get(nextKey).at)) { nextKey = k; }
        }
        if (nextKey === null) { break; }
        const t = timers.get(nextKey);
        timers.delete(nextKey);
        now = Math.max(now, t.at);
        t.fn();
      }
      now = end;
    },
    // frames runs the animation frame callbacks every stepMs for totalMs.
    frames(totalMs, stepMs = 16) {
      for (let t = 0; t < totalMs; t += stepMs) {
        now += stepMs;
        const batch = Array.from(rafs.entries());
        rafs.clear();
        batch.forEach(([, fn]) => fn(now));
      }
    },
    pendingFrames() { return rafs.size; },
    pendingTimers() { return timers.size; },
    fireWindow(type) { (winHandlers[type] || []).forEach((fn) => fn({ type })); },
    fireDocument(type) { (docHandlers[type] || []).forEach((fn) => fn({ type })); },
    setHidden(h) { documentFake.hidden = h; ctl.fireDocument('visibilitychange'); },
    setReduce(v) { reduceQuery.matches = v; reduceHandlers.forEach((fn) => fn({ matches: v })); },
    elementsOf(tag) { return head.children.filter((c) => c.tagName === tag.toUpperCase()); },
    // loadLibs makes the injected stylesheet and script arrive.
    loadLibs() {
      sandbox.maplibregl = { Map: FakeMap };
      ctl.elementsOf('link').concat(ctl.elementsOf('script')).forEach((el) => { if (el.onload) { el.onload(); } });
    },
    // start runs the page through load and idle up to the injection.
    start() {
      if (o.readyState !== 'complete') { ctl.fireWindow('load'); }
      ctl.advance(o.idle ? 50 : 200);
    }
  };
  return ctl;
}

// ready boots a page, injects, loads the libraries and fires the map's load.
function ready(opts) {
  const c = boot(opts);
  c.start();
  c.loadLibs();
  c.maps[0].fire('load');
  return c;
}

test('with no WebGL2 nothing is downloaded and no element is created for it', () => {
  const c = boot({ webgl: false });
  c.start();
  assert.equal(c.created.filter((t) => t !== 'canvas').length, 0, 'only the probe canvas may be created');
  assert.equal(c.head.children.length, 0);
  assert.equal(c.maps.length, 0);
  assert.equal(c.sandbox.PinalertLoginGlobe.canDrawWebGL2(), false);
  assert.equal(c.wrapper.classes.has('is-ready'), false);
  assert.equal(c.credit.classes.has('is-ready'), false);
});

test('with WebGL2 the probe says yes', () => {
  const c = boot();
  assert.equal(c.sandbox.PinalertLoginGlobe.canDrawWebGL2(), true);
});

test('nothing is injected before the page has loaded and gone idle', () => {
  const c = boot({ readyState: 'loading' });
  c.advance(5000);
  assert.equal(c.head.children.length, 0, 'nothing while the page is still loading');
  c.fireWindow('load');
  assert.equal(c.head.children.length, 0, 'nothing at the load event itself, the browser must go idle first');
  c.advance(2000);
  assert.equal(c.elementsOf('link').length, 1);
  assert.equal(c.elementsOf('script').length, 1);
});

test('requestIdleCallback is used when the browser has it', () => {
  const c = boot({ idle: true });
  assert.equal(c.head.children.length, 0);
  c.advance(100);
  assert.equal(c.elementsOf('script').length, 1);
});

test('the libraries are injected with the pinned address, SRI and privacy attributes', () => {
  const c = boot();
  c.start();
  const [link] = c.elementsOf('link');
  const [script] = c.elementsOf('script');
  assert.equal(link.href, LIB_CSS);
  assert.equal(link.integrity, LIB_CSS_SRI);
  assert.equal(link.crossOrigin, 'anonymous');
  assert.equal(link.referrerPolicy, 'no-referrer');
  assert.equal(link.rel, 'stylesheet');
  assert.equal(script.src, LIB_JS);
  assert.equal(script.integrity, LIB_JS_SRI);
  assert.equal(script.crossOrigin, 'anonymous');
  assert.equal(script.referrerPolicy, 'no-referrer');
  assert.equal(script.async, true);
});

test('a tile address outside our own /globe/tiles/ route downloads nothing', () => {
  for (const tiles of ['https://gibs.earthdata.nasa.gov/x/{z}/{y}/{x}.jpg', '//evil.example/{z}/{y}/{x}.jpg', '', '/api/{z}/{y}/{x}.jpg']) {
    const c = boot({ tiles });
    c.start();
    assert.equal(c.head.children.length, 0, 'tiles ' + JSON.stringify(tiles));
    assert.equal(c.maps.length, 0);
  }
});

test('with no globe markup on the page (Low data view) the script does nothing', () => {
  const c = boot({ elements: false });
  c.start();
  assert.equal(c.head.children.length, 0);
  assert.equal(c.created.length, 0, 'not even the probe canvas');
  assert.equal(c.pendingTimers(), 0);
});

test('one map is built with the capped raster source, globe projection and no interaction', () => {
  const c = boot();
  c.start();
  assert.equal(c.maps.length, 0, 'no map before the libraries arrive');
  c.loadLibs();
  assert.equal(c.maps.length, 1);
  const o = c.maps[0].options;
  assert.equal(o.container, c.sandbox.document.getElementById('login-globe-map'));
  assert.equal(o.interactive, false);
  assert.equal(o.attributionControl, false);
  assert.equal(o.canvasContextAttributes.antialias, true);
  assert.equal(o.canvasContextAttributes.contextType, 'webgl2');
  assert.equal(o.canvasContextAttributes.powerPreference, 'low-power');
  assert.equal(o.pixelRatio, 2, 'the pixel ratio is capped at 2');
  assert.equal(o.center[0], 80);
  assert.equal(o.center[1], 18);
  assert.equal(o.style.projection.type, 'globe');
  const layers = o.style.layers;
  assert.equal(layers.length, 1);
  assert.equal(layers[0].type, 'raster');
  const sources = Object.values(o.style.sources);
  assert.equal(sources.length, 1);
  assert.equal(sources[0].type, 'raster');
  assert.equal(sources[0].maxzoom, 3);
  assert.equal(sources[0].tileSize, 256);
  assert.deepEqual(Array.from(sources[0].tiles), [TILES]);
});

test('the first full frame shows the globe and its credit together, and the loop turns', () => {
  const c = boot();
  c.start();
  c.loadLibs();
  assert.equal(c.wrapper.classes.has('is-ready'), false);
  assert.equal(c.credit.classes.has('is-ready'), false);
  c.maps[0].fire('load');
  assert.equal(c.wrapper.classes.has('is-ready'), true);
  assert.equal(c.credit.classes.has('is-ready'), true);
  assert.equal(c.pendingFrames(), 1, 'the animation frame loop is running');
  c.frames(500);
  assert.ok(c.maps[0].jumps.length > 5, 'the globe moved');
});

test('reduce motion draws one still frame with India in the middle and runs no loop', () => {
  const c = ready({ reduce: true });
  assert.equal(c.wrapper.classes.has('is-ready'), true);
  assert.equal(c.pendingFrames(), 0, 'no animation frame is queued');
  c.frames(2000);
  assert.equal(c.pendingFrames(), 0);
  assert.equal(c.maps[0].jumps.length <= 1, true, 'at most the one still frame is placed');
  if (c.maps[0].jumps.length === 1) { assert.deepEqual(c.maps[0].jumps[0], [80, 18]); }
});

test('reduce motion is followed live, both ways', () => {
  const c = ready();
  c.frames(1000);
  assert.equal(c.pendingFrames(), 1);
  c.setReduce(true);
  assert.equal(c.pendingFrames(), 0, 'the loop stops when the setting turns on');
  const last = c.maps[0].jumps[c.maps[0].jumps.length - 1];
  assert.deepEqual(last, [80, 18], 'and the globe returns to India for the still frame');
  c.setReduce(false);
  assert.equal(c.pendingFrames(), 1, 'the loop resumes when the setting turns off');
});

test('a hidden tab stops the loop and a visible one resumes it', () => {
  const c = ready();
  assert.equal(c.pendingFrames(), 1);
  c.setHidden(true);
  assert.equal(c.pendingFrames(), 0);
  const before = c.maps[0].jumps.length;
  c.frames(1000);
  assert.equal(c.maps[0].jumps.length, before, 'nothing moves while hidden');
  c.setHidden(false);
  assert.equal(c.pendingFrames(), 1);
  c.frames(500);
  assert.ok(c.maps[0].jumps.length > before);
});

test('a tab that becomes visible does not restart the loop under reduce motion', () => {
  const c = ready({ reduce: true });
  c.setHidden(true);
  c.setHidden(false);
  assert.equal(c.pendingFrames(), 0);
});

test('the turn speed is 360 degrees per 120 seconds, west to east', () => {
  const c = ready();
  const jumps = c.maps[0].jumps;
  const startAt = jumps.length;
  c.frames(60000, 33);
  // The centre longitude falls as the surface moves west to east. Unwrap the
  // jump at -180 and measure the net travel.
  let total = 0;
  let prev = startAt === 0 ? 80 : jumps[startAt - 1][0];
  for (let i = startAt; i < jumps.length; i++) {
    let step = jumps[i][0] - prev;
    if (step > 180) { step -= 360; }
    if (step < -180) { step += 360; }
    total += step;
    prev = jumps[i][0];
  }
  assert.ok(total < 0, 'the centre longitude decreases: the surface moves west to east');
  assert.ok(Math.abs(Math.abs(total) - 180) < 1, 'half a turn in 60 seconds, got ' + total);
  for (const j of jumps) { assert.equal(j[1], 18, 'the latitude is held'); }
});

test('the longitude wraps at 180 so the globe keeps turning', () => {
  const c = ready();
  c.frames(120000, 33);
  const lngs = c.maps[0].jumps.map((j) => j[0]);
  assert.ok(lngs.every((v) => v >= -180 && v <= 180), 'all longitudes stay in range');
});

test('a library load error gives up: no map, credit hidden, space kept', () => {
  const c = boot();
  c.start();
  const [script] = c.elementsOf('script');
  script.onerror();
  assert.equal(c.maps.length, 0);
  assert.equal(c.credit.classes.has('is-ready'), false);
  assert.equal(c.wrapper.classes.has('is-ready'), false);
  c.sandbox.maplibregl = { Map: class {} };
  c.loadLibs();
  assert.equal(c.maps.length, 0, 'a library that arrives late builds no map');
});

test('a stylesheet load error gives up the same way', () => {
  const c = boot();
  c.start();
  c.elementsOf('link')[0].onerror();
  c.loadLibs();
  assert.equal(c.maps.length, 0);
});

test('nothing drawn 10 seconds after the download started gives up and removes the map', () => {
  const c = boot();
  c.start();
  c.loadLibs();
  const map = c.maps[0];
  c.advance(9000);
  assert.equal(map.removed, false);
  c.advance(1100);
  assert.equal(map.removed, true);
  assert.equal(c.credit.classes.has('is-ready'), false);
  assert.equal(c.wrapper.classes.has('is-ready'), false);
  map.fire('load');
  assert.equal(c.wrapper.classes.has('is-ready'), false, 'a late load after giving up shows nothing');
  assert.equal(c.pendingFrames(), 0);
});

test('with no library at all the globe still gives up after 10 seconds, and a late arrival builds no map', () => {
  const c = boot();
  c.start();
  c.advance(10500);
  c.loadLibs();
  assert.equal(c.maps.length, 0);
});

test('the 10 seconds do not run down while the tab is hidden', () => {
  const c = boot();
  c.start();
  c.loadLibs();
  const map = c.maps[0];
  c.advance(4000);
  c.setHidden(true);
  c.advance(60000);
  assert.equal(map.removed, false, 'a hidden tab draws nothing, so nothing is given up');
  c.setHidden(false);
  c.advance(9000);
  assert.equal(map.removed, false);
  c.advance(1500);
  assert.equal(map.removed, true);
});

test('a first frame inside 10 seconds cancels the give up', () => {
  const c = ready();
  c.advance(30000);
  assert.equal(c.maps[0].removed, false);
  assert.equal(c.wrapper.classes.has('is-ready'), true);
});

test('a map error before the first frame gives up', () => {
  const c = boot();
  c.start();
  c.loadLibs();
  c.maps[0].fire('error', { error: new Error('tile refused') });
  assert.equal(c.maps[0].removed, true);
  assert.equal(c.credit.classes.has('is-ready'), false);
  c.maps[0].fire('load');
  assert.equal(c.wrapper.classes.has('is-ready'), false);
});

test('a map error after the first frame does not take the globe away', () => {
  const c = ready();
  c.maps[0].fire('error', { error: new Error('one tile late') });
  assert.equal(c.maps[0].removed, false);
  assert.equal(c.wrapper.classes.has('is-ready'), true);
});

test('a lost WebGL context gives up and keeps the empty space', () => {
  const c = ready();
  c.maps[0].fire('webglcontextlost', {});
  assert.equal(c.maps[0].removed, true);
  assert.equal(c.pendingFrames(), 0);
  assert.equal(c.credit.classes.has('is-ready'), false);
  assert.equal(c.wrapper.classes.has('is-ready'), false);
});

test('a constructor that throws gives up quietly', () => {
  const c = boot();
  c.start();
  c.sandbox.maplibregl = { Map: class { constructor() { throw new Error('no webgl after all'); } } };
  c.elementsOf('link').concat(c.elementsOf('script')).forEach((el) => el.onload());
  assert.equal(c.credit.classes.has('is-ready'), false);
  assert.equal(c.pendingFrames(), 0);
});

test('sizeGlobe sizes from the sign in area, 1440 by 900 laptop', () => {
  const c = boot({ width: 1440, innerHeight: 900 });
  const size = c.sandbox.PinalertLoginGlobe.sizeGlobe();
  assert.equal(Math.round(size.diameter), 828);
  assert.equal(c.wrapper.styleVars['--globe-w'], '878px');
  assert.equal(c.wrapper.styleVars['--globe-h'], '878px');
  assert.equal(c.wrapper.styleVars['--globe-x'], '281px');
  assert.equal(c.wrapper.styleVars['--globe-y'], '11px');
});

test('sizeGlobe on a 390 by 844 phone is wider than the screen on purpose', () => {
  const c = boot({ width: 390, innerHeight: 844 });
  const size = c.sandbox.PinalertLoginGlobe.sizeGlobe();
  assert.ok(size.diameter > 390, 'the sphere is wider than the screen');
  assert.ok(size.diameter <= 844 * 0.92 + 0.01);
  assert.equal(c.wrapper.styleVars['--globe-w'], '390px');
  assert.equal(c.wrapper.styleVars['--globe-x'], '0px');
});

test('sizeGlobe subtracts the area top and never reads a bare viewport height as the area', () => {
  const c = boot({ width: 1440, innerHeight: 900, offsetTop: 100 });
  const size = c.sandbox.PinalertLoginGlobe.sizeGlobe();
  assert.equal(Math.round(size.diameter), Math.round(800 * 0.92));
});

test('sizeGlobe tells a built map its new zoom, and a resize does it again', () => {
  const c = ready({ width: 1440, innerHeight: 900 });
  const map = c.maps[0];
  const first = map.zooms.length;
  assert.ok(map.options.zoom > 0 && map.options.zoom < 3, 'the map was built at a sensible globe zoom, got ' + map.options.zoom);
  c.sandbox.innerHeight = 700;
  c.fireWindow('resize');
  assert.ok(map.zooms.length > first, 'a resize sets the zoom again');
  const z = map.zooms[map.zooms.length - 1];
  assert.ok(z > 0 && z < 3, 'the zoom is a sensible globe zoom, got ' + z);
});
