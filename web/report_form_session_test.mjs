// report_form_session_test.mjs - runs the real web/static/js/report-form.js in
// a Node sandbox and checks WR-W03: the answer to a post whose form was closed
// while the post was out never touches the form open now (its fields, its
// message, its button), and the person still learns how that earlier post
// ended: posted, held, saved on the device or refused.
//
// Run it with: node --test web/report_form_session_test.mjs
// It is not part of CI (CI runs Go only) and is not embedded in the server.
// The fakes cover only what report-form.js calls: a minimal element model, a
// fake Pinalert whose submitReport answers when the test says so, a fake
// queue and a location object that is always set.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';
import crypto from 'node:crypto';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'report-form.js'), 'utf8');

const EARLIER_POSTED = 'Your earlier report was posted.';
const EARLIER_HELD = "Your earlier report was received. It isn't shown yet. A text check has to finish first.";
const EARLIER_SAVED = "No signal. Your earlier report is saved on this device and will send when you're back online.";
const EARLIER_NOT_SAVED = 'Your earlier report could not be saved on this device, so it was not sent.';

// ----- fake element model -----

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
    this.required = false;
    this.value = '';
    this.selectedIndex = 0;
    this.tabIndex = 0;
    this.type = '';
    const self = this;
    this.classList = {
      add: (...names) => names.forEach((n) => { if (!self.classes().includes(n)) { self.className = (self.className + ' ' + n).trim(); } }),
      remove: (...names) => { self.className = self.classes().filter((c) => !names.includes(c)).join(' '); },
      toggle: (n, on) => { if (on) { self.classList.add(n); } else { self.classList.remove(n); } },
      contains: (n) => self.classes().includes(n)
    };
  }
  classes() { return this.className.split(/\s+/).filter(Boolean); }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  get previousElementSibling() {
    if (!this.parentNode) { return null; }
    const at = this.parentNode.children.indexOf(this);
    return at > 0 ? this.parentNode.children[at - 1] : null;
  }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  removeAttribute(n) { delete this.attrs[n]; }
  appendChild(node) { return this.insertBefore(node, null); }
  insertBefore(node, ref) {
    if (node.parentNode) { node.parentNode.removeChild(node); }
    const at = ref ? this.children.indexOf(ref) : -1;
    if (at === -1) { this.children.push(node); } else { this.children.splice(at, 0, node); }
    node.parentNode = this;
    return node;
  }
  removeChild(node) {
    const at = this.children.indexOf(node);
    if (at !== -1) { this.children.splice(at, 1); }
    node.parentNode = null;
    return node;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener(type, fn) { this.handlers[type] = (this.handlers[type] || []).filter((h) => h !== fn); }
  dispatchEvent(evt) {
    if (!evt.target) { evt.target = this; }
    evt.currentTarget = this;
    (this.handlers[evt.type] || []).slice().forEach((h) => h(evt));
    return true;
  }
  click() { this.dispatchEvent({ type: 'click', preventDefault() {} }); }
  input() { this.dispatchEvent({ type: 'input' }); }
  focus() { this.doc.activeElement = this; }
  closest() { return null; }
  descendants() {
    const out = [];
    for (const c of this.children) { out.push(c, ...c.descendants()); }
    return out;
  }
  querySelectorAll(selector) {
    if (!selector.startsWith('.') || selector.includes(',')) { return []; }
    const cls = selector.slice(1);
    return this.descendants().filter((el) => el.classList.contains(cls));
  }
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
}

// ----- the page -----

function setup() {
  const doc = {
    activeElement: null,
    handlers: {},
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    addEventListener(type, fn) { (doc.handlers[type] = doc.handlers[type] || []).push(fn); },
    removeEventListener(type, fn) { doc.handlers[type] = (doc.handlers[type] || []).filter((h) => h !== fn); }
  };
  const make = (tag, id, parent) => {
    const el = new FakeEl(doc, tag, id);
    if (id) { doc.byId[id] = el; }
    if (parent) { parent.appendChild(el); }
    return el;
  };
  const backdrop = make('div', 'report-backdrop');
  backdrop.hidden = true;
  const form = make('div', 'report-form', backdrop);
  make('div', 'category-grid', form);
  make('label', '', form);
  const severity = make('input', 'severity', form);
  severity.value = '1';
  make('output', 'severity-readout', form);
  make('textarea', 'description', form);
  make('p', 'form-error', form);
  make('input', 'location-search', form);
  make('div', 'location-search-results', form);
  make('p', 'location-search-status', form);
  make('div', 'shelter-fields', form);
  make('select', 'shelter-capacity-status', form);
  make('input', 'shelter-headcount', form);
  make('div', 'discard-confirm', form).hidden = true;
  make('button', 'discard-confirm-discard', form);
  make('button', 'discard-confirm-keep', form);
  make('p', 'form-note', form).hidden = true;
  make('button', 'send-after-wait', form).hidden = true;
  make('button', 'cancel-report', form);
  make('button', 'submit-report', form);
  make('div', 'notice-live');
  const opener = make('button', 'open-report');

  const toasts = [];
  const posts = [];
  const queueSaves = [];
  const queueDiscards = [];
  let queueSaveAnswer = () => Promise.resolve(true);

  const Pinalert = {
    CATEGORIES: ['flood', 'road_blocked', 'shelter_open'],
    CATEGORY_LABELS: { flood: 'Flood', road_blocked: 'Road blocked', shelter_open: 'Shelter open' },
    CAPACITY_STATUSES: ['available', 'limited', 'full', 'closed'],
    SEVERITY_LABELS: { low: '1 · Low', medium: '2 · Medium', critical: '3 · Critical' },
    iconClass: (c) => 'icon-glyph icon-glyph--' + c,
    setText: (el, v) => { el.textContent = v === null || v === undefined ? '' : String(v); },
    showToast: (message) => { toasts.push(message); },
    lockPage: () => {},
    unlockPage: () => {},
    submitReport: (payload) => new Promise((resolve, reject) => {
      posts.push({ payload: JSON.parse(JSON.stringify(payload)), resolve, reject });
    })
  };
  const PinalertQueue = {
    save: (record) => { queueSaves.push(JSON.parse(JSON.stringify(record))); return queueSaveAnswer(); },
    discard: (id) => { queueDiscards.push(id); }
  };
  const picker = {
    current: () => ({ latitude: 12.97, longitude: 77.59 }),
    choose: () => {},
    isSet: () => true,
    missingMessage: () => 'Set a location.',
    searchFailureMessage: () => 'Search failed.'
  };
  class CustomEvent {
    constructor(type, init) { this.type = type; this.detail = init && init.detail; }
  }
  const window = {
    setTimeout: () => 0,
    clearTimeout: () => {},
    PinalertQueue
  };
  const sandbox = { window, document: doc, CustomEvent, crypto, Map, Date, Math, JSON, Promise, Array, Object, isFinite, encodeURIComponent };
  window.Pinalert = Pinalert;
  sandbox.Pinalert = Pinalert;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  const Form = window.PinalertReportForm;
  Form.mount({ backdrop, form, location: picker });

  const el = (id) => doc.byId[id];
  return {
    Form,
    el,
    opener,
    toasts,
    posts,
    queueSaves,
    queueDiscards,
    setQueueSave(fn) { queueSaveAnswer = fn; },
    open(options) { Form.open(Object.assign({ opener }, options || {})); },
    fill(text) {
      el('category-grid').querySelector('.category-tile').click();
      el('description').value = text;
      el('description').input();
    },
    submit() { el('submit-report').click(); },
    discard() {
      el('cancel-report').click();
      assert.equal(el('discard-confirm').hidden, false, 'a touched form asks before it closes');
      el('discard-confirm-discard').click();
    }
  };
}

const settle = () => new Promise((resolve) => setImmediate(resolve));

// postThenReopen sends a first report, discards the form while the post is
// out, opens the form again and types a second draft.
function postThenReopen(page, openOptions) {
  page.open(openOptions);
  page.fill('Water over the main road near the bridge');
  page.submit();
  assert.equal(page.posts.length, 1);
  assert.equal(page.el('submit-report').disabled, true);
  page.discard();
  assert.equal(page.el('report-backdrop').hidden, true);
  page.open();
  page.fill('A second report about the shelter');
}

function assertNewDraftUntouched(page) {
  assert.equal(page.el('report-backdrop').hidden, false, 'the open form must stay open');
  assert.equal(page.el('description').value, 'A second report about the shelter', 'the new draft must be kept');
  assert.equal(page.el('form-error').textContent, '', 'the open form must show no message');
  assert.equal(page.el('submit-report').disabled, false, 'the open form keeps a working button');
  assert.equal(page.el('submit-report').textContent, 'Post report');
}

test('a reopened form gets a working Post button while the earlier post is out', () => {
  const page = setup();
  postThenReopen(page);
  assertNewDraftUntouched(page);
});

test('an earlier post that succeeds leaves the new draft alone and says it was posted', async () => {
  const page = setup();
  postThenReopen(page);
  page.posts[0].resolve({ id: 41 });
  await settle();
  assertNewDraftUntouched(page);
  assert.deepEqual(page.toasts, [EARLIER_POSTED]);
});

test('an earlier post that is held says so', async () => {
  const page = setup();
  postThenReopen(page);
  page.posts[0].resolve({ id: 42, held: true });
  await settle();
  assertNewDraftUntouched(page);
  assert.deepEqual(page.toasts, [EARLIER_HELD]);
});

test('an earlier post that gets no answer is saved under its own id and time', async () => {
  const page = setup();
  postThenReopen(page);
  const first = page.posts[0];
  first.reject(Object.assign(new Error('x'), { queueable: true, status: 0, fieldMessage: 'Something went wrong. Try again.' }));
  await settle();
  assertNewDraftUntouched(page);
  assert.equal(page.queueSaves.length, 1);
  const saved = page.queueSaves[0];
  assert.equal(saved.request_id, first.payload.request_id);
  assert.match(saved.request_id, /^[A-Za-z0-9-]{16,64}$/);
  assert.equal(saved.payload.description, 'Water over the main road near the bridge');
  assert.ok(typeof saved.written_at === 'string' && saved.written_at !== '', 'the written time must be kept');
  assert.equal(saved.status, 'waiting');
  assert.deepEqual(page.toasts, [EARLIER_SAVED]);
});

test('an earlier post that cannot be saved says it was not sent', async () => {
  const page = setup();
  page.setQueueSave(() => Promise.reject(new Error('not valid')));
  postThenReopen(page);
  page.posts[0].reject(Object.assign(new Error('x'), { queueable: true, status: 503 }));
  await settle();
  assertNewDraftUntouched(page);
  assert.deepEqual(page.toasts, [EARLIER_NOT_SAVED]);
});

test('an earlier post that is refused says why and leaves the new form alone', async () => {
  const page = setup();
  postThenReopen(page);
  page.el('severity').focus();
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 422, field: 'description', fieldMessage: 'That text is not allowed.'
  }));
  await settle();
  assertNewDraftUntouched(page);
  assert.equal(page.el('severity').doc.activeElement, page.el('severity'), 'focus must not move in the new form');
  assert.deepEqual(page.toasts, ['Your earlier report was not posted. That text is not allowed.']);
  assert.equal(page.queueSaves.length, 0);
});

test('a refused earlier post over the serious limit still gives the emergency number', async () => {
  const page = setup();
  postThenReopen(page);
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 429, fieldMessage: 'Please wait 5 minutes.', retryAfterSeconds: 300,
    serious: true, emergencyNumber: '112'
  }));
  await settle();
  assertNewDraftUntouched(page);
  assert.equal(page.el('send-after-wait').hidden, true, 'the new form must not offer Send after the wait');
  assert.deepEqual(page.toasts, ['Your earlier report was not posted. Please wait 5 minutes. In an emergency, call 112 now.']);
});

test('an earlier resend that succeeds removes only its own saved report', async () => {
  const page = setup();
  postThenReopen(page, { requestId: 'aaaaaaaaaaaaaaaa1111', prefill: { written_at: '2026-10-16T08:00:00Z' } });
  page.Form.close();
  page.open({ requestId: 'bbbbbbbbbbbbbbbb2222', prefill: { written_at: '2026-10-16T09:00:00Z' } });
  page.fill('A second report about the shelter');
  page.posts[0].resolve({ id: 43 });
  await settle();
  assert.deepEqual(page.queueDiscards, ['aaaaaaaaaaaaaaaa1111']);
  assert.equal(page.posts[0].payload.request_id, 'aaaaaaaaaaaaaaaa1111');
  assertNewDraftUntouched(page);
});

test('a post answered while its form is still open closes it as before', async () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].resolve({ id: 44 });
  await settle();
  assert.equal(page.el('report-backdrop').hidden, true);
  assert.equal(page.el('description').value, '');
  assert.deepEqual(page.toasts, ['Report posted.']);
});

test('a post refused while its form is still open shows the message in the form', async () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 422, field: 'description', fieldMessage: 'That text is not allowed.'
  }));
  await settle();
  assert.equal(page.el('report-backdrop').hidden, false);
  assert.equal(page.el('form-error').textContent, 'That text is not allowed.');
  assert.equal(page.el('description').value, 'Water over the main road near the bridge');
  assert.deepEqual(page.toasts, []);
});

test('a post with no answer while its form is still open saves and closes as before', async () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  const first = page.posts[0];
  first.reject(Object.assign(new Error('x'), { queueable: true, status: 504 }));
  await settle();
  assert.equal(page.el('report-backdrop').hidden, true);
  assert.equal(page.queueSaves.length, 1);
  assert.equal(page.queueSaves[0].request_id, first.payload.request_id);
  assert.equal(page.queueSaves[0].status, 'no_answer');
  assert.deepEqual(page.toasts, ["No signal. Your report is saved on this device and will send when you're back online."]);
});
