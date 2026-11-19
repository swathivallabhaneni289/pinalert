// popup_footer_test.mjs - checks the Open report row of the laptop bubble and
// the laptop report panel's enter, switch and exit rules (Phase 8 plan 07,
// D-18 default A, SC1, SC4).
//
// map.js builds a whole Leaflet map on load and keeps its steps private, so this
// test cuts the laptop panel section out of the real file text (it starts and
// ends with the plan 07 section comments) and runs it with a fake map, fake
// markers, a fake root element and a fake clock. A rename in map.js fails here
// loudly instead of passing on stale code.
//
// Run it with: node --test web/popup_footer_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

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

const section = between(
  mapSource,
  '  // ---- The laptop report panel (Phase 8 plan 07)',
  '  // ---- End of the laptop report panel (Phase 8 plan 07)'
);

// A fake DOM node: enough of Element for the section's row builder.
class FakeNode {
  constructor(tag) {
    this.tagName = tag;
    this.className = '';
    this.children = [];
    this.attrs = {};
    this.hidden = false;
    this.textContent = '';
    this.handlers = {};
    this.style = { left: '', transform: '' };
  }
  setAttribute(name, value) { this.attrs[name] = String(value); }
  getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; }
  removeAttribute(name) { delete this.attrs[name]; }
  appendChild(node) { this.children.push(node); return node; }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  click() { (this.handlers.click || []).forEach((fn) => fn({ type: 'click' })); }
  contains(node) { return node === this || this.children.some((c) => c.contains && c.contains(node)); }
}

// load runs the section. wide is true on a laptop (phoneDetail false).
function load({ wide = true } = {}) {
  const log = [];
  const timers = { next: 1, list: [] };
  const root = new FakeNode('html');
  const body = new FakeNode('body');
  const host = new FakeNode('div');
  const doc = {
    documentElement: root,
    body,
    activeElement: body,
    createElement(tag) { return new FakeNode(tag); },
    querySelector(selector) { return selector === '.report-view-host' ? host : null; }
  };
  const state = { wide, hostSwitchSeen: [], popupOpen: null };
  function marker(id) {
    const m = {
      id,
      closePopup() { log.push('close ' + id); state.hostSwitchSeen.push('close:' + ctx.hostSwitching); state.popupOpen = null; },
      openPopup() { log.push('open ' + id); state.hostSwitchSeen.push('open:' + ctx.hostSwitching); state.popupOpen = id; ctx.popupOpener = { id, kind: 'marker' }; }
    };
    return m;
  }
  const ctx = {
    document: doc,
    window: {
      setTimeout(fn) { const id = timers.next++; timers.list.push({ id, fn }); return id; },
      clearTimeout(id) { timers.list = timers.list.filter((t) => t.id !== id); }
    },
    Pinalert: { setText(el, value) { el.textContent = String(value); } },
    map: {},
    markers: { 7: marker('7'), 8: marker('8') },
    cards: { 7: {}, 8: {} },
    popupOpener: null,
    phoneDetail() { return !state.wide; },
    applyPanPadding() {},
    applyPopupPlacement(popup, card) { log.push('place ' + (ctx.panelOpen ? 'panel' : 'bubble')); },
    returnFocusToOpener(opener) { log.push('focus ' + opener.kind + ' ' + opener.id); },
    // exitPanel asks whether a GDACS card is the open popup (Phase 8 review fix
    // WR-B02, 2026-10-29); no card is open in these tests.
    activePopup: null,
    isOfficialPopup() { return false; },
    isFinite
  };
  ctx.markers[7].getPopup = () => ({});
  ctx.markers[8].getPopup = () => ({});
  vm.createContext(ctx);
  vm.runInContext(section, ctx);
  // Variables declared with var in the section are properties of the context.
  const flush = () => {
    const due = timers.list.slice();
    timers.list = [];
    due.forEach((t) => t.fn());
  };
  return { ctx, log, root, doc, host, body, state, flush, timers };
}

test('count words', () => {
  const { ctx } = load();
  assert.equal(ctx.openReportWords(undefined), '');
  assert.equal(ctx.openReportWords(null), '');
  assert.equal(ctx.openReportWords('3'), '');
  assert.equal(ctx.openReportWords(-1), '');
  assert.equal(ctx.openReportWords(2.5), '');
  assert.equal(ctx.openReportWords(0), '', 'amended 2026-11-19: no comments gives no words');
  assert.equal(ctx.openReportWords(1), '1 comment');
  assert.equal(ctx.openReportWords(3), '3 comments');
  assert.equal(ctx.openReportWords(1204), '1204 comments');
});

test('row name', () => {
  const { ctx } = load();
  assert.equal(ctx.openReportName(undefined), 'Open full report');
  assert.equal(ctx.openReportName(3), 'Open full report, 3 comments');
  assert.equal(ctx.openReportName(0), 'Open full report');
  assert.equal(ctx.openReportName(1), 'Open full report, 1 comment');
});

test('row count is written from comment_count only and never replaces the node', () => {
  const { ctx } = load();
  const built = ctx.buildOpenReportRow('7');
  const card = { openRow: built.el, openCount: built.count };
  assert.equal(built.el.getAttribute('aria-label'), 'Open full report');
  assert.equal(built.count.hidden, true, 'no count shows before the server sends one');
  ctx.writeOpenReportRow(card, { id: 7 });
  assert.equal(built.count.textContent, '');
  assert.equal(built.count.hidden, true);
  ctx.writeOpenReportRow(card, { id: 7, comment_count: 12 });
  assert.equal(built.count.textContent, '12 comments');
  assert.equal(built.count.hidden, false);
  assert.equal(built.el.getAttribute('aria-label'), 'Open full report, 12 comments');
  ctx.writeOpenReportRow(card, { id: 7, comment_count: 0 });
  assert.equal(built.count.textContent, '', 'amended 2026-11-19: no comments gives no words');
  assert.equal(built.count.hidden, true);
  ctx.writeOpenReportRow(card, { id: 7 });
  assert.equal(built.count.hidden, true);
  assert.equal(built.el.getAttribute('aria-label'), 'Open full report');
  assert.equal(card.openRow, built.el, 'the same node throughout');
});

test('enter and exit', () => {
  const t = load();
  const { ctx, log, root, state } = t;
  const row = ctx.buildOpenReportRow('7');
  ctx.popupOpener = { id: '7', kind: 'list' };
  row.el.click();
  assert.equal(ctx.panelOpen, true);
  assert.equal(root.getAttribute('data-report-panel'), 'true');
  assert.equal(ctx.detailInPane(), true);
  assert.deepEqual(log.filter((l) => l.startsWith('close') || l.startsWith('open')), ['close 7', 'open 7']);
  assert.deepEqual(state.hostSwitchSeen, ['close:true', 'open:true'], 'the host switch flag is up for both');
  assert.equal(ctx.hostSwitching, false, 'the flag is down again');
  assert.deepEqual(ctx.popupOpener, { id: '7', kind: 'list' }, 'the opener is kept across the host switch');
  assert.equal(log.indexOf('place panel') >= 0 && log.indexOf('place panel') < log.indexOf('close 7'), true, 'placement is re-applied before the reopen');

  // A real close (Back, Escape, browser back, empty map): nothing yet, then exit.
  ctx.panelNoteClose({ id: '7', kind: 'list' });
  assert.equal(ctx.panelOpen, true, 'settled a turn later');
  t.flush();
  assert.equal(ctx.panelOpen, false);
  assert.equal(root.getAttribute('data-report-panel'), null);
  assert.equal(ctx.detailInPane(), false);
  assert.equal(log[log.length - 1], 'focus list 7', 'focus goes back to the row that opened the bubble');
});

test('switch keeps the panel', () => {
  const t = load();
  const { ctx, log, root } = t;
  ctx.popupOpener = { id: '7', kind: 'marker' };
  assert.equal(ctx.enterPanel('7'), true);
  log.length = 0;
  // A pin tap while the panel is open: Leaflet closes the open report, then
  // opens the other one, in the same turn.
  ctx.panelNoteClose({ id: '7', kind: 'marker' });
  ctx.panelNoteOpen();
  t.flush();
  assert.equal(ctx.panelOpen, true);
  assert.equal(root.getAttribute('data-report-panel'), 'true');
  assert.equal(log.some((l) => l.startsWith('focus')), false, 'no focus return on a switch');
  // And the close after the switch still exits.
  ctx.panelNoteClose({ id: '8', kind: 'marker' });
  t.flush();
  assert.equal(ctx.panelOpen, false);
  assert.equal(log[log.length - 1], 'focus marker 8');
});

test('exit does not steal focus the reader moved on purpose', () => {
  const t = load();
  const { ctx, log, doc } = t;
  ctx.enterPanel('7');
  const field = new FakeNode('input');
  doc.activeElement = field;
  ctx.panelNoteClose({ id: '7', kind: 'marker' });
  t.flush();
  assert.equal(ctx.panelOpen, false);
  assert.equal(log.some((l) => l.startsWith('focus')), false);
});

test('list row keeps the bubble on a laptop', () => {
  // Default A (review W10, UI-SPEC Surface 2a item 5): a Nearby row tap opens
  // the bubble. Nothing in the list selection path enters the panel; only the
  // Open report row does.
  const select = between(mapSource, '    Pinalert.onSelect(function (id, source) {', '    centerOnVisitor();');
  assert.equal(/enterPanel/.test(select), false);
  const fly = between(mapSource, '  function flyTo(id, opts) {', '  // PIN_HALF');
  assert.equal(/enterPanel/.test(fly), false);
  const t = load();
  assert.equal(t.ctx.panelOpen, false);
  const callers = mapSource.split('enterPanel(').length - 1;
  // The definition, the row's click and nothing else.
  assert.equal(callers, 2, "enterPanel is defined once and called by the row only");
});

test('phone never enters the panel', () => {
  const t = load({ wide: false });
  const { ctx, log, root } = t;
  assert.equal(ctx.enterPanel('7'), false);
  assert.equal(ctx.panelOpen, false);
  assert.equal(root.getAttribute('data-report-panel'), null);
  assert.equal(log.length, 0);
  assert.equal(ctx.detailInPane(), true, 'the phone is always the detail pane');
  assert.equal(ctx.enterPanel('99'), false, 'an unknown report does nothing');
});

test('the section writes no address and no history', () => {
  const code = section.split("\n").filter((line) => !line.trim().startsWith("//")).join("\n");
  assert.equal(/history|pushState|replaceState|location\./.test(code), false, 'the panel never writes the address (T-08-32)');
});

test('the close and open handlers honour the host switch', () => {
  const close = between(mapSource, '  function onPopupClose(evt) {', '  // endLinkOnClose ends a report link');
  assert.ok(/if \(hostSwitching\) \{\s*hadFocus = false;/.test(close), 'no focus return during the host switch');
  assert.ok(/if \(!hostSwitching\) \{\s*if \(id !== null && String\(id\) === openedFocusId && !resizeClosing\) \{\s*endLinkOnClose/.test(close), 'the report link survives the host switch');
  assert.ok(/if \(!hostSwitching\) \{\s*noteReportClosed\(\);/.test(close), 'the address is not settled during the host switch');
  const open = between(mapSource, '  function onPopupOpen(evt) {', '  // centerPinInStrip puts');
  assert.ok(/if \(!hostSwitching\) \{\s*noteReportOpened\(id\);/.test(open), 'the open does not write the address during the host switch');
});

test('the pan helpers leave the fixed panel alone', () => {
  const tools = between(mapSource, '  function keepCardClearOfTools(popup) {', '    if (map.dragging && map.dragging.moving()) {');
  assert.ok(/phoneDetail\(\) \|\| panelOpen/.test(tools));
  const pan = between(mapSource, '  function panPopupIntoView(popup) {', '    if (map.dragging && map.dragging.moving()) {');
  assert.ok(/if \(panelOpen\) \{\s*return;/.test(pan));
});

test('a closing panel popup is hidden at once and shown again if it reopens', () => {
  const t = load();
  const { ctx } = t;
  ctx.enterPanel('7');
  const popupEl = new FakeNode('div');
  ctx.panelNoteClose({ id: '7', kind: 'marker' }, popupEl);
  assert.equal(popupEl.style.visibility, 'hidden', 'the fading popup does not drift to the corner of the page');
  ctx.panelNoteOpen(popupEl);
  assert.equal(popupEl.style.visibility, '', 'the same container is shown again on a reopen');
  // The host switch itself never hides the popup.
  const other = new FakeNode('div');
  ctx.hostSwitching = true;
  ctx.panelNoteClose({ id: '7', kind: 'marker' }, other);
  assert.notEqual(other.style.visibility, 'hidden', 'the host switch leaves the popup alone');
});
