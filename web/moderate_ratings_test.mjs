// moderate_ratings_test.mjs - runs the real web/static/js/moderate.js in a Node
// sandbox against a small fake page and checks the Ratings section of the
// moderator screen, which the Go source tests cannot (Phase 9 plan 15, D-14,
// UI-SPEC P9-C30, review W12):
//
//   - Show ratings reads the typed username, draws one row per rating through
//     the text setter only, names the state of a removed or taken back rating,
//     and gives the Remove buttons to live rows only;
//   - Remove posts once, with the rater in a JSON body, and redraws the row as
//     Removed from the answer;
//   - Remove all by a rater opens the box, Cancel sends nothing, Remove all
//     posts once, and the status uses the server's count;
//   - an unknown person, an empty list and a failed read each get their line;
//   - the box stays open with its own line when the removal fails, and Escape
//     does nothing while it is being sent.
//
// Run it with: node --test web/moderate_ratings_test.mjs
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

// One selector of the forms moderate.js uses: tag, #id, .class, [attr], [attr="v"].
const SELECTOR = /^([a-z0-9]*)(?:#([\w-]+))?(?:\.([\w-]+))?(?:\[([\w-]+)(?:="([^"]*)")?\])?$/i;

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
    this.value = '';
    this.open = false;
    this.setTextCount = 0;
    this.focused = 0;
    const classes = new Set();
    this.classList = {
      add: (c) => classes.add(c),
      remove: (c) => classes.delete(c),
      contains: (c) => classes.has(c),
    };
  }
  // Only a getter: moderate.js is strict, so an assignment to textContent throws
  // and the test fails. Text must go through Pinalert.setText.
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  get firstChild() { return this.children[0] || null; }
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
  get nextSibling() {
    if (!this.parentNode) { return null; }
    const siblings = this.parentNode.children;
    return siblings[siblings.indexOf(this) + 1] || null;
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
  focus() { this.focused += 1; this.doc.activeElement = this; }
  matches(sel) {
    const m = SELECTOR.exec(sel);
    if (!m) { return false; }
    const [, tag, id, cls, attr, val] = m;
    if (tag && this.tagName !== tag.toUpperCase()) { return false; }
    if (id && this.id !== id) { return false; }
    if (cls && !this.className.split(' ').includes(cls)) { return false; }
    if (attr) {
      const have = this.getAttribute(attr);
      if (have === null || (val !== undefined && have !== val)) { return false; }
    }
    return true;
  }
  closest(sel) {
    for (let el = this; el; el = el.parentNode) { if (el.matches(sel)) { return el; } }
    return null;
  }
  querySelectorAll(sel) {
    const out = [];
    const walk = (el) => { el.children.forEach((c) => { if (c.matches(sel)) { out.push(c); } walk(c); }); };
    walk(this);
    return out;
  }
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  // Events. fire runs the handlers of this element only; click bubbles to the root.
  fire(type, extra = {}) {
    const evt = { type, target: this, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() {}, ...extra };
    (this.handlers[type] || []).slice().forEach((fn) => fn(evt));
    return evt;
  }
  click() {
    const evt = { type: 'click', target: this, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() {} };
    for (let el = this; el; el = el.parentNode) { (el.handlers.click || []).slice().forEach((fn) => fn(evt)); }
    return evt;
  }
  showModal() { this.open = true; this.shown = (this.shown || 0) + 1; }
  close() { this.open = false; this.closed = (this.closed || 0) + 1; }
}

const flush = async () => { for (let i = 0; i < 30; i += 1) { await Promise.resolve(); } };

// load builds the page, evaluates moderate.js and answers its first two reads
// (the queue and, with no comments section, nothing else).
async function load() {
  const doc = {
    activeElement: null,
    byId: {},
    created: [],
    createElement(tag) { const el = new FakeEl(doc, tag); doc.created.push(tag); return el; },
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
  ['auto', 'people', 'pending'].forEach((slug) => {
    const section = make('section', '', root);
    make('h2', 'mod-h-' + slug, section);
    make('span', 'mod-count-' + slug, section);
    make('ol', 'mod-list-' + slug, section);
    make('p', 'mod-empty-' + slug, section);
  });
  const ratings = make('section', '', root);
  make('h2', 'mod-h-ratings', ratings);
  const form = make('form', 'mod-ratings-form', ratings);
  make('label', '', form);
  const input = make('input', 'mod-ratings-name', form);
  const submit = make('button', '', form);
  submit.setAttribute('type', 'submit');
  make('p', 'mod-ratings-error', ratings).hidden = true;
  make('div', 'mod-ratings-status', ratings);
  make('ol', 'mod-ratings-list', ratings);
  const box = make('dialog', 'mod-ratings-confirm', root);
  make('h2', 'mod-ratings-confirm-title', box);
  make('p', 'mod-ratings-confirm-error', box).hidden = true;
  make('button', 'mod-ratings-confirm-cancel', box).setAttribute('data-mod-rating-action', 'box-cancel');
  make('button', 'mod-ratings-confirm-go', box).setAttribute('data-mod-rating-action', 'box-go');
  box.byRole = {};
  Object.defineProperty(box, 'title', { value: doc.byId['mod-ratings-confirm-title'] });

  const fetches = [];
  const Pinalert = {
    setText(el, text) { el.own = String(text); el.setTextCount += 1; },
    requestTimeout() { return { signal: undefined, expired: new Promise(() => {}), clear() {} }; },
    relativeTime(iso) { return 'when ' + iso; },
    writtenLine() { return ''; },
    showToast() {},
    CATEGORY_LABELS: {},
  };
  const confirms = [];
  const sandbox = {
    document: doc,
    Pinalert,
    PinalertTrust: {},
    fetch: (target, init) => new Promise((resolve) => { fetches.push({ url: target, init, resolve }); }),
    encodeURIComponent, JSON, Promise, Array, Object, String, Number, Math, Date, isFinite,
  };
  sandbox.window = sandbox;
  sandbox.confirm = (q) => { confirms.push(q); return false; };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);

  const answer = (i, status, body) => fetches[i].resolve({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  });
  // The queue read of the page load.
  assert.equal(fetches[0].url, '/api/moderation/queue');
  answer(0, 200, { review: [], flagged: [], pending: [], counts: { review: 0, flagged: 0, pending: 0 } });
  await flush();

  const ui = {
    form, input, box, root, submit,
    status: doc.byId['mod-ratings-status'],
    error: doc.byId['mod-ratings-error'],
    list: doc.byId['mod-ratings-list'],
    title: doc.byId['mod-ratings-confirm-title'],
    boxError: doc.byId['mod-ratings-confirm-error'],
    cancel: doc.byId['mod-ratings-confirm-cancel'],
    go: doc.byId['mod-ratings-confirm-go'],
  };
  // show types a username and submits the form, then answers the read.
  const show = async (name, status, body) => {
    input.value = name;
    const before = fetches.length;
    const evt = form.fire('submit');
    assert.equal(evt.defaultPrevented, true, 'the form never reloads the page');
    if (status !== undefined) {
      assert.equal(fetches.length, before + 1);
      answer(before, status, body);
      await flush();
    }
  };
  return { doc, ui, fetches, answer, show, confirms };
}

const rows = (ui) => ui.list.children;
const noteText = (ui) => ui.status.children.map((c) => c.textContent).join('|');

const THREE = {
  ratings: [
    { rater: 'asha.k', stars: 5, updated_at: '2026-10-29T10:00:00Z', removed: false, cleared: false },
    { rater: 'ravi_m', stars: 1, updated_at: '2026-10-29T09:00:00Z', removed: true, cleared: false },
    { rater: 'dev.p', stars: 3, updated_at: '2026-10-28T09:00:00Z', removed: false, cleared: true },
  ],
};

test('Show ratings draws one row per rating with text only', async () => {
  const { doc, ui, fetches, show } = await load();
  await show('rep.one', 200, THREE);
  assert.equal(fetches[1].url, '/api/moderation/people/rep.one/ratings');
  assert.equal(fetches[1].init.method, 'GET');
  assert.equal(rows(ui).length, 3);
  assert.match(rows(ui)[0].textContent, /asha\.k gave 5 stars/);
  assert.match(rows(ui)[1].textContent, /ravi_m gave 1 star(?!s)/);
  assert.match(rows(ui)[0].textContent, /when 2026-10-29T10:00:00Z/);
  assert.match(rows(ui)[1].textContent, /Removed/);
  assert.match(rows(ui)[2].textContent, /Cleared by the rater/);
  // Remove and Remove all by {rater} only on the live row.
  const live = rows(ui)[0].querySelectorAll('button');
  assert.deepEqual(live.map((b) => b.textContent), ['Remove', 'Remove all by asha.k']);
  assert.deepEqual(live.map((b) => b.getAttribute('aria-label')), ['Remove the rating by asha.k', 'Remove all by asha.k']);
  assert.equal(rows(ui)[1].querySelectorAll('button').length, 0);
  assert.equal(rows(ui)[2].querySelectorAll('button').length, 0);
  // Only text elements were made: no image, link or script.
  assert.deepEqual([...new Set(doc.created)].sort(), ['button', 'div', 'li', 'p', 'span']);
});

test('a name that looks like markup stays text', async () => {
  const { doc, ui, show } = await load();
  await show('rep.one', 200, { ratings: [{ rater: '<img src=x onerror=1>', stars: 2, updated_at: 'now', removed: false, cleared: false }] });
  assert.match(rows(ui)[0].textContent, /^<img src=x onerror=1> gave 2 stars/);
  assert.equal(doc.created.includes('img'), false);
});

test('the field is sent URL encoded and an empty field sends nothing', async () => {
  const { ui, fetches, show } = await load();
  await show('   ');
  assert.equal(fetches.length, 1);
  assert.equal(noteText(ui), "Enter the reporter's username.");
  await show(' a b/c ', 200, { ratings: [] });
  assert.equal(fetches[1].url, '/api/moderation/people/a%20b%2Fc/ratings');
});

test('an empty list, an unknown person and a failed read each get their line', async () => {
  const { ui, show } = await load();
  await show('rep.one', 200, { ratings: [] });
  assert.equal(rows(ui).length, 0);
  assert.equal(noteText(ui), 'No ratings for this person.');
  await show('nobody', 404, { error: { message: 'No such person.' } });
  assert.equal(noteText(ui), 'No such person.');
  assert.equal(ui.error.hidden, true);
  await show('rep.one', 500, null);
  assert.equal(ui.error.hidden, false);
  assert.equal(ui.error.textContent, "Couldn't load ratings. Try again.");
  assert.equal(rows(ui).length, 0);
  await show('rep.one', 200, THREE);
  assert.equal(ui.error.hidden, true, 'a good read clears the error line');
  assert.equal(rows(ui).length, 3);
});

test('Remove posts once with the rater in a JSON body and redraws the row as Removed', async () => {
  const { ui, fetches, answer, show } = await load();
  await show('rep.one', 200, THREE);
  const remove = rows(ui)[0].querySelectorAll('button')[0];
  remove.click();
  remove.click();
  assert.equal(fetches.length, 3, 'a second press while one is in flight sends nothing');
  assert.equal(fetches[2].url, '/api/moderation/people/rep.one/ratings/remove');
  assert.equal(fetches[2].init.method, 'POST');
  assert.deepEqual(JSON.parse(fetches[2].init.body), { rater: 'asha.k' });
  assert.equal(fetches[2].init.headers['Content-Type'], 'application/json');
  assert.equal(remove.getAttribute('aria-disabled'), 'true');
  assert.equal(remove.classList.contains('btn--off'), true);

  const after = { ratings: THREE.ratings.map((r) => (r.rater === 'asha.k' ? { ...r, removed: true } : r)) };
  answer(2, 200, after);
  await flush();
  assert.match(rows(ui)[0].textContent, /Removed/);
  assert.equal(rows(ui)[0].querySelectorAll('button').length, 0);
  assert.equal(noteText(ui), 'Removed the rating by asha.k.');
  assert.equal(ui.submit.getAttribute('aria-disabled'), null, 'the form is usable again');
});

test('a failed Remove keeps the row and says so', async () => {
  const { ui, fetches, answer, show } = await load();
  await show('rep.one', 200, THREE);
  rows(ui)[0].querySelectorAll('button')[0].click();
  answer(fetches.length - 1, 500, null);
  await flush();
  assert.equal(ui.error.hidden, false);
  assert.equal(ui.error.textContent, "Couldn't save that. Try again.");
  assert.equal(rows(ui)[0].querySelectorAll('button').length, 2);
});

test('Remove all by a rater asks first, Cancel sends nothing', async () => {
  const { ui, fetches, show } = await load();
  await show('rep.one', 200, THREE);
  const all = rows(ui)[0].querySelectorAll('button')[1];
  all.click();
  assert.equal(ui.box.open, true);
  assert.equal(ui.title.textContent, 'Remove all ratings by asha.k?');
  assert.equal(ui.title.focused, 1, 'the question takes the first focus');
  assert.equal(ui.cancel.focused, 0, 'no button is chosen on open');
  assert.equal(ui.go.focused, 0);
  assert.equal(fetches.length, 2, 'opening the box sends nothing');
  ui.cancel.click();
  assert.equal(ui.box.open, false);
  assert.equal(fetches.length, 2, 'Cancel sends nothing');
});

test('Remove all posts once and the status uses the server count', async () => {
  const { ui, fetches, answer, show } = await load();
  await show('rep.one', 200, THREE);
  rows(ui)[0].querySelectorAll('button')[1].click();
  ui.go.click();
  ui.go.click();
  assert.equal(fetches.length, 3, 'a second press sends nothing');
  assert.equal(fetches[2].url, '/api/moderation/raters/asha.k/remove-ratings');
  assert.equal(fetches[2].init.method, 'POST');
  assert.equal(fetches[2].init.body, undefined);
  assert.equal(ui.go.textContent, 'Removing');
  assert.equal(ui.go.getAttribute('aria-disabled'), 'true');

  // Escape while it is being sent does nothing.
  assert.equal(ui.box.fire('cancel').defaultPrevented, true);

  answer(2, 200, { removed: 4 });
  await flush();
  assert.equal(ui.box.open, false, 'the box closes when the removal is saved');
  assert.equal(fetches.length, 4, 'the list is read again');
  assert.equal(fetches[3].url, '/api/moderation/people/rep.one/ratings');
  answer(3, 200, { ratings: THREE.ratings.map((r) => ({ ...r, removed: true })) });
  await flush();
  assert.equal(noteText(ui), 'Removed 4 ratings by asha.k.', 'the count is the server\'s, not the number of rows');
  assert.equal(ui.go.textContent, 'Remove all');
  assert.equal(ui.go.getAttribute('aria-disabled'), null);
  assert.equal(ui.box.fire('cancel').defaultPrevented, false, 'Escape is the same as Cancel when idle');
});

test('one removed rating reads in the singular', async () => {
  const { ui, answer, fetches, show } = await load();
  await show('rep.one', 200, THREE);
  rows(ui)[0].querySelectorAll('button')[1].click();
  ui.go.click();
  answer(fetches.length - 1, 200, { removed: 1 });
  await flush();
  answer(fetches.length - 1, 200, THREE);
  await flush();
  assert.equal(noteText(ui), 'Removed 1 rating by asha.k.');
});

test('a failed Remove all keeps the box open with its own line', async () => {
  const { ui, fetches, answer, show } = await load();
  await show('rep.one', 200, THREE);
  rows(ui)[0].querySelectorAll('button')[1].click();
  ui.go.click();
  answer(fetches.length - 1, 500, null);
  await flush();
  assert.equal(ui.box.open, true);
  assert.equal(ui.boxError.hidden, false);
  assert.equal(ui.boxError.textContent, "Couldn't save that. Try again.");
  assert.equal(ui.go.textContent, 'Remove all');
  assert.equal(ui.go.getAttribute('aria-disabled'), null);
  assert.equal(fetches.length, 3, 'nothing else was sent');
  // Pressing Remove all again sends again, once.
  ui.go.click();
  assert.equal(fetches.length, 4);
});

test('a browser with no native box asks with confirm and sends nothing on no', async () => {
  const { ui, fetches, confirms, show } = await load();
  ui.box.showModal = undefined;
  await show('rep.one', 200, THREE);
  rows(ui)[0].querySelectorAll('button')[1].click();
  assert.deepEqual(confirms, ['Remove all ratings by asha.k?']);
  assert.equal(fetches.length, 2);
});
