// official_card_close_test.mjs - where focus goes when a refresh closes the GDACS
// card (Phase 5 review fix NP-2, round 2 of the review fixes).
//
// A refresh that removes the pin of the open card takes the pin off the map and
// then closes the card (official-pins.js). map.js's closeOfficialPopup must then
// put focus on the map container, never leave it on the page body. On a phone
// the card sits in the detail pane outside the map container, so the map's own
// focusin listener never sees focus inside it: the close path must also look at
// where focus is when it runs.
//
// Like official_card_heading_test.mjs, this cuts closeOfficialPopup out of the
// real map.js text and runs it with fakes. A rename in map.js fails here loudly.
//
// Run it with: node --test web/official_card_close_test.mjs

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

const closeSource = between(mapSource, '  function closeOfficialPopup(popup) {', '  // ---- End of the overlays and the GDACS card');

class FakeEl {
  constructor(doc, name) {
    this.doc = doc;
    this.name = name;
    this.kids = [];
    this.attrs = {};
    this.isConnected = true;
  }
  add(child) { this.kids.push(child); return child; }
  contains(node) { return node === this || this.kids.some((k) => k.contains(node)); }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  removeAttribute(n) { delete this.attrs[n]; }
  focus() { this.doc.activeElement = this; }
}

// run closes a card whose focus is on its heading. seenByMap is what the map
// container's focusin listener recorded (false on a phone, where the card is
// outside the container); openerOnMap says whether the pin is still there.
function run({ seenByMap, openerOnMap }) {
  const doc = { activeElement: null };
  doc.body = new FakeEl(doc, 'body');
  doc.documentElement = new FakeEl(doc, 'html');
  const mapEl = new FakeEl(doc, 'map');
  const cardEl = new FakeEl(doc, 'card');
  const heading = cardEl.add(new FakeEl(doc, 'heading'));
  const opener = new FakeEl(doc, 'pin');
  opener.isConnected = openerOnMap;
  heading.focus();
  const popup = { getElement: () => cardEl };
  const closed = [];
  const sandbox = {
    document: doc,
    map: { getContainer: () => mapEl },
    officialPopup: popup,
    officialMeta: { opener, onClose() { closed.push('layer told'); } },
    closePressHadFocus: false,
    popupFocusInside: seenByMap,
    popupOpener: { id: null, kind: 'official' },
    openedWhileHidden: false,
    activePopup: popup,
    noteReportClosed() {},
    popup
  };
  vm.createContext(sandbox);
  vm.runInContext('var officialPopup = this.officialPopup, officialMeta = this.officialMeta, ' +
    'closePressHadFocus = false, popupFocusInside = this.popupFocusInside, popupOpener = this.popupOpener, ' +
    'openedWhileHidden = false, activePopup = this.activePopup;\n' + closeSource + '\ncloseOfficialPopup(popup);', sandbox);
  return { doc, mapEl, opener, closed };
}

test('NP-2: a refresh that removed the pin of a phone card puts focus on the map, not the body', () => {
  const { doc, mapEl, closed } = run({ seenByMap: false, openerOnMap: false });
  assert.ok(doc.activeElement === mapEl, 'focus is on the map container, it was on ' + (doc.activeElement && doc.activeElement.name));
  assert.deepEqual(closed, ['layer told']);
});

test('NP-2: the same on a laptop, where the map saw focus inside the card', () => {
  const { doc, mapEl } = run({ seenByMap: true, openerOnMap: false });
  assert.ok(doc.activeElement === mapEl);
});

test('NP-2: a card closed while its pin is still on the map hands focus back to the pin', () => {
  const { doc, opener } = run({ seenByMap: false, openerOnMap: true });
  assert.ok(doc.activeElement === opener, 'focus goes back to the pin that opened the card');
});
