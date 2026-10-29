// official_card_heading_test.mjs - the heading a phone reader meets first when a
// GDACS card opens (Phase 5 review fix WR-B03, IN-T01, ROBUST-06, UI-SPEC 3b).
//
// On a phone, focus lands on the popup's heading. map.js puts in a heading for
// every popup; for a GDACS card it must never say "Report", or VoiceOver would
// announce a crowd report before the hazard. A crowd report keeps "Report".
//
// map.js builds a whole Leaflet map on load and keeps its steps private, so,
// like report_view_test.mjs, this test cuts onPopupOpen (up to the end of the
// GDACS branch) and ensureReportHeading out of the real file text and runs them
// with a fake popup container. A rename in map.js fails here loudly.
//
// Run it with: node --test web/official_card_heading_test.mjs

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

const titles = between(mapSource, '  var BACK_LABEL = ', '  // \'pushed\' when this script');
const ensureHeading = between(mapSource, '  function ensureReportHeading(', '  // focusReportHeading');
// onPopupOpen up to the end of its GDACS branch; the crowd path after it needs
// the whole map, and the case below stops there.
const onOpen = between(mapSource, '  function onPopupOpen(evt) {', '    var popup = evt.popup;') + '    opened.push(\'report\');\n  }\n';

class FakeEl {
  constructor(tag, cls) {
    this.tagName = tag.toUpperCase();
    this.className = cls || '';
    this.children = [];
    this.attrs = {};
    this.textContent = '';
  }
  get firstChild() { return this.children[0] || null; }
  get nextSibling() {
    const kids = this.parent ? this.parent.children : [];
    return kids[kids.indexOf(this) + 1] || null;
  }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  insertBefore(node, ref) {
    const at = this.children.indexOf(node);
    if (at !== -1) { this.children.splice(at, 1); }
    const where = ref ? this.children.indexOf(ref) : -1;
    if (where === -1) { this.children.push(node); } else { this.children.splice(where, 0, node); }
    node.parent = this;
    return node;
  }
  querySelector(sel) {
    const cls = sel.replace(/^\./, '');
    const walk = (el) => {
      for (const c of el.children) {
        if (c.className.split(/\s+/).includes(cls)) { return c; }
        const deeper = walk(c);
        if (deeper) { return deeper; }
      }
      return null;
    };
    return walk(this);
  }
}

function run(official) {
  const opened = [];
  const container = new FakeEl('div', 'leaflet-popup');
  const close = new FakeEl('a', 'leaflet-popup-close-button');
  container.insertBefore(new FakeEl('div', 'leaflet-popup-content-wrapper'), null);
  container.insertBefore(close, null);
  const sandbox = {
    opened,
    container,
    document: { createElement: (tag) => new FakeEl(tag) },
    Pinalert: { setText(el, text) { el.textContent = text; } },
    phoneDetail: () => true,
    isOfficialPopup: (popup) => popup.official === true,
    openOfficialPopup: () => { opened.push('official'); }
  };
  vm.createContext(sandbox);
  vm.runInContext(titles + ensureHeading + onOpen + '\nonPopupOpen({ popup: { official: ' + official +
    ', getElement: function () { return container; } } });', sandbox);
  return { container, close, opened };
}

test('WR-B03: the GDACS card heading on a phone is not Report', () => {
  const { container, close, opened } = run(true);
  assert.deepEqual(opened, ['official']);
  const heading = container.querySelector('.report-view__title');
  assert.ok(heading, 'the card still has the heading focus lands on');
  assert.notEqual(heading.textContent, 'Report');
  assert.equal(heading.textContent, 'Hazard');
  assert.equal(close.getAttribute('aria-label'), 'Back');
});

test('WR-B03: a crowd report keeps its Report heading', () => {
  const { container, opened } = run(false);
  assert.deepEqual(opened, ['report']);
  assert.equal(container.querySelector('.report-view__title').textContent, 'Report');
});
