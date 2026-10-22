// logout_confirm_test.mjs - runs the real web/static/js/account-menu.js in a
// Node sandbox and checks that Log out asks first (owner, 2026-10-22: "we
// should have like a confirm button before we log out. Are you sure?"). A
// Log out form's submit is held and opens the dialog; Log out in the dialog
// sends that form; Cancel or Escape send nothing and put focus back; a press
// anywhere else leaves it open; other forms pass; without dialog support the
// browser's confirm asks.
//
// Run it with: node --test web/logout_confirm_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what account-menu.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'account-menu.js'), 'utf8');

function fakeEl(id, extra = {}) {
  return {
    id,
    hidden: true,
    attrs: {},
    listeners: [],
    focused: 0,
    style: { setProperty() {} },
    addEventListener(type, fn) { this.listeners.push({ type, fn }); },
    removeEventListener(type, fn) { this.listeners = this.listeners.filter((l) => l.type !== type || l.fn !== fn); },
    setAttribute(name, value) { this.attrs[name] = value; },
    getAttribute(name) { return this.attrs[name]; },
    contains(node) { return node === this || (this.kids || []).includes(node); },
    getBoundingClientRect() { return { top: 60 }; },
    querySelectorAll() { return []; },
    focus() { this.focused += 1; },
    fire(type, e = {}) { this.listeners.filter((l) => l.type === type).forEach((l) => l.fn(e)); },
    ...extra
  };
}

function fakeForm(action, button) {
  return {
    attrs: { action },
    submitted: 0,
    getAttribute(name) { return this.attrs[name]; },
    querySelector() { return button; },
    submit() { this.submitted += 1; }
  };
}

function load({ dialogSupport = true, confirmAnswer = false } = {}) {
  const trigger = fakeEl('account-menu-trigger');
  const menuButton = fakeEl('menu-logout');
  const panel = fakeEl('account-menu', { kids: [menuButton] });
  const cancel = fakeEl('logout-confirm-cancel');
  const yes = fakeEl('logout-confirm-yes');
  const dialog = fakeEl('logout-confirm', { open: false });
  if (dialogSupport) {
    dialog.showModal = function () { this.open = true; };
    dialog.close = function () { if (this.open) { this.open = false; this.fire('close'); } };
  }
  const title = fakeEl('logout-confirm-title', { textContent: ' Log out of Pinalert? ' });
  const els = { 'account-menu-trigger': trigger, 'account-menu': panel, 'logout-confirm': dialog, 'logout-confirm-title': title, 'logout-confirm-cancel': cancel, 'logout-confirm-yes': yes };
  const docListeners = [];
  const asked = [];
  const body = { moved: [], appendChild(n) { this.moved.push(n); n.parentNode = this; } };
  const document = {
    body,
    getElementById: (id) => els[id] || null,
    addEventListener(type, fn) { docListeners.push({ type, fn }); },
    removeEventListener() {},
    activeElement: null
  };
  const window = {
    innerHeight: 568,
    addEventListener() {},
    removeEventListener() {},
    confirm(q) { asked.push(q); return confirmAnswer; }
  };
  vm.runInNewContext(source, { document, window });
  const submit = (form, submitter) => {
    const e = { target: form, submitter, prevented: false, preventDefault() { this.prevented = true; } };
    docListeners.filter((l) => l.type === 'submit').forEach((l) => l.fn(e));
    return e;
  };
  return { trigger, panel, menuButton, cancel, yes, dialog, title, submit, asked, body };
}

test('a Log out form is held and the dialog opens with the question focused', () => {
  const m = load();
  const settingsButton = fakeEl('settings-logout');
  const form = fakeForm('/auth/logout', settingsButton);
  const e = m.submit(form, settingsButton);
  assert.equal(e.prevented, true, 'the submit is held');
  assert.equal(form.submitted, 0, 'nothing is sent yet');
  assert.equal(m.dialog.open, true, 'the dialog opens');
  assert.equal(m.title.focused, 1, "the question takes focus, as the app's other windows do");
  assert.equal(m.cancel.focused, 0, 'no button looks chosen (owner, 2026-10-22)');
  assert.equal(m.yes.focused, 0);
});

test('Log out in the dialog sends the form that asked', () => {
  const m = load();
  const form = fakeForm('/auth/logout', fakeEl('b'));
  m.submit(form);
  m.yes.fire('click');
  assert.equal(form.submitted, 1);
});

test('Cancel sends nothing and puts focus back on the Settings row', () => {
  const m = load();
  const settingsButton = fakeEl('settings-logout');
  const form = fakeForm('/auth/logout', settingsButton);
  m.submit(form, settingsButton);
  m.cancel.fire('click');
  assert.equal(m.dialog.open, false);
  assert.equal(form.submitted, 0);
  assert.equal(settingsButton.focused, 1);
  m.yes.fire('click');
  assert.equal(form.submitted, 0, 'a later press on the closed dialog sends nothing');
});

test('a press outside the buttons leaves it open (owner, 2026-10-22)', () => {
  const m = load();
  const form = fakeForm('/auth/logout', m.menuButton);
  m.submit(form, m.menuButton);
  m.dialog.fire('click', { target: m.dialog });
  assert.equal(m.dialog.open, true, 'a press on the backdrop does not close it');
  m.dialog.fire('click', { target: fakeEl('logout-confirm-title') });
  assert.equal(m.dialog.open, true, 'a press on the question does not close it');
  assert.equal(form.submitted, 0, 'and sends nothing');
});

test('Escape cancels; from the menu, focus goes to the person button', () => {
  const m = load();
  const form = fakeForm('/auth/logout', m.menuButton);
  m.submit(form, m.menuButton);
  m.dialog.close();
  assert.equal(m.dialog.open, false);
  assert.equal(form.submitted, 0, 'Escape (the native close) sends nothing');
  assert.equal(m.trigger.focused, 1, 'the menu is closed, so the person button takes focus');
});

test('other forms pass untouched', () => {
  const m = load();
  const form = fakeForm('/api/me/consent', null);
  const e = m.submit(form);
  assert.equal(e.prevented, false);
  assert.equal(m.dialog.open, false);
});

test('without dialog support the browser confirm asks', () => {
  const no = load({ dialogSupport: false, confirmAnswer: false });
  const kept = fakeForm('/auth/logout', null);
  assert.equal(no.submit(kept).prevented, true);
  assert.deepEqual(no.asked, ['Log out of Pinalert?']);
  assert.equal(kept.submitted, 0, 'No keeps the person signed in');

  const yes = load({ dialogSupport: false, confirmAnswer: true });
  const sent = fakeForm('/auth/logout', null);
  yes.submit(sent);
  assert.equal(sent.submitted, 1, 'Yes logs out');
});

test('the dialog moves out of the menu to the body once, so a hidden menu cannot hide it', () => {
  const m = load();
  assert.deepEqual(m.body.moved, [m.dialog]);
  assert.equal(m.dialog.parentNode, m.body);
});
