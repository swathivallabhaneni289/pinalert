// report_form_session_test.mjs - runs the real web/static/js/report-form.js in
// a Node sandbox and checks WR-W03: the answer to a post whose form was closed
// while the post was out never touches the form open now (its fields, its
// message, its button), and the person still learns how that earlier post
// ended: posted, held, saved on the device or refused.
//
// Run it with: node --test web/report_form_session_test.mjs
// CI runs it with the other web/*.mjs files (node --test web/*.mjs in
// .github/workflows/ci.yml). It is not embedded in the server.
// The fakes cover only what report-form.js calls: a minimal element model, a
// fake Pinalert whose submitReport answers when the test says so, a fake
// queue and a location object that is always set.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-23, D-24,
// D-24a): setup({ window: true }) builds the Post a report window form (three
// severity boxes in #severity-group and the X, #close-report) next to the Low
// data form the tests above use (the slider and Cancel report). The tests at
// the end cover the window mode: eight tiles from WINDOW_TILES, the severity
// boxes, the X, and a report queued before the change (Rescue needed or
// Earthquake) reopened through Edit and resend. The Phase 3.1 and Phase 4
// hands-on tests are walked on this layout after Phase 3.4 is built.

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
const EARLIER_NOT_SAVED = "Your earlier report couldn't be saved on this device, so it wasn't sent.";

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
  // A disabled button fires no click, as in a browser.
  click() { if (!this.disabled) { this.dispatchEvent({ type: 'click', preventDefault() {} }); } }
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

function setup(options) {
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
  if (options && options.window) {
    // The Post a report window: three severity boxes instead of the slider, and
    // the X in the window header instead of Cancel report.
    const group = make('div', 'severity-group', form);
    for (const key of ['low', 'medium', 'critical']) {
      const box = make('button', '', group);
      box.className = 'severity-box severity-box--' + key;
      box.dataset.severity = key;
      box.setAttribute('role', 'radio');
      box.setAttribute('aria-checked', key === 'low' ? 'true' : 'false');
      box.tabIndex = key === 'low' ? 0 : -1;
    }
  } else {
    make('label', '', form);
    const severity = make('input', 'severity', form);
    severity.value = '1';
    make('output', 'severity-readout', form);
  }
  make('textarea', 'description', form);
  make('p', 'form-error', form);
  make('input', 'location-search', form);
  make('div', 'location-search-results', form);
  make('p', 'location-search-status', form);
  make('div', 'shelter-fields', form).hidden = true;
  make('select', 'shelter-capacity-status', form);
  make('input', 'shelter-headcount', form);
  make('div', 'discard-confirm', form).hidden = true;
  make('button', 'discard-confirm-discard', form);
  make('button', 'discard-confirm-keep', form);
  make('p', 'form-note', form).hidden = true;
  make('button', 'send-after-wait', form).hidden = true;
  make('button', options && options.window ? 'close-report' : 'cancel-report', options && options.window ? backdrop : form);
  make('button', 'submit-report', form);
  make('div', 'notice-live');
  const opener = make('button', 'open-report');

  const toasts = [];
  const toastActions = [];
  const posts = [];
  const queueSaves = [];
  const queueDiscards = [];
  let queueSaveAnswer = () => Promise.resolve(true);

  const Pinalert = {
    CATEGORIES: ['flood', 'road_blocked', 'shelter_open', 'earthquake'],
    CATEGORY_LABELS: { flood: 'Flood', road_blocked: 'Road blocked', shelter_open: 'Shelter open', earthquake: 'Earthquake' },
    CAPACITY_STATUSES: ['available', 'limited', 'full', 'closed'],
    SEVERITY_LABELS: { low: '1 · Low', medium: '2 · Medium', critical: '3 · Critical' },
    iconClass: (c) => 'icon-glyph icon-glyph--' + c,
    // The same rounding as app.js: seconds under a minute, then minutes, then hours.
    formatWait: (seconds) => {
      const s = Math.max(1, Math.ceil(Number(seconds)));
      if (s < 60) { return s + (s === 1 ? ' second' : ' seconds'); }
      if (s < 3600) { const m = Math.ceil(s / 60); return m + (m === 1 ? ' minute' : ' minutes'); }
      const h = Math.ceil(s / 3600);
      return h + (h === 1 ? ' hour' : ' hours');
    },
    setText: (el, v) => { el.textContent = v === null || v === undefined ? '' : String(v); },
    showToast: (message, action) => { toasts.push(message); toastActions.push(action || null); },
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
  if (options && typeof options.pickerReset === 'function') {
    picker.reset = options.pickerReset;
  }
  class CustomEvent {
    constructor(type, init) { this.type = type; this.detail = init && init.detail; }
  }
  const window = {
    setTimeout: () => 0,
    clearTimeout: () => {},
    location: { href: 'https://pinalert.example/' },
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
    toastActions,
    window,
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
      el(options && options.window ? 'close-report' : 'cancel-report').click();
      assert.equal(el('discard-confirm').hidden, false, 'a touched form asks before it closes');
      el('discard-confirm-discard').click();
    },
    // cancelInFlight closes the form while its post is out: no Discard
    // question, since that report is already on its way, and focus goes back
    // to the button that opened the form.
    cancelInFlight() {
      el(options && options.window ? 'close-report' : 'cancel-report').click();
      assert.equal(el('discard-confirm').hidden, true, 'a form whose post is out closes without the Discard question');
      assert.equal(el('report-backdrop').hidden, true, 'the form must close');
      assert.equal(doc.activeElement, opener, 'focus must go back to the opener');
    },
    escape() {
      (doc.handlers.keydown || []).slice().forEach((h) => h({ key: 'Escape', preventDefault() {} }));
    }
  };
}

const settle = () => new Promise((resolve) => setImmediate(resolve));

// postThenReopen sends a first report, closes the form while the post is out
// (with no Discard question), opens the form again and types a second draft.
function postThenReopen(page, openOptions) {
  page.open(openOptions);
  page.fill('Water over the main road near the bridge');
  page.submit();
  assert.equal(page.posts.length, 1);
  assert.equal(page.el('submit-report').disabled, true);
  page.cancelInFlight();
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

// The server's 429 sentences end like this one, which is true only while the
// form is open (CR-01).
const SERIOUS_LIMIT = "You've posted several serious reports in a short time. You can post again in 5 minutes. Your text is still here.";
const EARLIER_REFUSED_SAVED = "It's saved on this device under Waiting to send, so you can edit it and send it again.";

function assertNoFormClaim(page) {
  page.toasts.forEach((t) => {
    assert.ok(!t.includes('Your text is still here.'), 'the form is gone, so no toast may say the text is still here: ' + t);
  });
}

test('an earlier post that is refused is saved as Not sent and says why (WR-01)', async () => {
  const page = setup();
  postThenReopen(page);
  page.el('severity').focus();
  const first = page.posts[0];
  first.reject(Object.assign(new Error('x'), {
    queueable: false, status: 422, field: 'description', fieldMessage: 'That text is not allowed.'
  }));
  await settle();
  assertNewDraftUntouched(page);
  assert.equal(page.el('severity').doc.activeElement, page.el('severity'), 'focus must not move in the new form');
  assert.equal(page.queueSaves.length, 1, 'the report is kept on the device');
  const saved = page.queueSaves[0];
  assert.equal(saved.request_id, first.payload.request_id);
  assert.equal(saved.status, 'rejected');
  assert.equal(saved.last_error, 'That text is not allowed.');
  assert.equal(saved.payload.description, 'Water over the main road near the bridge');
  assert.ok(typeof saved.written_at === 'string' && saved.written_at !== '');
  assert.deepEqual(page.toasts, ["Your earlier report wasn't posted. That text is not allowed. " + EARLIER_REFUSED_SAVED]);
});

test('an earlier post refused with no reason is saved as Not sent with the plain reason (WR-01)', async () => {
  const page = setup();
  postThenReopen(page);
  page.posts[0].reject(Object.assign(new Error('x'), { queueable: false, status: 500 }));
  await settle();
  assert.equal(page.queueSaves[0].status, 'rejected');
  assert.equal(page.queueSaves[0].last_error, 'Something went wrong. Try again.');
  assert.deepEqual(page.toasts, ["Your earlier report wasn't posted. Something went wrong. " + EARLIER_REFUSED_SAVED]);
});

test('an earlier post refused while signed out waits for a sign in (WR-01)', async () => {
  const page = setup();
  postThenReopen(page);
  page.posts[0].reject(Object.assign(new Error('x'), { queueable: false, status: 401, fieldMessage: 'Sign in first.' }));
  await settle();
  assertNewDraftUntouched(page);
  assert.equal(page.queueSaves[0].status, 'signed_out');
  assert.deepEqual(page.toasts,
    ["Your earlier report wasn't posted because you're signed out. It's saved on this device. Sign in again to send it."]);
});

test('an earlier post over the serious limit is saved to send after the wait, with the emergency number (CR-01)', async () => {
  const page = setup();
  postThenReopen(page);
  const before = Date.now();
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 429, fieldMessage: SERIOUS_LIMIT, retryAfterSeconds: 300,
    serious: true, emergencyNumber: '112'
  }));
  await settle();
  assertNewDraftUntouched(page);
  assert.equal(page.el('send-after-wait').hidden, true, 'the new form must not offer Send after the wait');
  assert.equal(page.queueSaves.length, 1, 'the report is kept on the device');
  const saved = page.queueSaves[0];
  assert.equal(saved.request_id, page.posts[0].payload.request_id);
  assert.equal(saved.status, 'blocked');
  assert.ok(saved.next_try_at >= before + 300000 && saved.next_try_at <= Date.now() + 300000);
  assert.equal(saved.serious, true);
  assert.equal(saved.emergency_number, '112');
  assert.equal(saved.payload.description, 'Water over the main road near the bridge');
  assert.deepEqual(page.toasts, ["Your earlier report has to wait before it can be posted. It's saved on this device and " +
    'will send by itself in 5 minutes. In an emergency, call 112 now.']);
  assertNoFormClaim(page);
  const action = page.toastActions[0];
  assert.ok(action, 'the serious toast must carry a Call action');
  assert.equal(action.label, 'Call 112');
  assert.equal(action.ariaLabel, 'Call 112');
  const next = action.onClick();
  assert.equal(page.window.location.href, 'tel:112', 'a press dials the number');
  assert.ok(next === null || next === undefined, 'the toast closes after the press');
  assertNewDraftUntouched(page);
});

test('an earlier post over a limit that is not serious has no Call action (CR-01)', async () => {
  const page = setup();
  postThenReopen(page);
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 429, fieldMessage: 'Please wait 5 minutes. Your text is still here.', retryAfterSeconds: 300,
    serious: false, emergencyNumber: ''
  }));
  await settle();
  assert.equal(page.queueSaves[0].status, 'blocked');
  assert.equal(page.queueSaves[0].serious, false);
  assert.deepEqual(page.toasts, ["Your earlier report has to wait before it can be posted. It's saved on this device and " +
    'will send by itself in 5 minutes.']);
  assert.deepEqual(page.toastActions, [null]);
});

test('an earlier wait of 0 seconds still waits at least 5 seconds (CR-01)', async () => {
  const page = setup();
  postThenReopen(page);
  const before = Date.now();
  page.posts[0].reject(Object.assign(new Error('x'), { queueable: false, status: 429, retryAfterSeconds: 0 }));
  await settle();
  assert.ok(page.queueSaves[0].next_try_at >= before + 5000);
});

test('an earlier refusal on a browser that cannot keep reports is kept on the page (CR-01, WR-01)', async () => {
  const page = setup();
  page.setQueueSave(() => Promise.resolve(false));
  postThenReopen(page);
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 429, fieldMessage: SERIOUS_LIMIT, retryAfterSeconds: 60, serious: true, emergencyNumber: '112'
  }));
  await settle();
  assert.deepEqual(page.toasts, ["Your earlier report has to wait before it can be posted. It's kept on this page only. " +
    "Keep the page open and it'll send by itself in 1 minute. In an emergency, call 112 now."]);
  assert.ok(page.toastActions[0], 'the serious toast keeps its Call action');

  const other = setup();
  other.setQueueSave(() => Promise.resolve(false));
  postThenReopen(other);
  other.posts[0].reject(Object.assign(new Error('x'), { queueable: false, status: 422, fieldMessage: 'That text is not allowed.' }));
  await settle();
  assert.deepEqual(other.toasts, ["Your earlier report wasn't posted. That text is not allowed. It's kept on this page only, " +
    'under Waiting to send. Keep the page open to edit it and send it again.']);

  const signedOut = setup();
  signedOut.setQueueSave(() => Promise.resolve(false));
  postThenReopen(signedOut);
  signedOut.posts[0].reject(Object.assign(new Error('x'), { queueable: false, status: 401 }));
  await settle();
  assert.deepEqual(signedOut.toasts, ["Your earlier report wasn't posted because you're signed out. It's kept on this page only. " +
    'Sign in again in another tab, then tap Send now on this page.']);
});

test('an earlier refusal that cannot be saved at all says so, and a serious one keeps the number (CR-01, WR-01)', async () => {
  const page = setup();
  page.setQueueSave(() => Promise.reject(new Error('not mounted')));
  postThenReopen(page);
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 429, fieldMessage: SERIOUS_LIMIT, retryAfterSeconds: 300, serious: true, emergencyNumber: '112'
  }));
  await settle();
  assert.deepEqual(page.toasts, [EARLIER_NOT_SAVED + ' In an emergency, call 112 now.']);
  assert.equal(page.toastActions[0].label, 'Call 112');
  assertNoFormClaim(page);

  const other = setup();
  other.setQueueSave(() => Promise.reject(new Error('not mounted')));
  postThenReopen(other);
  other.posts[0].reject(Object.assign(new Error('x'), { queueable: false, status: 422, fieldMessage: 'That text is not allowed.' }));
  await settle();
  assert.deepEqual(other.toasts, ["Your earlier report wasn't posted. That text is not allowed. It couldn't be saved on this device."]);
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

test('a posted report is never called refused when the closing code throws', async () => {
  let armed = false;
  const page = setup({ pickerReset: () => { if (armed) { throw new Error('map script missing'); } } });
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  armed = true;
  page.posts[0].resolve({ id: 45 });
  await settle();
  assert.ok(page.toasts.every((t) => !t.startsWith("Your earlier report wasn't posted.")),
    'a posted report must not be told as refused: ' + JSON.stringify(page.toasts));
  assert.equal(page.queueSaves.length, 0, 'a posted report must not be saved again');
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

// ----- closing the form while its post is out -----

test('Escape while the post is out closes the form without the Discard question', () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.escape();
  assert.equal(page.el('discard-confirm').hidden, true);
  assert.equal(page.el('report-backdrop').hidden, true);
  assert.equal(page.el('description').doc.activeElement, page.opener);
});

test('an edit typed after Post does not bring the question back while the post is out', () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.el('description').value = 'Water over the main road near the old bridge';
  page.el('description').input();
  page.cancelInFlight();
});

test('a Discard question open when Post is pressed goes away', () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.el('cancel-report').click();
  assert.equal(page.el('discard-confirm').hidden, false);
  page.submit();
  assert.equal(page.posts.length, 1);
  assert.equal(page.el('discard-confirm').hidden, true, 'the question must not stay over a report already sent');
  page.cancelInFlight();
});

test('after a refusal the form holds the report again, so closing asks first', async () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 422, field: 'description', fieldMessage: 'That text is not allowed.'
  }));
  await settle();
  page.el('cancel-report').click();
  assert.equal(page.el('discard-confirm').hidden, false, 'a refused report is only in the form now');
  assert.equal(page.el('report-backdrop').hidden, false);
});

test('after a wait refusal the form holds the report again, so closing asks first', async () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 429, fieldMessage: 'Please wait 5 minutes.', retryAfterSeconds: 300
  }));
  await settle();
  page.el('cancel-report').click();
  assert.equal(page.el('discard-confirm').hidden, false);
});

test('a report with no answer that cannot be saved is back in the form, so closing asks first', async () => {
  const page = setup();
  page.setQueueSave(() => Promise.reject(new Error('not valid')));
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), { queueable: true, status: 504 }));
  await settle();
  assert.equal(page.el('report-backdrop').hidden, false);
  page.el('cancel-report').click();
  assert.equal(page.el('discard-confirm').hidden, false);
});

test('a report with no answer still being saved closes without the question', async () => {
  const page = setup();
  page.setQueueSave(() => new Promise(() => {}));
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), { queueable: true, status: 504 }));
  await settle();
  assert.equal(page.queueSaves.length, 1);
  page.cancelInFlight();
});

test('Send after the wait still saving closes without the question', async () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 429, fieldMessage: 'Please wait 5 minutes.', retryAfterSeconds: 300
  }));
  await settle();
  page.setQueueSave(() => new Promise(() => {}));
  page.el('send-after-wait').click();
  assert.equal(page.queueSaves.length, 1);
  assert.equal(page.queueSaves[0].status, 'blocked');
  page.cancelInFlight();
});

test('a Discard question open when Send after the wait is pressed goes away', async () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 429, fieldMessage: 'Please wait 5 minutes.', retryAfterSeconds: 300
  }));
  await settle();
  page.el('cancel-report').click();
  assert.equal(page.el('discard-confirm').hidden, false);
  page.setQueueSave(() => new Promise(() => {}));
  page.el('send-after-wait').click();
  assert.equal(page.queueSaves.length, 1);
  assert.equal(page.el('discard-confirm').hidden, true, 'the question must not stay over a report being saved');
  page.cancelInFlight();
});

// ----- Edit and resend names the saved account (WR-02) -----

const RESEND = { requestId: 'cccccccccccccccc3333', accountId: '7', prefill: { written_at: '2026-10-16T08:00:00Z' } };
const OTHER_ACCOUNT_MESSAGE = 'This report was saved under a different account than the one signed in now. ' +
  'Sign in to that account to send it.';

test('Edit and resend posts the account the report was saved under; a fresh post does not', async () => {
  const page = setup();
  page.open(RESEND);
  page.fill('Water over the main road near the bridge');
  page.submit();
  assert.equal(page.posts[0].payload.request_id, 'cccccccccccccccc3333');
  assert.equal(page.posts[0].payload.expected_account_id, '7', 'a resend names the saved account');
  page.posts[0].resolve({ id: 50 });
  await settle();

  page.open();
  page.fill('A second report about the shelter');
  page.submit();
  assert.equal(Object.hasOwn(page.posts[1].payload, 'expected_account_id'), false, 'a fresh post names no account');

  // An id that is not digits only is never sent.
  const odd = setup();
  odd.open(Object.assign({}, RESEND, { accountId: '7 or 8' }));
  odd.fill('Water over the main road near the bridge');
  odd.submit();
  assert.equal(Object.hasOwn(odd.posts[0].payload, 'expected_account_id'), false);
});

test('a resend refused for another account keeps the text and the saved report', async () => {
  const page = setup();
  page.open(RESEND);
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 409, field: 'expected_account_id', fieldMessage: OTHER_ACCOUNT_MESSAGE
  }));
  await settle();
  assert.equal(page.el('report-backdrop').hidden, false, 'the form stays open');
  assert.equal(page.el('form-error').textContent, OTHER_ACCOUNT_MESSAGE);
  assert.equal(page.el('description').value, 'Water over the main road near the bridge');
  assert.deepEqual(page.queueDiscards, [], 'the saved report stays');
  assert.deepEqual(page.queueSaves, []);
  assert.equal(page.el('submit-report').disabled, false);
});

test('a resend with no answer is saved without the account it names', async () => {
  const page = setup();
  page.open(RESEND);
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), { queueable: true, status: 0 }));
  await settle();
  assert.equal(page.queueSaves.length, 1);
  assert.equal(Object.hasOwn(page.queueSaves[0].payload, 'expected_account_id'), false, 'the saved payload never holds it');
});

// ----- Post stays off while a report is being saved (WR-06) -----

test('Post stays off while a report with no answer is being saved', async () => {
  const page = setup();
  page.setQueueSave(() => new Promise(() => {}));
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), { queueable: true, status: 504 }));
  await settle();
  assert.equal(page.queueSaves.length, 1);
  assert.equal(page.el('submit-report').disabled, true, 'Post must stay off while the save is out');
  page.submit();
  assert.equal(page.posts.length, 1, 'no second post while the report is being saved');
});

test('a failed save after no answer gives Post back', async () => {
  const page = setup();
  page.setQueueSave(() => Promise.reject(new Error('not valid')));
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), { queueable: true, status: 504 }));
  await settle();
  assert.equal(page.el('submit-report').disabled, false);
  assert.equal(page.el('submit-report').textContent, 'Post report');
});

test('Post and Send after the wait stay off while Send after the wait saves', async () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 429, fieldMessage: 'Please wait 5 minutes.', retryAfterSeconds: 300
  }));
  await settle();
  page.setQueueSave(() => new Promise(() => {}));
  page.el('send-after-wait').click();
  assert.equal(page.queueSaves.length, 1);
  assert.equal(page.el('submit-report').disabled, true, 'Post must stay off while the save is out');
  assert.equal(page.el('send-after-wait').disabled, true, 'Send after the wait must stay off while the save is out');
  page.submit();
  page.el('send-after-wait').click();
  assert.equal(page.posts.length, 1, 'no second post while the report is being saved');
  assert.equal(page.queueSaves.length, 1, 'no second save');
});

test('a failed Send after the wait gives both buttons back, and a new form starts with them on', async () => {
  const page = setup();
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 429, fieldMessage: 'Please wait 5 minutes.', retryAfterSeconds: 300
  }));
  await settle();
  page.setQueueSave(() => Promise.reject(new Error('not valid')));
  page.el('send-after-wait').click();
  await settle();
  assert.equal(page.el('submit-report').disabled, false);
  assert.equal(page.el('send-after-wait').disabled, false);

  page.setQueueSave(() => Promise.resolve(true));
  page.el('send-after-wait').click();
  await settle();
  assert.equal(page.el('report-backdrop').hidden, true, 'the saved report closes the form');
  page.open();
  assert.equal(page.el('submit-report').disabled, false);
  assert.equal(page.el('send-after-wait').disabled, false);
});

test('a late answer leaves the question working on the form open now', async () => {
  const page = setup();
  postThenReopen(page);
  page.posts[0].resolve({ id: 46 });
  await settle();
  page.el('cancel-report').click();
  assert.equal(page.el('discard-confirm').hidden, false, 'the new draft has no post out, so closing asks first');
  assert.equal(page.el('report-backdrop').hidden, false);
});

// ----- The Post a report window (Phase 3.4, D-23, D-24, D-24a) -----

const WINDOW_ORDER = ['flood', 'traffic', 'shelter_open', 'power_outage', 'road_blocked', 'fire', 'storm_cyclone', 'other'];
const WINDOW_WORDS = ['Flood', 'Traffic', 'Shelter open', 'Power out', 'Road blocked', 'Fire', 'Storm', 'Other'];
// service.RescueNeededRetiredMessage, the server's answer to a Rescue needed post.
const RESCUE_RETIRED = 'Rescue needed is no longer a report category. Choose another one.';
const QUEUED_PREFILL = {
  severity: 'medium',
  description: 'Water rising fast near the bridge',
  latitude: 16.5,
  longitude: 80.6,
  written_at: '2026-10-16T08:00:00Z'
};

const tilesOf = (page) => page.el('category-grid').children;
const chosenTiles = (page) => tilesOf(page).filter((tile) => tile.classList.contains('category-tile--selected'));
const boxesOf = (page) => page.el('severity-group').children;
const chosenBox = (page) => boxesOf(page).filter((box) => box.getAttribute('aria-checked') === 'true');
const press = (el, key) => {
  const evt = { type: 'keydown', key, preventDefault() {} };
  el.dispatchEvent(evt);
};

test('window: eight tiles from WINDOW_TILES in order, none chosen on open', () => {
  const page = setup({ window: true });
  page.open();
  const tiles = tilesOf(page);
  assert.deepEqual(tiles.map((tile) => tile.dataset.category), WINDOW_ORDER);
  assert.deepEqual(tiles.map((tile) => tile.textContent), WINDOW_WORDS);
  assert.equal(chosenTiles(page).length, 0, 'no tile is chosen when the window opens');
  assert.equal(tiles.filter((tile) => tile.tabIndex === 0).length, 1, 'one tile is in the tab order');
  for (const retired of ['rescue_needed', 'earthquake']) {
    assert.equal(WINDOW_ORDER.includes(retired), false);
    assert.equal(tiles.some((tile) => tile.dataset.category === retired), false, retired + ' is not a tile');
  }
});

test('window: Left and Right move by one, Up and Down by four, wrapping', () => {
  const page = setup({ window: true });
  page.open();
  const at = () => chosenTiles(page)[0].dataset.category;
  tilesOf(page)[0].click();
  assert.equal(at(), 'flood');
  press(tilesOf(page)[0], 'ArrowRight');
  assert.equal(at(), 'traffic');
  press(tilesOf(page)[1], 'ArrowDown');
  assert.equal(at(), 'fire', 'four after index 1 is index 5');
  press(tilesOf(page)[5], 'ArrowDown');
  assert.equal(at(), 'traffic', 'four after index 5 wraps to index 1');
  press(tilesOf(page)[1], 'ArrowUp');
  assert.equal(at(), 'fire', 'four before index 1 wraps to index 5');
  press(tilesOf(page)[5], 'ArrowLeft');
  assert.equal(at(), 'road_blocked');
  press(tilesOf(page)[0], 'ArrowLeft');
  assert.equal(at(), 'other', 'Left from the first tile wraps to the last');
  assert.equal(chosenTiles(page).length, 1);
});

test('window: the severity boxes start on Low, move and choose with Left and Right, and the payload follows', () => {
  const page = setup({ window: true });
  page.open();
  assert.deepEqual(chosenBox(page).map((box) => box.dataset.severity), ['low']);
  assert.equal(boxesOf(page).filter((box) => box.tabIndex === 0).length, 1, 'roving tabindex');

  press(boxesOf(page)[0], 'ArrowRight');
  assert.deepEqual(chosenBox(page).map((box) => box.dataset.severity), ['medium']);
  press(boxesOf(page)[1], 'ArrowRight');
  assert.deepEqual(chosenBox(page).map((box) => box.dataset.severity), ['critical']);
  press(boxesOf(page)[2], 'ArrowRight');
  assert.deepEqual(chosenBox(page).map((box) => box.dataset.severity), ['low'], 'wraps to Low');
  press(boxesOf(page)[0], 'ArrowLeft');
  assert.deepEqual(chosenBox(page).map((box) => box.dataset.severity), ['critical'], 'wraps back to Critical');

  page.fill('Water over the main road near the bridge');
  page.submit();
  assert.equal(page.posts.length, 1);
  assert.equal(page.posts[0].payload.severity, 'critical');
  assert.equal(page.posts[0].payload.category, 'flood');
});

test('window: tapping a box chooses it, and closing and reopening starts on Low again', () => {
  const page = setup({ window: true });
  page.open();
  boxesOf(page)[1].click();
  assert.deepEqual(chosenBox(page).map((box) => box.dataset.severity), ['medium']);
  page.discard();
  page.open();
  assert.deepEqual(chosenBox(page).map((box) => box.dataset.severity), ['low']);
});

test('window: the X closes it, asking first only when something was typed', () => {
  const page = setup({ window: true });
  page.open();
  page.el('close-report').click();
  assert.equal(page.el('report-backdrop').hidden, true, 'an untouched window closes at once');

  page.open();
  page.fill('Water over the main road near the bridge');
  page.el('close-report').click();
  assert.equal(page.el('discard-confirm').hidden, false, 'a touched window asks first');
  assert.equal(page.el('report-backdrop').hidden, false);
  page.escape();
  assert.equal(page.el('discard-confirm').hidden, true, 'Escape on the question keeps editing');
  assert.equal(page.el('description').value, 'Water over the main road near the bridge');
  page.discard();
  assert.equal(page.el('report-backdrop').hidden, true);
});

test('window: Escape closes it as the X does, and a tap on the backdrop does not', () => {
  const page = setup({ window: true });
  page.open();
  page.fill('Water over the main road near the bridge');
  page.el('report-backdrop').dispatchEvent({ type: 'click', target: page.el('report-backdrop'), preventDefault() {} });
  assert.equal(page.el('discard-confirm').hidden, true, 'a tap beside the window asks nothing');
  assert.equal(page.el('report-backdrop').hidden, false, 'and does not close it');
  page.escape();
  assert.equal(page.el('discard-confirm').hidden, false, 'Escape runs the X path and asks first');
});

test('window: submitting while a post is out lets the X close without asking', () => {
  const page = setup({ window: true });
  page.open();
  page.fill('Water over the main road near the bridge');
  page.submit();
  page.cancelInFlight();
});

test('queued rescue_needed reopens with no tile', () => {
  const page = setup({ window: true });
  assert.doesNotThrow(() => {
    page.open({
      requestId: 'dddddddddddddddd4444',
      accountId: '7',
      note: 'Not sent. ' + RESCUE_RETIRED,
      prefill: Object.assign({ category: 'rescue_needed' }, QUEUED_PREFILL)
    });
  });
  assert.equal(page.el('report-backdrop').hidden, false);
  assert.equal(chosenTiles(page).length, 0, 'no tile is chosen');
  assert.equal(tilesOf(page).filter((tile) => tile.getAttribute('aria-checked') === 'true').length, 0);
  assert.deepEqual(chosenBox(page).map((box) => box.dataset.severity), ['medium'], 'the severity is restored');
  assert.equal(page.el('description').value, QUEUED_PREFILL.description, 'the text is restored');
  assert.equal(page.el('shelter-fields').hidden, true, 'no shelter fields without a shelter tile');
  assert.equal(page.el('form-note').hidden, false);
  assert.ok(page.el('form-note').textContent.includes(RESCUE_RETIRED), "the server's message shows with the form");

  // Posting it again unchanged is stopped before it leaves the form: nothing is
  // chosen, so the form asks for a tile, as for any new report.
  page.submit();
  assert.equal(page.posts.length, 0);
  assert.equal(page.el('form-error').textContent, 'Choose a category to continue.');

  // Choosing a tile posts the report under the id it was saved with.
  tilesOf(page).find((tile) => tile.dataset.category === 'flood').click();
  page.submit();
  assert.equal(page.posts.length, 1);
  assert.equal(page.posts[0].payload.request_id, 'dddddddddddddddd4444');
  assert.equal(page.posts[0].payload.category, 'flood');
  assert.equal(page.posts[0].payload.severity, 'medium');
  assert.equal(page.posts[0].payload.expected_account_id, '7');
});

test('a server 400 for the category shows its message in the window and keeps the text', async () => {
  const page = setup({ window: true });
  page.open({ requestId: 'eeeeeeeeeeeeeeee5555', prefill: Object.assign({ category: 'flood' }, QUEUED_PREFILL) });
  assert.deepEqual(chosenTiles(page).map((tile) => tile.dataset.category), ['flood']);
  page.submit();
  page.posts[0].reject(Object.assign(new Error('x'), {
    queueable: false, status: 400, field: 'category', fieldMessage: RESCUE_RETIRED
  }));
  await settle();
  assert.equal(page.el('form-error').textContent, RESCUE_RETIRED);
  assert.equal(page.el('report-backdrop').hidden, false);
  assert.equal(page.el('description').value, QUEUED_PREFILL.description);
});

test('queued earthquake reopens with no tile', () => {
  const page = setup({ window: true });
  assert.doesNotThrow(() => {
    page.open({
      requestId: 'ffffffffffffffff6666',
      prefill: Object.assign({ category: 'earthquake' }, QUEUED_PREFILL)
    });
  });
  assert.equal(chosenTiles(page).length, 0, 'Earthquake is not a tile in the window, so none is chosen');
  assert.deepEqual(chosenBox(page).map((box) => box.dataset.severity), ['medium']);
  assert.equal(page.el('description').value, QUEUED_PREFILL.description);
  page.submit();
  assert.equal(page.posts.length, 0);
  assert.equal(page.el('form-error').textContent, 'Choose a category to continue.');

  // The same entry opened in the Low data mode keeps the Earthquake tile.
  const low = setup();
  low.open({
    requestId: 'ffffffffffffffff7777',
    prefill: Object.assign({ category: 'earthquake' }, QUEUED_PREFILL)
  });
  const chosen = low.el('category-grid').children.filter((tile) => tile.classList.contains('category-tile--selected'));
  assert.deepEqual(chosen.map((tile) => tile.dataset.category), ['earthquake']);
});

test('a queued shelter report still reopens with its tile and its shelter fields in the window', () => {
  const page = setup({ window: true });
  page.open({
    requestId: 'aaaaaaaaaaaaaaaa8888',
    prefill: Object.assign({ category: 'shelter_open', shelter_capacity_status: 'limited', shelter_headcount: 40 }, QUEUED_PREFILL)
  });
  assert.deepEqual(chosenTiles(page).map((tile) => tile.dataset.category), ['shelter_open']);
  assert.equal(page.el('shelter-fields').hidden, false);
  assert.equal(page.el('shelter-headcount').value, '40');
});
