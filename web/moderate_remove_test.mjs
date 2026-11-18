// moderate_remove_test.mjs - runs the real web/static/js/moderate.js in a Node
// sandbox against a small fake page and checks quick 261118-rz6 (owner
// 2026-11-18) on the moderator screen, which the Go source tests cannot:
//
//   - Remove report opens the Why remove? box and sends nothing; the box opens
//     with no reason picked, its title focused and Remove off;
//   - the five reasons are one radio group: a press picks one, the arrow keys
//     move the pick, and only the picked reason is in the tab order;
//   - Remove does nothing until a reason is picked, then sends one remove with
//     the reason in the body; Cancel and Escape send nothing and put focus back
//     on the row's Remove report;
//   - the Review asks rows show Why and the author's note as text, with
//     Restore and Keep removed; the Removed rows show Why and Restore;
//   - Restore drops the report from both lists and Keep removed from the asks
//     list only, each lowering the heading counts.
//
// Run it with: node --test web/moderate_remove_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what moderate.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'moderate.js'), 'utf8');

// One selector of the forms moderate.js uses: tag, #id, .class, [attr], [attr="v"],
// and the :not([hidden]) suffix of firstRowButton.
const SELECTOR = /^([a-z0-9]*)(?:#([\w-]+))?(?:\.([\w-]+))?(?:\[([\w-]+)(?:="([^"]*)")?\])?(:not\(\[hidden\]\))?$/i;

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
    this.className = '';
    this.hidden = false;
    this.open = false;
    const classes = new Set();
    this.classList = {
      add: (c) => classes.add(c),
      remove: (c) => classes.delete(c),
      contains: (c) => classes.has(c),
    };
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  get firstChild() { return this.children[0] || null; }
  get nextElementSibling() {
    if (!this.parentNode) { return null; }
    const s = this.parentNode.children;
    return s[s.indexOf(this) + 1] || null;
  }
  get previousElementSibling() {
    if (!this.parentNode) { return null; }
    const s = this.parentNode.children;
    return s[s.indexOf(this) - 1] || null;
  }
  get nextSibling() { return this.nextElementSibling; }
  setAttribute(n, v) { this.attrs[n] = String(v); if (n === 'id') { this.id = String(v); } }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  removeAttribute(n) { delete this.attrs[n]; }
  appendChild(node) {
    if (node.parentNode) { node.parentNode.removeChild(node); }
    this.children.push(node);
    node.parentNode = this;
    if (node.id) { this.doc.byId[node.id] = node; }
    return node;
  }
  insertBefore(node, ref) {
    if (node.parentNode) { node.parentNode.removeChild(node); }
    const at = ref ? this.children.indexOf(ref) : -1;
    if (at < 0) { this.children.push(node); } else { this.children.splice(at, 0, node); }
    node.parentNode = this;
    return node;
  }
  removeChild(node) {
    const at = this.children.indexOf(node);
    if (at >= 0) { this.children.splice(at, 1); }
    node.parentNode = null;
    return node;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  focus() { this.doc.activeElement = this; }
  matches(sel) {
    const m = SELECTOR.exec(sel);
    if (!m) { return false; }
    const [, tag, id, cls, attr, val, notHidden] = m;
    if (tag && this.tagName !== tag.toUpperCase()) { return false; }
    if (id && this.id !== id) { return false; }
    if (cls && !this.className.split(' ').includes(cls)) { return false; }
    if (attr) {
      const have = this.getAttribute(attr);
      if (have === null || (val !== undefined && have !== val)) { return false; }
    }
    if (notHidden && this.hidden) { return false; }
    return true;
  }
  closest(sel) {
    for (let el = this; el; el = el.parentNode) { if (el.matches && el.matches(sel)) { return el; } }
    return null;
  }
  querySelectorAll(sel) {
    const out = [];
    const walk = (el) => { el.children.forEach((c) => { if (c.matches(sel)) { out.push(c); } walk(c); }); };
    walk(this);
    return out;
  }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  fire(type, extra = {}) {
    const evt = { type, target: this, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() {}, ...extra };
    for (let el = this; el; el = el.parentNode) { (el.handlers[type] || []).slice().forEach((fn) => fn(evt)); }
    return evt;
  }
  click() { return this.fire('click'); }
  showModal() { this.open = true; this.shown = (this.shown || 0) + 1; }
  close() { this.open = false; }
}

const flush = async () => { for (let i = 0; i < 30; i += 1) { await Promise.resolve(); } };

function report(id, over = {}) {
  return {
    id, category: 'flood', severity: 'low', description: 'Water on the road ' + id,
    created_at: '2026-11-18T10:00:00Z', expires_at: '2026-11-18T18:00:00Z', serious: false,
    moderation: 'clear', view: 'active', visibility_word: 'Showing to everyone',
    flags: { total: 1, reasons: [], notes: [] }, ...over
  };
}

const REASONS = ['spam', 'abusive', 'false', 'dangerous', 'other'];
const WORDS = { spam: 'Spam', abusive: 'Abusive', false: 'False', dangerous: 'Dangerous', other: 'Other' };

// load builds the page with the five lists and the Why remove? box, evaluates
// moderate.js and answers its queue read with queue.
async function load(queue) {
  const doc = {
    activeElement: null,
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    querySelector(sel) { return sel === '.mod-page' ? doc.root : null; },
  };
  const make = (tag, id, parent) => {
    const el = new FakeEl(doc, tag, id);
    if (id) { doc.byId[id] = el; }
    if (parent) { parent.appendChild(el); }
    return el;
  };
  const root = make('main', '', null);
  root.className = 'mod-page';
  doc.root = root;
  make('button', 'mod-refresh', root);
  make('p', 'mod-refresh-error', root).hidden = true;
  make('form', 'mod-find-form', root);
  make('input', 'mod-find', root);
  make('div', 'mod-find-result', root);
  ['asks', 'auto', 'people', 'pending', 'removed'].forEach((slug) => {
    const section = make('section', '', root);
    make('h2', 'mod-h-' + slug, section);
    make('span', 'mod-count-' + slug, section);
    make('ol', 'mod-list-' + slug, section);
    make('p', 'mod-empty-' + slug, section);
  });
  const box = make('dialog', 'mod-remove-confirm', root);
  make('h2', 'mod-remove-title', box);
  const group = make('div', 'mod-remove-reasons', box);
  const reasons = REASONS.map((slug, i) => {
    const b = make('button', '', group);
    b.className = 'btn mod-reason';
    b.setAttribute('role', 'radio');
    b.setAttribute('aria-checked', 'false');
    b.setAttribute('tabindex', i === 0 ? '0' : '-1');
    b.setAttribute('data-mod-reason', slug);
    b.own = WORDS[slug];
    return b;
  });
  const cancel = make('button', 'mod-remove-cancel', box);
  cancel.setAttribute('data-mod-remove-box', 'cancel');
  const go = make('button', 'mod-remove-go', box);
  go.setAttribute('data-mod-remove-box', 'go');
  go.setAttribute('aria-disabled', 'true');

  const fetches = [];
  const toasts = [];
  const Pinalert = {
    setText(el, text) { el.own = String(text); },
    requestTimeout() { return { signal: undefined, expired: new Promise(() => {}), clear() {} }; },
    relativeTime(iso) { return 'when ' + iso; },
    writtenLine() { return ''; },
    showToast(t) { toasts.push(t); },
    CATEGORY_LABELS: { flood: 'Flood' },
  };
  const PinalertTrust = {
    createSeverityLabel() { return new FakeEl(doc, 'span'); },
    updateSeverityLabel() {},
  };
  const sandbox = {
    document: doc, Pinalert, PinalertTrust,
    fetch: (target, init) => new Promise((resolve) => { fetches.push({ url: target, init, resolve }); }),
    JSON, Promise, Array, Object, String, Number, Math, Date, isFinite, encodeURIComponent,
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);

  const answer = (i, status, body) => fetches[i].resolve({ ok: status >= 200 && status < 300, status, json: async () => body });
  assert.equal(fetches[0].url, '/api/moderation/queue');
  answer(0, 200, queue);
  await flush();
  return { doc, root, box, reasons, cancel, go, fetches, answer, toasts };
}

const QUEUE = {
  asks: [report(7, { moderation: 'removed', view: 'none', visibility_word: undefined, removed_reason: 'false', ask_note: 'I took the photo myself.' })],
  review: [],
  flagged: [report(3)],
  pending: [],
  removed: [
    report(7, { moderation: 'removed', view: 'none', visibility_word: undefined, removed_reason: 'false' }),
    report(8, { moderation: 'removed', view: 'none', visibility_word: undefined, removed_reason: 'text_check' }),
    report(9, { moderation: 'removed', view: 'none', visibility_word: undefined })
  ],
  counts: { asks: 1, review: 0, flagged: 1, pending: 0, removed: 3 }
};

const rowsOf = (env, slug) => env.doc.byId['mod-list-' + slug].children;
const buttonOf = (row, action) => row.querySelector('button[data-mod-action="' + action + '"]');
const checked = (env) => env.reasons.filter((b) => b.getAttribute('aria-checked') === 'true').map((b) => b.getAttribute('data-mod-reason'));

test('Remove report opens the Why remove? box with nothing picked and Remove off, and sends nothing', async () => {
  const env = await load(QUEUE);
  const removeBtn = buttonOf(rowsOf(env, 'people')[0], 'remove');
  assert.equal(removeBtn.getAttribute('aria-haspopup'), 'dialog');
  removeBtn.click();
  assert.equal(env.box.open, true);
  assert.equal(env.doc.activeElement, env.doc.byId['mod-remove-title'], 'the title takes the first focus');
  assert.deepEqual(checked(env), []);
  assert.equal(env.go.getAttribute('aria-disabled'), 'true');
  assert.equal(env.reasons[0].getAttribute('tabindex'), '0');
  assert.ok(env.reasons.slice(1).every((b) => b.getAttribute('tabindex') === '-1'));
  env.go.click();
  assert.equal(env.fetches.length, 1, 'Remove sends nothing until a reason is picked');
  assert.equal(env.box.open, true);
});

test('the reasons are one radio group: a press picks one and the arrow keys move the pick', async () => {
  const env = await load(QUEUE);
  buttonOf(rowsOf(env, 'people')[0], 'remove').click();
  env.reasons[2].click();
  assert.deepEqual(checked(env), ['false']);
  assert.equal(env.go.getAttribute('aria-disabled'), null, 'Remove is on once a reason is picked');
  assert.equal(env.reasons[2].getAttribute('tabindex'), '0');
  assert.ok(env.reasons.filter((_, i) => i !== 2).every((b) => b.getAttribute('tabindex') === '-1'));
  let evt = env.reasons[2].fire('keydown', { key: 'ArrowRight' });
  assert.equal(evt.defaultPrevented, true);
  assert.deepEqual(checked(env), ['dangerous']);
  assert.equal(env.doc.activeElement, env.reasons[3]);
  env.reasons[3].fire('keydown', { key: 'ArrowDown' });
  env.reasons[4].fire('keydown', { key: 'ArrowDown' });
  assert.deepEqual(checked(env), ['spam'], 'the pick wraps at the end');
  env.reasons[0].fire('keydown', { key: 'ArrowLeft' });
  assert.deepEqual(checked(env), ['other'], 'and at the start');
});

test('Remove sends one remove with the picked reason and closes the box', async () => {
  const env = await load(QUEUE);
  const row = rowsOf(env, 'people')[0];
  buttonOf(row, 'remove').click();
  env.reasons[1].click();
  env.go.click();
  assert.equal(env.box.open, false);
  assert.equal(env.fetches.length, 2);
  const sent = env.fetches[1];
  assert.equal(sent.url, '/api/moderation/reports/3/remove');
  assert.equal(sent.init.method, 'POST');
  assert.deepEqual(JSON.parse(sent.init.body), { reason: 'abusive' });
  env.answer(1, 200, { report: report(3, { moderation: 'removed', removed_reason: 'abusive' }) });
  await flush();
  assert.equal(rowsOf(env, 'people').length, 0);
  assert.equal(env.doc.byId['mod-count-people'].textContent, '(0)');
  assert.deepEqual(env.toasts, ['Report 3 removed.']);
});

test('Cancel and Escape close the box, send nothing and put focus back on Remove report', async () => {
  const env = await load(QUEUE);
  const removeBtn = buttonOf(rowsOf(env, 'people')[0], 'remove');
  removeBtn.click();
  env.reasons[0].click();
  env.cancel.click();
  assert.equal(env.box.open, false);
  assert.equal(env.doc.activeElement, removeBtn);
  removeBtn.click();
  assert.deepEqual(checked(env), [], 'the box opens fresh each time');
  const evt = env.box.fire('cancel');
  assert.equal(evt.defaultPrevented, true);
  assert.equal(env.box.open, false);
  assert.equal(env.doc.activeElement, removeBtn);
  assert.equal(env.fetches.length, 1);
});

test('Review asks rows show Why and the note as text with Restore and Keep removed; Removed rows show Why and Restore', async () => {
  const env = await load(QUEUE);
  const ask = rowsOf(env, 'asks')[0];
  assert.match(ask.textContent, /Why: False/);
  assert.match(ask.textContent, /Note: I took the photo myself\./);
  assert.ok(buttonOf(ask, 'restore'));
  assert.ok(buttonOf(ask, 'keep-removed'));
  assert.equal(buttonOf(ask, 'remove'), null);
  assert.equal(buttonOf(ask, 'keep'), null);
  const removed = rowsOf(env, 'removed');
  assert.equal(removed.length, 3);
  assert.match(removed[0].textContent, /Why: False/);
  assert.match(removed[1].textContent, /Why: Text check/);
  assert.doesNotMatch(removed[2].textContent, /Why:/, 'no Why line without a recorded reason');
  removed.forEach((r) => {
    assert.ok(buttonOf(r, 'restore'));
    assert.equal(buttonOf(r, 'keep-removed'), null);
    assert.doesNotMatch(r.textContent, /Note:/);
  });
  assert.equal(env.doc.byId['mod-count-asks'].textContent, '(1)');
  assert.equal(env.doc.byId['mod-count-removed'].textContent, '(3)');
});

test('Keep removed drops the report from the asks list only; Restore drops it from both', async () => {
  const env = await load(QUEUE);
  buttonOf(rowsOf(env, 'asks')[0], 'keep-removed').click();
  assert.equal(env.fetches[1].url, '/api/moderation/reports/7/keep-removed');
  assert.equal(env.fetches[1].init.body, undefined);
  env.answer(1, 200, { report: report(7, { moderation: 'removed' }) });
  await flush();
  assert.equal(rowsOf(env, 'asks').length, 0);
  assert.equal(rowsOf(env, 'removed').length, 3, 'it stays on the Removed list');
  assert.equal(env.doc.byId['mod-count-asks'].textContent, '(0)');
  assert.equal(env.doc.byId['mod-count-removed'].textContent, '(3)');
  assert.deepEqual(env.toasts, ['Report 7 stays removed.']);

  buttonOf(rowsOf(env, 'removed')[0], 'restore').click();
  assert.equal(env.fetches[2].url, '/api/moderation/reports/7/restore');
  env.answer(2, 200, { report: report(7) });
  await flush();
  assert.equal(rowsOf(env, 'removed').length, 2);
  assert.equal(env.doc.byId['mod-count-removed'].textContent, '(2)');
  assert.equal(env.toasts[1], 'Report 7 restored.');
});

test('Restore from the asks list drops the report from the Removed list too', async () => {
  const env = await load(QUEUE);
  buttonOf(rowsOf(env, 'asks')[0], 'restore').click();
  env.answer(1, 200, { report: report(7) });
  await flush();
  assert.equal(rowsOf(env, 'asks').length, 0);
  assert.deepEqual(rowsOf(env, 'removed').map((r) => r.getAttribute('data-mod-key')), ['removed:8', 'removed:9']);
  assert.equal(env.doc.byId['mod-count-asks'].textContent, '(0)');
  assert.equal(env.doc.byId['mod-count-removed'].textContent, '(2)');
});

test('a stale Restore shows the already handled line and keeps the row', async () => {
  const env = await load(QUEUE);
  const row = rowsOf(env, 'removed')[1];
  buttonOf(row, 'restore').click();
  env.answer(1, 409, { error: { field: 'id', message: 'x' } });
  await flush();
  assert.equal(rowsOf(env, 'removed').length, 3);
  assert.match(row.textContent, /This report was already handled\. Refresh the lists\./);
  assert.equal(env.doc.activeElement, buttonOf(row, 'restore'));
});
