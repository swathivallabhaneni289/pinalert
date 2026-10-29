// official_pins_test.mjs - runs the real web/static/js/official-pins.js in a Node
// sandbox and checks the GDACS hazard layer of Phase 5 plan 22 (ROBUST-05,
// ROBUST-06, D-12, UI-SPEC Surface 3, P5-C14):
//
//   - drawing: a fresh answer from GET /api/official/pins draws one marker per
//     pin through PinalertMap.addOverlay, each named "GDACS, <name>, <level
//     word>" and drawn as the flat official pin (the word GDACS and the hazard
//     glyph) inside a 44px marker box; a pin of a kind or a place the layer
//     does not know is skipped;
//   - honesty (T-05-118): a stale or unavailable answer, a failed request and an
//     answer that is not shaped like the route's draw nothing and clear what was
//     drawn before, so an old hazard is never shown as current;
//   - the card: Enter, Space and a tap on a marker call PinalertMap.openCard
//     with the card node; the card holds the GDACS badge, the level chip word,
//     the title, Hazard, Issued, Until, Issued by GDACS, the severity text, the
//     honesty line and the credit, and no vote, trust, Flag or Mute part;
//   - the link (T-05-115): Open GDACS page is drawn only for a link that starts
//     with https://www.gdacs.org/ and opens in a new tab with noopener
//     noreferrer; an http link, a javascript link and a look alike host drop it;
//   - text (T-05-114): a name holding markup appears as text, and the fake
//     element refuses any innerHTML write;
//   - never counted (T-05-117): the layer fetches only <base>/official/pins,
//     once per load, and never touches the list, the counts or the reports;
//   - refresh: a pinalert:feed-centre event refetches at most every 5 minutes,
//     and a timer refetches every 5 minutes;
//   - the chosen look: the pin of the open card carries is-chosen until the card
//     closes.
//
// Run it with: node --test web/official_pins_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what official-pins.js calls: a minimal element model, a
// fake Leaflet (divIcon, marker, layerGroup), a fake PinalertMap, and a fetch
// and timers the test drives by hand.

process.env.TZ = 'Asia/Kolkata';

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'official-pins.js'), 'utf8');

class FakeEl {
  constructor(doc, tag) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.attrs = {};
    this.handlers = {};
    this.className = '';
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  set innerHTML(v) { throw new Error('innerHTML must never be used: ' + v); }
  set outerHTML(v) { throw new Error('outerHTML must never be used: ' + v); }
  insertAdjacentHTML(where, v) { throw new Error('insertAdjacentHTML must never be used: ' + where + v); }
  setAttribute(n, v) {
    this.attrs[n] = String(v);
    if (n === 'class') { this.className = String(v); }
  }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  removeAttribute(n) { delete this.attrs[n]; }
  appendChild(node) {
    if (node.parentNode) { node.parentNode.children.splice(node.parentNode.children.indexOf(node), 1); }
    this.children.push(node);
    node.parentNode = this;
    return node;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  focus() { this.doc.activeElement = this; }
}

const hasClass = (node, name) => (node.className || '').split(/\s+/).includes(name);
const walk = (node, fn) => { fn(node); node.children.forEach((c) => walk(c, fn)); };
const findAll = (root, pred) => { const found = []; walk(root, (n) => { if (pred(n)) { found.push(n); } }); return found; };
const byClass = (root, name) => findAll(root, (n) => hasClass(n, name));
const textOf = (root) => root.textContent;

const NOW = Date.parse('2026-10-29T10:34:00Z');

function loadLayer(options = {}) {
  const requests = [];
  const intervals = new Map();
  const listeners = {};
  const overlays = [];
  const cards = [];
  let now = NOW;
  let nextTimer = 1;
  const doc = {
    activeElement: null,
    createElement(tag) { return new FakeEl(doc, tag); },
    addEventListener(type, fn) { (listeners[type] = listeners[type] || []).push(fn); }
  };
  const advance = (ms) => {
    const end = now + ms;
    for (;;) {
      const due = [...intervals.entries()].filter(([, t]) => t.next <= end).sort((a, b) => a[1].next - b[1].next)[0];
      if (!due) { break; }
      const [id, timer] = due;
      now = timer.next;
      if (timer.once) { intervals.delete(id); } else { timer.next += timer.every; }
      timer.fn();
    }
    now = end;
  };
  const RealDate = Date;
  class FakeDate extends RealDate {
    static now() { return now; }
    constructor(...args) { if (args.length === 0) { super(now); } else { super(...args); } }
  }
  const groups = [];
  const L = {
    divIcon(opts) { return { options: opts }; },
    layerGroup() {
      const group = {
        markers: [],
        addLayer(m) { this.markers.push(m); m.inGroup = this; return this; },
        removeLayer(m) { this.markers = this.markers.filter((x) => x !== m); m.inGroup = null; return this; },
        clearLayers() { this.markers.forEach((m) => { m.inGroup = null; }); this.markers = []; return this; }
      };
      groups.push(group);
      return group;
    },
    marker(latlng, opts) {
      const m = {
        latlng,
        options: opts || {},
        icon: (opts || {}).icon,
        el: new FakeEl(doc, 'div'),
        handlers: {},
        zOffset: 0,
        on(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); return this; },
        // With lateIcons the element exists only after the map has a view, as in
        // Leaflet: the test calls markers' add handlers by hand (showIcons).
        getElement() { return this.inGroup && (!options.lateIcons || this.shown) ? this.el : null; },
        showIcons() { this.shown = true; (this.handlers.add || []).forEach((h) => h({ type: 'add' })); },
        setZIndexOffset(n) { this.zOffset = n; return this; },
        setIcon(icon) { this.icon = icon; return this; },
        addTo(group) { group.addLayer(this); return this; }
      };
      return m;
    }
  };
  const PinalertMap = options.noMap ? undefined : {
    addOverlay(layer) { overlays.push(layer); return options.overlayRefused ? false : true; },
    openCard(latlng, node, name, opts) { cards.push({ latlng, node, name, opts }); return true; }
  };
  const sandbox = {
    console,
    document: doc,
    Date: FakeDate,
    L: options.noLeaflet ? undefined : L,
    fetch(target, init) {
      return new Promise((resolve, reject) => {
        requests.push({ url: String(target), init: init || {}, resolve, reject });
      });
    },
    setInterval(fn, every) { const id = nextTimer++; intervals.set(id, { fn, every, next: now + every }); return id; },
    clearInterval(id) { intervals.delete(id); },
    // Amended 2026-10-29 by the Phase 5 review fix (WR-B01): a timeout waits for
    // the clock the test moves, so the request time limit fires only when
    // advance() reaches it. The script made no setTimeout call before the fix,
    // so no older case depended on the old fire-at-once fake.
    setTimeout(fn, ms) { const id = nextTimer++; intervals.set(id, { fn, every: 0, next: now + (ms || 0), once: true }); return id; },
    clearTimeout(id) { intervals.delete(id); },
    AbortController: class { constructor() { this.signal = { aborted: false }; } abort() { this.signal.aborted = true; } },
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } }
  };
  if (PinalertMap) { sandbox.PinalertMap = PinalertMap; }
  if (options.apiBase !== undefined) { sandbox.PinalertApiBase = options.apiBase; }
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'official-pins.js' });
  const emit = (type, detail) => (listeners[type] || []).forEach((fn) => fn({ type, detail }));
  return { doc, api: sandbox.PinalertOfficialPins, requests, overlays, cards, groups, advance, emit, intervals, sandbox };
}

const flush = async () => { for (let i = 0; i < 8; i++) { await new Promise((r) => setImmediate(r)); } };

const answer = (req, status, body) => req.resolve({
  ok: status >= 200 && status < 300,
  status,
  json: body === undefined ? () => Promise.reject(new Error('no body')) : () => Promise.resolve(body)
});

const fire = (target, type, extra) => {
  const event = {
    type, target, prevented: false, stopped: false,
    preventDefault() { this.prevented = true; },
    stopPropagation() { this.stopped = true; },
    ...extra
  };
  (target.handlers[type] || []).forEach((h) => h(event));
  return event;
};

const hoursAgo = (h) => new Date(NOW - h * 3600 * 1000).toISOString();
const hoursAhead = (h) => new Date(NOW + h * 3600 * 1000).toISOString();

function pin(over = {}) {
  return {
    id: 7,
    event_type: 'EQ',
    hazard_word: 'Earthquake',
    name: 'Earthquake in Vanuatu Islands',
    alert_level: 'red',
    level_word: 'Red',
    lat: 16.5,
    lon: 80.6,
    from_date: hoursAgo(3),
    to_date: hoursAhead(5),
    severity_text: 'Magnitude 6.5M, Depth 10 km',
    report_url: 'https://www.gdacs.org/report.aspx?eventid=1&episodeid=2&eventtype=EQ',
    ...over
  };
}

const fresh = (pins) => ({ state: 'fresh', age_seconds: 300, pins });

// open loads the layer and answers its first request with body.
async function open(body, options) {
  const layer = loadLayer(options);
  await flush();
  assert.equal(layer.requests.length, 1, 'the layer makes one request on load');
  answer(layer.requests[0], 200, body);
  await flush();
  return layer;
}

const markersOf = (layer) => layer.groups.flatMap((g) => g.markers);

test('the layer leaves quietly with no map, no Leaflet or a refused overlay', async () => {
  for (const options of [{ noMap: true }, { noLeaflet: true }]) {
    const layer = loadLayer(options);
    await flush();
    assert.equal(layer.requests.length, 0, 'nothing is fetched when nothing can be drawn');
  }
  const refused = await open(fresh([pin()]), { overlayRefused: true });
  assert.equal(markersOf(refused).length, 0);
});

test('a fresh answer draws one named marker per pin on its own layer group', async () => {
  const layer = await open(fresh([
    pin(),
    pin({ id: 8, event_type: 'TC', name: 'Tropical Cyclone ARNAB-26', alert_level: 'orange', level_word: 'Orange', lat: 14.0, lon: 86.0 }),
    pin({ id: 9, event_type: 'FL', name: 'Flood in India', alert_level: 'green', level_word: 'Green', lat: 25.0, lon: 82.0 })
  ]));
  assert.equal(layer.overlays.length, 1, 'one layer group is handed to the map');
  const markers = markersOf(layer);
  assert.equal(markers.length, 3);
  assert.deepEqual(markers.map((m) => m.el.getAttribute('aria-label')), [
    'GDACS, Earthquake in Vanuatu Islands, Red',
    'GDACS, Tropical Cyclone ARNAB-26, Orange',
    'GDACS, Flood in India, Green'
  ]);
  assert.deepEqual(markers.map((m) => [m.latlng[0], m.latlng[1]]), [[16.5, 80.6], [14, 86], [25, 82]]);
});

test('the pin is the flat rectangle with the word GDACS inside a 44px marker box', async () => {
  const layer = await open(fresh([pin()]));
  const icon = markersOf(layer)[0].icon;
  assert.equal(icon.options.className, 'official-pin-box');
  assert.deepEqual([...icon.options.iconSize], [96, 44]);
  assert.deepEqual([...icon.options.iconAnchor], [48, 22]);
  const node = icon.options.html;
  assert.ok(hasClass(node, 'official-pin'), 'the inner shape is .official-pin');
  assert.ok(!/icon-badge/.test(node.className), 'never the round crowd badge');
  assert.match(textOf(node), /GDACS/);
  assert.equal(byClass(node, 'official-pin__glyph').length, 1);
  assert.ok(byClass(node, 'official-pin__glyph')[0].className.includes('official-glyph--earthquake'));
});

test('a marker whose element appears later still gets its name and its keys', async () => {
  const layer = await open(fresh([pin()]), { lateIcons: true });
  const marker = markersOf(layer)[0];
  assert.equal(marker.getElement(), null, 'no element yet, the map has no view');
  marker.showIcons();
  assert.equal(marker.el.getAttribute('aria-label'), 'GDACS, Earthquake in Vanuatu Islands, Red');
  fire(marker.el, 'keydown', { key: 'Enter' });
  assert.equal(layer.cards.length, 1, 'Enter opens the card once the element is there');
  marker.showIcons();
  fire(marker.el, 'keydown', { key: ' ' });
  assert.equal(layer.cards.length, 2, 'the same element is not given a second key handler');
});

test('each hazard kind has its own glyph and no pin carries a severity class', async () => {
  const kinds = { EQ: 'earthquake', TC: 'cyclone', FL: 'flood', WF: 'wildfire', VO: 'volcano' };
  let id = 20;
  const pins = Object.keys(kinds).map((type) => pin({ id: id++, event_type: type, name: 'Event ' + type }));
  const layer = await open(fresh(pins));
  const markers = markersOf(layer);
  assert.equal(markers.length, 5);
  markers.forEach((m, i) => {
    const type = Object.keys(kinds)[i];
    const node = m.icon.options.html;
    assert.ok(byClass(node, 'official-pin__glyph')[0].className.includes('official-glyph--' + kinds[type]), type);
    assert.ok(!/sev-|severity|critical|high|medium|low/.test(node.className + byClass(node, 'official-pin__glyph')[0].className));
  });
});

test('a pin of an unknown kind, level or place is skipped, the rest are drawn', async () => {
  const layer = await open(fresh([
    pin({ id: 1, event_type: 'DR' }),
    pin({ id: 2, alert_level: 'purple' }),
    pin({ id: 3, lat: 'north' }),
    pin({ id: 4, lat: 95 }),
    pin({ id: 5, name: '' }),
    pin({ id: 6 })
  ]));
  assert.equal(markersOf(layer).length, 1);
});

test('stale and unavailable answers draw nothing and clear what was drawn before', async () => {
  const layer = await open(fresh([pin(), pin({ id: 8 })]));
  assert.equal(markersOf(layer).length, 2);
  layer.api.drawPins({ state: 'stale', age_seconds: 4000, pins: [pin()] });
  assert.equal(markersOf(layer).length, 0, 'a stale answer clears the layer even if pins came with it');
  layer.api.drawPins(fresh([pin()]));
  assert.equal(markersOf(layer).length, 1);
  layer.api.drawPins({ state: 'unavailable', age_seconds: null, pins: [] });
  assert.equal(markersOf(layer).length, 0);
});

test('an answer that is not shaped like the route draws nothing', async () => {
  const layer = await open(fresh([pin()]));
  for (const bad of [null, 'text', 42, [], {}, { state: 'fresh' }, { state: 'fresh', pins: 'x' }]) {
    layer.api.drawPins(fresh([pin()]));
    layer.api.drawPins(bad);
    assert.equal(markersOf(layer).length, 0, JSON.stringify(bad));
  }
});

test('a failed request or an HTTP error clears the layer', async () => {
  const layer = await open(fresh([pin()]));
  assert.equal(markersOf(layer).length, 1);
  layer.advance(300000);
  layer.requests[1].reject(new Error('offline'));
  await flush();
  assert.equal(markersOf(layer).length, 0, 'a network failure');
  layer.api.drawPins(fresh([pin()]));
  layer.advance(300000);
  answer(layer.requests[2], 503, { error: 'x' });
  await flush();
  assert.equal(markersOf(layer).length, 0, 'an HTTP error');
});

test('Enter, Space and a tap open the card through PinalertMap.openCard', async () => {
  const layer = await open(fresh([pin()]));
  const marker = markersOf(layer)[0];
  const enter = fire(marker.el, 'keydown', { key: 'Enter' });
  assert.equal(layer.cards.length, 1);
  assert.ok(enter.prevented, 'Enter is consumed');
  fire(marker.el, 'keydown', { key: ' ' });
  assert.equal(layer.cards.length, 2);
  fire(marker.el, 'keydown', { key: 'Tab' });
  fire(marker.el, 'keydown', { key: 'a' });
  assert.equal(layer.cards.length, 2, 'no other key opens it');
  (marker.handlers.click || []).forEach((h) => h({ type: 'click' }));
  assert.equal(layer.cards.length, 3, 'a tap');
  const opened = layer.cards[0];
  assert.deepEqual([...opened.latlng], [16.5, 80.6]);
  assert.equal(opened.name, 'GDACS, Earthquake in Vanuatu Islands, Red');
  assert.ok(hasClass(opened.node, 'official-card'));
});

test('the card holds the badge, the chip, the title, the lines, the honesty line and the credit', async () => {
  const layer = await open(fresh([pin()]));
  fire(markersOf(layer)[0].el, 'keydown', { key: 'Enter' });
  const card = layer.cards[0].node;
  const text = textOf(card);
  assert.match(text, /GDACS/);
  assert.equal(byClass(card, 'official-card__badge').length, 1);
  const chip = byClass(card, 'official-card__chip')[0];
  assert.equal(chip.textContent, 'Red');
  assert.ok(hasClass(chip, 'official-card__chip--extreme'));
  assert.equal(byClass(card, 'official-card__title')[0].textContent, 'Earthquake in Vanuatu Islands');
  assert.match(text, /Hazard\s*Earthquake/);
  assert.match(text, /Issued 3 hours ago/);
  assert.match(text, /Until .*\d:\d\d [ap]m/);
  assert.match(text, /Issued by GDACS/);
  assert.match(text, /Magnitude 6\.5M, Depth 10 km/);
  assert.match(text, /Automatic estimate\. Follow your state disaster management authority\./);
  assert.match(text, /Global Disaster Alert and Coordination System, GDACS/);
  assert.match(text, /Open GDACS page/);
});

test('the level chip word and class follow the level: Red extreme, Orange severe, Green neutral', async () => {
  const layer = await open(fresh([
    pin({ id: 1 }),
    pin({ id: 2, alert_level: 'orange', level_word: 'Orange' }),
    pin({ id: 3, alert_level: 'green', level_word: 'Green' })
  ]));
  const expected = [['Red', 'extreme'], ['Orange', 'severe'], ['Green', 'neutral']];
  markersOf(layer).forEach((m, i) => {
    fire(m.el, 'keydown', { key: 'Enter' });
    const chip = byClass(layer.cards[i].node, 'official-card__chip')[0];
    assert.equal(chip.textContent, expected[i][0]);
    assert.ok(hasClass(chip, 'official-card__chip--' + expected[i][1]));
  });
});

test('Issued and Until are left out when GDACS sent no date, and the severity line when it sent no text', async () => {
  const layer = await open(fresh([pin({ from_date: null, to_date: null, severity_text: '' })]));
  fire(markersOf(layer)[0].el, 'keydown', { key: 'Enter' });
  const text = textOf(layer.cards[0].node);
  assert.doesNotMatch(text, /Issued \d|Issued just|Until/);
  assert.match(text, /Issued by GDACS/);
  assert.doesNotMatch(text, /Magnitude/);
});

test('the card has no vote, trust, Flag, Mute, count or rating part', async () => {
  const layer = await open(fresh([pin()]));
  fire(markersOf(layer)[0].el, 'keydown', { key: 'Enter' });
  const card = layer.cards[0].node;
  const classes = findAll(card, () => true).map((n) => n.className).join(' ');
  assert.doesNotMatch(classes, /vote|trust|flag|mute|reliab|confirm|dispute|severity|rating/i);
  assert.equal(findAll(card, (n) => n.tagName === 'BUTTON').length, 0, 'the one action is a link');
  assert.doesNotMatch(textOf(card), /Confirm|Dispute|Flag|Mute|Mark resolved|reports?\b|people/i);
});

test('the link is kept for a gdacs.org page and dropped for anything else', async () => {
  const good = 'https://www.gdacs.org/report.aspx?eventid=1';
  const layer = await open(fresh([pin({ report_url: good })]));
  fire(markersOf(layer)[0].el, 'keydown', { key: 'Enter' });
  const links = findAll(layer.cards[0].node, (n) => n.tagName === 'A');
  assert.equal(links.length, 1);
  assert.equal(links[0].getAttribute('href'), good);
  assert.equal(links[0].getAttribute('target'), '_blank');
  assert.equal(links[0].getAttribute('rel'), 'noopener noreferrer');
  assert.equal(links[0].textContent, 'Open GDACS page');
  assert.ok(hasClass(links[0], 'official-card__link'));

  assert.equal(layer.api.safeGdacsLink(good), good);
  for (const bad of [
    'http://evil.example/', 'javascript:alert(1)', 'http://www.gdacs.org/report.aspx', 'https://gdacs.org/x',
    'https://www.gdacs.org.evil.example/x', 'https://evil.example/https://www.gdacs.org/', '//www.gdacs.org/x',
    '', null, undefined, 42, 'HTTPS://WWW.GDACS.ORG/x', ' https://www.gdacs.org/x',
    'https://www.gdacs.org/x y', 'https://www.gdacs.org/x"onclick=1', 'https://www.gdacs.org/x<script>',
    'https://www.gdacs.org/x\\y', 'https://www.gdacs.org/\u0000'
  ]) {
    assert.equal(layer.api.safeGdacsLink(bad), '', String(bad));
  }
  for (const bad of ['http://evil.example/', 'javascript:alert(1)', '']) {
    const other = await open(fresh([pin({ report_url: bad })]));
    fire(markersOf(other)[0].el, 'keydown', { key: 'Enter' });
    assert.equal(findAll(other.cards[0].node, (n) => n.tagName === 'A').length, 0, 'no link for ' + bad);
    assert.doesNotMatch(textOf(other.cards[0].node), /Open GDACS page/);
  }
});

test('feed text is text: markup in a name or a severity line never becomes an element', async () => {
  const hostile = '<img src=x onerror=1><b>Cyclone</b>';
  const layer = await open(fresh([pin({ name: hostile, severity_text: '<script>alert(1)</script>' })]));
  const marker = markersOf(layer)[0];
  assert.equal(marker.el.getAttribute('aria-label'), 'GDACS, ' + hostile + ', Red');
  fire(marker.el, 'keydown', { key: ' ' });
  const card = layer.cards[0].node;
  assert.equal(byClass(card, 'official-card__title')[0].textContent, hostile);
  assert.equal(findAll(card, (n) => ['IMG', 'B', 'SCRIPT'].includes(n.tagName)).length, 0);
  assert.match(textOf(card), /<script>alert\(1\)<\/script>/);
});

test('never counted: one fetch of <base>/official/pins with the credentials of the page, nothing else', async () => {
  const layer = await open(fresh([pin()]));
  assert.equal(layer.requests.length, 1);
  assert.equal(layer.requests[0].url, '/api/official/pins');
  assert.equal(layer.requests[0].init.credentials, 'same-origin');
  assert.ok(!layer.requests.some((r) => /reports|votes|feed/.test(r.url)));
  const based = loadLayer({ apiBase: '/sandbox/api' });
  await flush();
  assert.equal(based.requests[0].url, '/sandbox/api/official/pins');
});

test('a feed-centre event refetches at most every 5 minutes and a timer refetches every 5 minutes', async () => {
  const layer = await open(fresh([pin()]));
  layer.emit('pinalert:feed-centre', { lat: 16.5, lon: 80.6 });
  layer.advance(60000);
  layer.emit('pinalert:feed-centre', { lat: 17, lon: 81 });
  await flush();
  assert.equal(layer.requests.length, 1, 'inside 5 minutes nothing is fetched again');
  layer.advance(240000);
  await flush();
  assert.equal(layer.requests.length, 2, 'the timer refetches at 5 minutes');
  answer(layer.requests[1], 200, fresh([pin()]));
  await flush();
  layer.advance(10000);
  layer.emit('pinalert:feed-centre', { lat: 17, lon: 81 });
  await flush();
  assert.equal(layer.requests.length, 2, 'an event after a fresh fetch waits again');
  layer.advance(300000);
  layer.emit('pinalert:feed-centre', { lat: 17, lon: 81 });
  await flush();
  assert.ok(layer.requests.length >= 3);
});

test('a second request is not sent while one is out', async () => {
  const layer = loadLayer();
  await flush();
  layer.emit('pinalert:feed-centre', { lat: 1, lon: 2 });
  await flush();
  assert.equal(layer.requests.length, 1, 'one request out at a time');
  answer(layer.requests[0], 200, fresh([pin()]));
  await flush();
  assert.equal(markersOf(layer).length, 1);
});

test('a redraw keeps an unchanged marker and drops one that left', async () => {
  const layer = await open(fresh([pin({ id: 1 }), pin({ id: 2, name: 'Other' })]));
  const first = markersOf(layer)[0];
  layer.api.drawPins(fresh([pin({ id: 1 })]));
  const after = markersOf(layer);
  assert.equal(after.length, 1);
  assert.equal(after[0], first, 'the same marker object, so keyboard focus is not lost');
  layer.api.drawPins(fresh([pin({ id: 1, name: 'Renamed' })]));
  assert.equal(markersOf(layer)[0].el.getAttribute('aria-label'), 'GDACS, Renamed, Red');
});

test('the pin of the open card is chosen until the card closes', async () => {
  const layer = await open(fresh([pin({ id: 1 }), pin({ id: 2, name: 'Other' })]));
  const [first, second] = markersOf(layer);
  fire(first.el, 'keydown', { key: 'Enter' });
  const inner = first.icon.options.html;
  assert.ok(hasClass(inner, 'is-chosen'), 'chosen while its card is open');
  assert.ok(first.zOffset > 0, 'raised above the other pins');
  assert.ok(!hasClass(second.icon.options.html, 'is-chosen'));
  const opts = layer.cards[0].opts;
  assert.equal(opts.opener, first.el, 'focus goes back to the pin on close');
  opts.onClose();
  assert.ok(!hasClass(inner, 'is-chosen'));
  assert.equal(first.zOffset, 0);
});

test('the layer never writes to the list, the counts or the shared store', async () => {
  const layer = loadLayer();
  await flush();
  assert.equal(layer.sandbox.Pinalert, undefined, 'it does not depend on the shared store');
  answer(layer.requests[0], 200, fresh([pin()]));
  await flush();
  assert.equal(layer.doc.activeElement, null, 'nothing took focus');
});

// ---- A request that never answers (Phase 5 review fix WR-B01) ----
// Added 2026-10-29. Every pins request has a 10 second limit: at the limit it
// is aborted, the layer is cleared (nothing is known to be current) and the
// next load sends a new request.

test('WR-B01: a pins request with no answer is given up at 10 seconds and the layer clears', async () => {
  const layer = await open(fresh([pin()]));
  assert.equal(markersOf(layer).length, 1);
  assert.ok(layer.requests[0].init.signal, 'the request carries an abort signal');
  layer.advance(300000);
  await flush();
  assert.equal(layer.requests.length, 2, 'the five minute refresh is out');
  layer.advance(9000);
  await flush();
  assert.equal(markersOf(layer).length, 1, 'inside the limit the pins stay');
  layer.advance(1000);
  await flush();
  assert.equal(markersOf(layer).length, 0, 'at the limit the pins drawn before are cleared');
  assert.equal(layer.requests[1].init.signal.aborted, true, 'the hung request is aborted');
  layer.api.loadPins();
  await flush();
  assert.equal(layer.requests.length, 3, 'a hung request never blocks the next load');
  answer(layer.requests[1], 200, fresh([pin({ id: 8 })]));
  await flush();
  assert.equal(markersOf(layer).length, 0, 'the late answer of the given up request is dropped');
  answer(layer.requests[2], 200, fresh([pin()]));
  await flush();
  assert.equal(markersOf(layer).length, 1);
});

test('WR-B01: a body that never arrives is given up at the limit too', async () => {
  const layer = loadLayer();
  await flush();
  layer.requests[0].resolve({ ok: true, status: 200, json: () => new Promise(() => {}) });
  await flush();
  layer.advance(10000);
  await flush();
  layer.api.loadPins();
  await flush();
  assert.equal(layer.requests.length, 2);
});
