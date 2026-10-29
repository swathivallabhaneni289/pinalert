// official_card_panel_test.mjs - a GDACS card opened while the laptop report
// panel is open (Phase 8 review fix WR-B02, 2026-10-29).
//
// The card is the Phase 5 hazard card: on a laptop it is a bubble in Leaflet's
// popup pane, on a phone it is the detail pane sheet. Opening it closes the
// report that was in the panel, and the panel's exit runs a turn later. The card
// must take its place from the phone test alone, never from the panel that is
// about to close, and the panel's exit must leave focus on the card.
//
// Like official_card_close_test.mjs, this cuts the functions out of the real
// map.js text and runs them with fakes. A rename in map.js fails here loudly.
//
// Run it with: node --test web/official_card_panel_test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const mapSource = fs.readFileSync(path.join(here, 'static', 'js', 'map.js'), 'utf8');

function between(source, from, to) {
  const start = source.indexOf(from);
  const end = source.indexOf(to, start);
  assert.ok(start >= 0 && end > start, 'the source no longer has the text between "' + from + '" and "' + to + '"');
  return source.slice(start, end);
}

const cut = [
  between(mapSource, '  function applyPopupPlacement(popup, card) {', '  // arrangeCard puts'),
  between(mapSource, '  function detailInPane() {', '  // openReportWords'),
  between(mapSource, '  function exitPanel() {', '  // holdPanelPosition'),
  between(mapSource, '  function isOfficialPopup(popup) {', '  // openCard opens'),
  between(mapSource, '  function openCard(latlng, node, name, opts) {', '  // closeCard closes')
].join('\n');

class FakeEl {
  constructor(doc, name) {
    this.doc = doc;
    this.name = name;
    this.kids = [];
    this.attrs = {};
    this.style = {};
  }
  add(child) { this.kids.push(child); return child; }
  contains(node) { return node === this || this.kids.some((k) => k.contains(node)); }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  removeAttribute(n) { delete this.attrs[n]; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  focus() { this.doc.activeElement = this; }
}

// run opens a GDACS card while a report is open in the laptop panel (or, with
// phone true, on a phone, where the panel is never open). cardInHost puts the
// card's element in the page level report host whatever its pane says, so the
// exit's own guard is checked on its own.
function run({ phone = false, cardInHost = null } = {}) {
  const doc = { activeElement: null };
  doc.body = new FakeEl(doc, 'body');
  doc.documentElement = new FakeEl(doc, 'html');
  const host = new FakeEl(doc, 'report-view-host');
  const mapPane = new FakeEl(doc, 'popup-pane');
  doc.querySelector = (sel) => (sel === '.report-view-host' ? host : null);
  if (!phone) { doc.documentElement.setAttribute('data-report-panel', 'true'); }
  const reportOpener = new FakeEl(doc, 'old-report-pin');
  const reportEl = host.add(new FakeEl(doc, 'report-popup'));
  const cardEl = new FakeEl(doc, 'gdacs-card');
  const timers = [];
  const returned = [];
  const reportPopup = { id: 'report' };
  const sandbox = {
    document: doc,
    window: { setTimeout(fn) { timers.push(fn); return timers.length; }, clearTimeout() {} },
    L: {
      popup(options) {
        return { options: { ...options }, setLatLng() { return this; }, setContent() { return this; } };
      }
    },
    reducedMotionValue: false,
    phoneValue: phone,
    returned,
    host, mapPane, cardEl, reportEl, reportOpener, reportPopup, cardInHost,
    map: null
  };
  sandbox.map = {
    closePopup() {},
    // openPopup closes the open report first (its close path, panelNoteClose),
    // then opens the card (openOfficialPopup sets activePopup and focuses it).
    openPopup(popup) {
      sandbox.ctx.panelNoteClose(reportOpener, reportEl);
      sandbox.ctx.setActive(popup);
      const inHost = cardInHost === null ? popup.options.pane === 'detailPane' : cardInHost;
      (inHost ? host : mapPane).add(cardEl);
      cardEl.focus();
    }
  };
  vm.createContext(sandbox);
  vm.runInContext(
    'var reducedMotion = this.reducedMotionValue, panPad = { topLeft: [0, 0], bottomRight: [0, 0] }, ' +
    'officialPopup = null, officialMeta = null, panelOpen = !this.phoneValue, hostSwitching = false, ' +
    'panelExitTimer = 0, panelOpener = null, activePopup = this.reportPopup, PANEL_ATTRIBUTE = "data-report-panel";\n' +
    'function phoneDetail() { return phoneValue; }\n' +
    'function arrangeCard() {}\n' +
    'function replaceAllPlacements() {}\n' +
    'function returnFocusToOpener(opener) { returned.push(opener); opener.focus(); }\n' +
    cut + '\n' +
    'this.ctx = { panelNoteClose: panelNoteClose, setActive: function (p) { activePopup = p; }, ' +
    'openCard: openCard, panelOpen: function () { return panelOpen; } };',
    sandbox
  );
  const ok = sandbox.ctx.openCard({ lat: 1, lng: 2 }, cardEl, 'Cyclone', { opener: new FakeEl(doc, 'gdacs-pin') });
  assert.equal(ok, true);
  return { doc, sandbox, timers, returned, cardEl, reportOpener };
}

function placed(sandbox) {
  return vm.runInContext('officialPopup', sandbox);
}

test('WR-B02: a GDACS card opened from the laptop panel opens as the laptop bubble', () => {
  const { sandbox } = run();
  const popup = placed(sandbox);
  assert.equal(popup.options.pane, 'popupPane', 'the card is placed by the phone test alone');
  assert.equal(popup.options.autoPan, true, 'the laptop bubble pans as the Phase 5 card does');
});

test('WR-B02: the panel exit leaves focus on the card and closes the panel', () => {
  const { doc, sandbox, timers, returned, cardEl } = run();
  assert.equal(timers.length, 1, 'the report close started the panel exit');
  timers.forEach((fn) => fn());
  assert.equal(sandbox.ctx.panelOpen(), false);
  assert.equal(doc.documentElement.hasAttribute('data-report-panel'), false, 'the list comes back');
  assert.deepEqual(returned, [], 'focus is not sent back to the old report pin');
  assert.ok(doc.activeElement === cardEl, 'focus stays on the card');
});

test('WR-B02: the exit leaves a focused GDACS card alone even inside the report host', () => {
  const { doc, timers, returned, cardEl } = run({ cardInHost: true });
  timers.forEach((fn) => fn());
  assert.deepEqual(returned, []);
  assert.ok(doc.activeElement === cardEl);
});

test('WR-B02: on a phone the card still opens in the detail pane sheet', () => {
  const { sandbox } = run({ phone: true });
  const popup = placed(sandbox);
  assert.equal(popup.options.pane, 'detailPane');
  assert.equal(popup.options.autoPan, false);
});
