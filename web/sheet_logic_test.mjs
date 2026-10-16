// sheet_logic_test.mjs - runs the real web/static/js/sheet.js in a Node sandbox
// and checks the pure part of the phone list sheet (Phase 3.4, D-16): where the
// three positions sit for a box height, which position a release lands on,
// what the arrow keys do, and who owns a gesture (the sheet or the list).
//
// The pure functions run with no document at all; the script's DOM glue stays
// idle without one, so no fake element is needed.
//
// Run it with: node --test web/sheet_logic_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'sheet.js'), 'utf8');

function load() {
  const window = {};
  const context = vm.createContext({ window });
  vm.runInContext(source, context, { filename: 'sheet.js' });
  return window.PinalertSheet;
}

const sheet = load();

test('the script exports the pure functions on window.PinalertSheet', () => {
  for (const name of ['snapTops', 'nearestSnap', 'stepSnap', 'gestureOwner']) {
    assert.equal(typeof sheet[name], 'function', name);
  }
});

test('snapTops(719) gives full 0, half 324 and short 511', () => {
  assert.deepEqual({ ...sheet.snapTops(719) }, { full: 0, half: 324, short: 511 });
});

test('snapTops(844) keeps short at 208 px of sheet and half at 45 percent', () => {
  const tops = sheet.snapTops(844);
  assert.equal(844 - tops.short, 208);
  assert.equal(tops.half, Math.round(0.45 * 844));
});

test('snapTops(150) clamps short to 0 and keeps the order full, half, short', () => {
  const tops = sheet.snapTops(150);
  assert.equal(tops.short, 0);
  assert.ok(tops.full <= tops.half, 'full is at or above half');
  assert.ok(tops.half <= tops.short, 'half is at or above short');
});

test('snapTops never goes negative or out of order for any box height', () => {
  for (let h = 0; h <= 1400; h += 7) {
    const tops = sheet.snapTops(h);
    assert.ok(tops.full === 0, 'full is the top of the box at ' + h);
    assert.ok(tops.full <= tops.half && tops.half <= tops.short, 'order at ' + h);
    assert.ok(tops.short >= 0, 'short is not negative at ' + h);
    assert.ok(tops.short <= Math.max(0, h), 'short stays inside the box at ' + h);
  }
});

test('snapTops treats a missing or bad height as an empty box', () => {
  for (const bad of [undefined, null, NaN, -20, 'x']) {
    assert.deepEqual({ ...sheet.snapTops(bad) }, { full: 0, half: 0, short: 0 });
  }
});

test('nearestSnap picks the closest top', () => {
  const tops = sheet.snapTops(719);
  assert.equal(sheet.nearestSnap(tops, 10, 324, 'half'), 'full');
  assert.equal(sheet.nearestSnap(tops, 330, 324, 'half'), 'half');
  assert.equal(sheet.nearestSnap(tops, 500, 324, 'half'), 'short');
});

test('a release within 32 px of the start stays on the start position', () => {
  const tops = sheet.snapTops(719);
  assert.equal(sheet.nearestSnap(tops, 324, 324, 'half'), 'half');
  assert.equal(sheet.nearestSnap(tops, 324 - 32, 324, 'half'), 'half');
  assert.equal(sheet.nearestSnap(tops, 324 + 32, 324, 'half'), 'half');
  assert.equal(sheet.nearestSnap(tops, 511, 511, 'short'), 'short');
});

test('a move of more than 32 px that lands on the same position moves one step the way it went', () => {
  const tops = sheet.snapTops(719);
  // Dragged up 40 px from half: still nearest to half, but it is a flick up.
  assert.equal(sheet.nearestSnap(tops, 324 - 40, 324, 'half'), 'full');
  // Dragged down 40 px from half: a flick down.
  assert.equal(sheet.nearestSnap(tops, 324 + 40, 324, 'half'), 'short');
  // From short, a flick up goes to half and a flick down stays at short.
  assert.equal(sheet.nearestSnap(tops, 511 - 40, 511, 'short'), 'half');
  assert.equal(sheet.nearestSnap(tops, 511 + 40, 511, 'short'), 'short');
  // From full, a flick down goes to half and a flick up stays at full.
  assert.equal(sheet.nearestSnap(tops, 0 + 40, 0, 'full'), 'half');
  assert.equal(sheet.nearestSnap(tops, 0 - 40, 0, 'full'), 'full');
});

test('a long drag lands on the nearest position, not one step', () => {
  const tops = sheet.snapTops(719);
  assert.equal(sheet.nearestSnap(tops, 520, 0, 'full'), 'short');
  assert.equal(sheet.nearestSnap(tops, 5, 511, 'short'), 'full');
});

test('stepSnap walks full, half, short and stops at the ends', () => {
  assert.equal(sheet.stepSnap('short', 'up'), 'half');
  assert.equal(sheet.stepSnap('half', 'up'), 'full');
  assert.equal(sheet.stepSnap('full', 'up'), 'full');
  assert.equal(sheet.stepSnap('full', 'down'), 'half');
  assert.equal(sheet.stepSnap('half', 'down'), 'short');
  assert.equal(sheet.stepSnap('short', 'down'), 'short');
});

test('stepSnap answers half for a position it does not know', () => {
  assert.equal(sheet.stepSnap('bogus', 'up'), 'full');
  assert.equal(sheet.stepSnap('half', 'sideways'), 'half');
});

test('gestureOwner: a finger moving up at half or short belongs to the sheet', () => {
  assert.equal(sheet.gestureOwner('half', 0, -12), 'sheet');
  assert.equal(sheet.gestureOwner('short', 0, -12), 'sheet');
});

test('gestureOwner: at full with the list scrolled, the list owns the gesture', () => {
  assert.equal(sheet.gestureOwner('full', 40, -12), 'list');
  assert.equal(sheet.gestureOwner('full', 40, 12), 'list');
});

test('gestureOwner: at full and the top of the list, down is the sheet and up is the list', () => {
  assert.equal(sheet.gestureOwner('full', 0, 12), 'sheet');
  assert.equal(sheet.gestureOwner('full', 0, -12), 'list');
});

test('gestureOwner: a list scrolled below its top scrolls back before the sheet moves', () => {
  assert.equal(sheet.gestureOwner('half', 60, 12), 'list');
  assert.equal(sheet.gestureOwner('half', 0, 12), 'sheet');
});

test('gestureOwner: no movement leaves the gesture with the list', () => {
  assert.equal(sheet.gestureOwner('half', 0, 0), 'list');
});

test('the named sizes are the ones of UI-SPEC 2b', () => {
  assert.match(source, /SHEET_SHORT_HEIGHT = 208/);
  assert.match(source, /FLICK_PX = 32/);
});
