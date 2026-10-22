// flag_dialog_test.mjs - runs the real web/static/js/flag.js in a Node
// sandbox and checks the Flag window of plan 04-31 (owner, 2026-10-22,
// 04-UAT test 10), which the Go source tests cannot:
//
//   - Send with no tick box ticked sends nothing, shows "Choose a reason to
//     continue." on the group and moves focus to the first box;
//   - more than one reason can be ticked, and the request body carries the
//     ticked slugs in the dialog's order and the note as typed;
//   - a 201 closes the window, turns the control into Flagged and shows the
//     toast;
//   - a refused note (422 on field note) shows the server's sentence under
//     the box, marks the box invalid, moves focus to it and keeps the ticks
//     and the text; typing hides the sentence again;
//   - the count shows only near the 300 character limit;
//   - opening the window again starts with no ticks and an empty box;
//   - Tab from the last button wraps to the first tick box, and the note box
//     is one of the stops.
//
// Run it with: node --test web/flag_dialog_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what flag.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const flagSource = fs.readFileSync(path.join(here, 'static', 'js', 'flag.js'), 'utf8');

class FakeEl {
  constructor(doc, tag, id) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.id = id || '';
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.attrs = {};
    this.handlers = {};
    this.dataset = {};
    this.className = '';
    this.hidden = false;
    this.disabled = false;
    this.checked = false;
    this.value = '';
    this.name = '';
    this.isConnected = true;
    if (id) { doc.byId[id] = this; }
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) { this.children = []; this.own = String(v); }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { delete this.attrs[n]; }
  appendChild(node) { this.children.push(node); node.parentNode = this; return node; }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  focus() { this.doc.activeElement = this; }
  closest() { return null; }
  getClientRects() { return this.hidden ? [] : [{}]; }
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  all() { return this.children.flatMap((c) => [c, ...c.all()]); }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  querySelectorAll(sel) {
    const all = this.all();
    if (sel === 'input[name="flag-reason"]') { return all.filter((e) => e.tagName === 'INPUT' && e.name === 'flag-reason'); }
    if (sel === 'button') { return all.filter((e) => e.tagName === 'BUTTON'); }
    if (sel === '.modal-panel') { return all.filter((e) => e.className.split(' ').includes('modal-panel')); }
    return [];
  }
  fire(type, evt = {}) {
    const event = { type, target: this, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; }, ...evt };
    // Bubble to the ancestors, as a browser does for these events.
    for (let n = this; n; n = n.parentNode) {
      (n.handlers[type] || []).slice().forEach((fn) => fn(event));
    }
    return event;
  }
}

const flush = async () => { for (let i = 0; i < 12; i += 1) { await Promise.resolve(); } };

// load builds the dialog markup of flag_dialog.html.tmpl as fakes, then runs
// flag.js. fetches keeps each request with a hook to answer it.
function load() {
  const doc = { activeElement: null, byId: {}, body: null, controls: [] };
  const el = (tag, id, cls) => { const e = new FakeEl(doc, tag, id); if (cls) { e.className = cls; } return e; };
  const backdrop = el('div', 'flag-dialog', 'modal-backdrop flag-dialog');
  backdrop.hidden = true;
  const panel = backdrop.appendChild(el('div', '', 'modal-panel flag-dialog__panel'));
  panel.appendChild(el('h2', 'flag-dialog-title'));
  panel.appendChild(el('p', 'flag-dialog-dispute'));
  const group = panel.appendChild(el('fieldset', 'flag-dialog-reasons'));
  const boxes = ['spam', 'abusive', 'false', 'dangerous', 'other'].map((slug) => {
    const label = group.appendChild(el('label', '', 'flag-dialog__reason'));
    const box = label.appendChild(el('input'));
    box.name = 'flag-reason';
    box.value = slug;
    return box;
  });
  const choose = panel.appendChild(el('p', 'flag-dialog-choose'));
  choose.textContent = 'Choose a reason to continue.';
  choose.hidden = true;
  const text = panel.appendChild(el('textarea', 'flag-dialog-text'));
  const count = panel.appendChild(el('p', 'flag-dialog-count'));
  count.hidden = true;
  const textError = panel.appendChild(el('p', 'flag-dialog-text-error'));
  textError.hidden = true;
  const error = panel.appendChild(el('p', 'flag-dialog-error'));
  error.hidden = true;
  const send = panel.appendChild(el('button', 'flag-dialog-send'));
  const back = panel.appendChild(el('button', 'flag-dialog-back'));
  const live = el('div', 'notice-live');

  const fetches = [];
  const toasts = [];
  const locks = [];
  const timers = [];
  const document = {
    get activeElement() { return doc.activeElement; },
    getElementById: (id) => doc.byId[id] || null,
    createElement: (tag) => el(tag),
    querySelectorAll: (sel) => (sel === '.flag-control' ? doc.controls : []),
  };
  const Pinalert = {
    setText(node, s) { node.textContent = s; },
    lockPage(b) { locks.push(['lock', b.id]); },
    unlockPage() { locks.push(['unlock']); },
    showToast(s) { toasts.push(s); },
    formatWait(s) { return s + ' seconds'; },
    requestTimeout() { return { signal: null, expired: new Promise(() => {}), clear() {} }; },
    CATEGORY_LABELS: { flood: 'Flood' },
    relativeTime() { return 'just now'; },
  };
  const window = {
    setTimeout: (fn) => { timers.push(fn); return timers.length; },
  };
  const fetch = (u, opts) => new Promise((resolve, reject) => {
    fetches.push({
      url: u,
      opts,
      body: JSON.parse(opts.body),
      answer(status, body) { resolve({ status, headers: { get: () => null }, json: () => Promise.resolve(body) }); },
      fail() { reject(new Error('offline')); },
    });
  });
  const ctx = { window, document, Pinalert, fetch, Promise, Date, Math, JSON, String, parseInt };
  window.Pinalert = Pinalert;
  vm.createContext(ctx);
  vm.runInContext(flagSource, ctx);
  const api = ctx.window.PinalertFlag;

  const report = { id: 41, category: 'flood', created_at: '2026-10-22T10:00:00Z', is_own_report: false, viewer_flagged: false };
  const control = api.createFlagControl(report);
  doc.controls.push(control.el);
  api.updateFlagControl(control, report);
  const open = () => control.el.fire('click');
  const flushTimers = () => { timers.splice(0).forEach((fn) => fn()); };
  return { doc, backdrop, panel, group, boxes, choose, text, count, textError, error, send, back, live, control, fetches, toasts, locks, open, flushTimers };
}

test('Send with nothing ticked sends nothing and asks for a reason on the group', () => {
  const t = load();
  t.open();
  assert.equal(t.backdrop.hidden, false);
  assert.deepEqual(t.locks[0], ['lock', 'flag-dialog']);
  assert.equal(t.doc.activeElement.id, 'flag-dialog-title');
  t.send.fire('click');
  assert.equal(t.fetches.length, 0);
  assert.equal(t.choose.hidden, false);
  assert.equal(t.group.getAttribute('aria-describedby'), 'flag-dialog-choose');
  assert.equal(t.doc.activeElement, t.boxes[0]);
  t.flushTimers();
  assert.equal(t.live.textContent, 'Choose a reason to continue.');
  // Ticking a box hides the line again.
  t.boxes[2].checked = true;
  t.boxes[2].fire('change');
  assert.equal(t.choose.hidden, true);
  assert.equal(t.group.getAttribute('aria-describedby'), null);
});

test('more than one reason and the note go in one request, in the dialog order', async () => {
  const t = load();
  t.open();
  t.boxes[4].checked = true; // Something else
  t.boxes[0].checked = true; // Spam or gibberish
  t.boxes[2].checked = true; // False or a hoax
  t.text.value = '  The photo is from 2019  ';
  t.send.fire('click');
  assert.equal(t.fetches.length, 1);
  const f = t.fetches[0];
  assert.equal(f.url, '/api/reports/41/flag');
  assert.equal(f.opts.method, 'POST');
  assert.deepEqual(f.body, { reasons: ['spam', 'false', 'other'], note: '  The photo is from 2019  ' });
  assert.equal(t.send.textContent, 'Sending');
  assert.equal(t.send.disabled, true);
  f.answer(201, { flagged: true });
  await flush();
  assert.equal(t.backdrop.hidden, true);
  assert.deepEqual(t.toasts, ['Flag sent. Moderators will see it.']);
  assert.equal(t.control.el.textContent, 'Flagged');
  assert.equal(t.control.el.getAttribute('aria-disabled'), 'true');
});

test('a refused note shows the server sentence under the box and keeps the ticks and the text', async () => {
  const t = load();
  t.open();
  t.boxes[1].checked = true;
  t.boxes[3].checked = true;
  t.text.value = 'some hateful words';
  t.send.fire('click');
  const sentence = "This text looks hateful, so it can't be posted. Reword it and try again.";
  t.fetches[0].answer(422, { error: { field: 'note', message: sentence } });
  await flush();
  assert.equal(t.backdrop.hidden, false);
  assert.equal(t.textError.hidden, false);
  assert.equal(t.textError.textContent, sentence);
  assert.equal(t.textError.getAttribute('role'), 'alert');
  assert.equal(t.text.getAttribute('aria-invalid'), 'true');
  assert.equal(t.text.getAttribute('aria-describedby'), 'flag-dialog-text-error');
  assert.equal(t.doc.activeElement, t.text);
  assert.equal(t.error.hidden, true);
  assert.deepEqual(t.boxes.map((b) => b.checked), [false, true, false, true, false]);
  assert.equal(t.text.value, 'some hateful words');
  assert.equal(t.send.disabled, false);
  // Editing the note hides the sentence; the next send carries the edit.
  t.text.value = 'The water went down an hour ago';
  t.text.fire('input');
  assert.equal(t.textError.hidden, true);
  assert.equal(t.text.getAttribute('aria-invalid'), null);
  t.send.fire('click');
  assert.deepEqual(t.fetches[1].body, { reasons: ['abusive', 'dangerous'], note: 'The water went down an hour ago' });
});

test('a reasons error from the server shows the group line, any other error the general line', async () => {
  const t = load();
  t.open();
  t.boxes[0].checked = true;
  t.send.fire('click');
  t.fetches[0].answer(400, { error: { field: 'reasons', message: 'Choose a reason to continue.' } });
  await flush();
  assert.equal(t.choose.hidden, false);
  t.send.fire('click');
  t.fetches[1].answer(500, {});
  await flush();
  assert.equal(t.error.hidden, false);
  assert.equal(t.error.textContent, 'Something went wrong. Try again.');
  assert.equal(t.textError.hidden, true);
});

test('the count shows only near the limit', () => {
  const t = load();
  t.open();
  t.text.value = 'a'.repeat(249);
  t.text.fire('input');
  assert.equal(t.count.hidden, true);
  t.text.value = 'a'.repeat(250);
  t.text.fire('input');
  assert.equal(t.count.hidden, false);
  assert.equal(t.count.textContent, '50 characters left');
  t.text.value = 'a'.repeat(299);
  t.text.fire('input');
  assert.equal(t.count.textContent, '1 character left');
  t.text.value = 'a'.repeat(300);
  t.text.fire('input');
  assert.equal(t.count.textContent, '0 characters left');
  t.text.value = 'short';
  t.text.fire('input');
  assert.equal(t.count.hidden, true);
});

test('Go back and open again start with no ticks and an empty box', () => {
  const t = load();
  t.open();
  t.boxes[0].checked = true;
  t.text.value = 'a'.repeat(260);
  t.text.fire('input');
  t.back.fire('click');
  assert.equal(t.backdrop.hidden, true);
  t.open();
  assert.deepEqual(t.boxes.map((b) => b.checked), [false, false, false, false, false]);
  assert.equal(t.text.value, '');
  assert.equal(t.count.hidden, true);
});

test('Tab wraps from the last button to the first tick box, Shift Tab back, and the note box is a stop', () => {
  const t = load();
  t.open();
  t.back.focus();
  const tab = t.back.fire('keydown', { key: 'Tab', shiftKey: false });
  assert.equal(tab.defaultPrevented, true);
  assert.equal(t.doc.activeElement, t.boxes[0]);
  const shift = t.boxes[0].fire('keydown', { key: 'Tab', shiftKey: true });
  assert.equal(shift.defaultPrevented, true);
  assert.equal(t.doc.activeElement, t.back);
  // The panel moves focus itself through every stop, so a browser whose
  // default Tab skips tick boxes and buttons (Safari) cannot drop focus on the
  // page behind: last box, note box, Send, Go back, first box.
  t.boxes[4].focus();
  t.boxes[4].fire('keydown', { key: 'Tab', shiftKey: false });
  assert.equal(t.doc.activeElement, t.text);
  const fromText = t.text.fire('keydown', { key: 'Tab', shiftKey: false });
  assert.equal(fromText.defaultPrevented, true);
  assert.equal(t.doc.activeElement, t.send);
  t.send.fire('keydown', { key: 'Tab', shiftKey: true });
  assert.equal(t.doc.activeElement, t.text);
  t.text.fire('keydown', { key: 'Tab', shiftKey: true });
  assert.equal(t.doc.activeElement, t.boxes[4]);
  // From the title, Tab goes to the first box and Shift Tab to the last button.
  t.doc.byId['flag-dialog-title'].focus();
  t.doc.byId['flag-dialog-title'].fire('keydown', { key: 'Tab', shiftKey: false });
  assert.equal(t.doc.activeElement, t.boxes[0]);
  t.doc.byId['flag-dialog-title'].focus();
  t.doc.byId['flag-dialog-title'].fire('keydown', { key: 'Tab', shiftKey: true });
  assert.equal(t.doc.activeElement, t.back);
  // Escape closes.
  t.text.fire('keydown', { key: 'Escape' });
  assert.equal(t.backdrop.hidden, true);
});
