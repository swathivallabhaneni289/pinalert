// comments_test.mjs - runs the real web/static/js/comments.js in a Node sandbox
// with a fake document, a fake fetch and a fake clock, and checks the comments
// half of the report page (Phase 8 plan 16, PAGE-02, D-02, D-12, D-13, D-15,
// UI-SPEC Surface 7):
//
//   - every word reaches the page as a text node; the only bold part is the
//     leading @username of a reply that equals the reply_to_author the server
//     sent, and the fake element throws if anything writes through a markup
//     sink (review W3, threat T-08-70);
//   - own comments show the username and no tag, others see Reporter, the
//     placeholders and the two names that are not usernames read as the
//     UI-SPEC says;
//   - replies fold behind View N replies, one level;
//   - one box serves comments and replies: Reply fills the mention only for a
//     person shown by a username, Cancel removes only the mention it inserted,
//     the counter counts code points, Enter posts and Shift with Enter adds a
//     line, and no key typed in the box reaches the map;
//   - every failure keeps the words, a closed report replaces the box with one
//     line, a held own comment shows its waiting line, Show more appends;
//   - a poll never rebuilds the list and keeps the typed words, the focus, the
//     selection and the open threads (research Pitfall 6).
//
// Run it with: node --test web/comments_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what comments.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'comments.js'), 'utf8');
// The real shared time limit of app.js (Pinalert.requestTimeout), cut out of
// the file so the sandbox runs the page's own limit on its fake timers (review
// fix WR-B01, 2026-10-29).
const appSource = fs.readFileSync(path.join(here, 'static', 'js', 'app.js'), 'utf8');
const limitAt = appSource.indexOf('  function requestTimeout(ms, what) {');
assert.notEqual(limitAt, -1, 'app.js defines requestTimeout');
const limitSource = appSource.slice(limitAt, appSource.indexOf('\n  }\n', limitAt) + 4);

class FakeText {
  constructor(value) {
    this.textContent = String(value);
    this.parentNode = null;
    this.tagName = '#text';
    this.children = [];
  }
}

// FakeEl keeps real parent semantics and bubbles events, so a test of "never
// reaches the map" means something. Writing innerHTML throws, so a markup sink
// anywhere in comments.js fails the run.
class FakeEl {
  constructor(doc, tag) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.attrs = {};
    this.handlers = {};
    this.className = '';
    this.hidden = false;
    this.value = '';
    this.style = {};
    this.scrollHeight = 0;
    this.selectionStart = 0;
    this.selectionEnd = 0;
    this.focusCalls = 0;
    this.id = '';
  }
  set innerHTML(v) { throw new Error('innerHTML written: ' + v); }
  set outerHTML(v) { throw new Error('outerHTML written: ' + v); }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  get firstChild() { return this.children[0] || null; }
  get lastChild() { return this.children[this.children.length - 1] || null; }
  get nextSibling() {
    if (!this.parentNode) { return null; }
    const sib = this.parentNode.children;
    return sib[sib.indexOf(this) + 1] || null;
  }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { delete this.attrs[n]; }
  detach(node) {
    if (node.parentNode) {
      const siblings = node.parentNode.children;
      siblings.splice(siblings.indexOf(node), 1);
      node.parentNode = null;
    }
  }
  appendChild(node) { this.detach(node); this.children.push(node); node.parentNode = this; return node; }
  insertBefore(node, ref) {
    this.detach(node);
    const at = ref ? this.children.indexOf(ref) : -1;
    if (at < 0) { this.children.push(node); } else { this.children.splice(at, 0, node); }
    node.parentNode = this;
    return node;
  }
  removeChild(node) { this.detach(node); return node; }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  focus() { this.focusCalls += 1; this.doc.activeElement = this; }
  setSelectionRange(a, b) { this.selectionStart = a; this.selectionEnd = b; }
  fire(type, evt = {}) {
    const event = {
      type, target: this, stopped: false, defaultPrevented: false,
      preventDefault() { this.defaultPrevented = true; },
      stopPropagation() { this.stopped = true; },
      ...evt
    };
    let node = this;
    while (node && !event.stopped) {
      (node.handlers[type] || []).slice().forEach((fn) => fn(event));
      node = node.parentNode;
    }
    return event;
  }
  hasClass(name) { return this.className.split(' ').includes(name); }
}

function walk(el, out = []) {
  out.push(el);
  el.children.forEach((c) => walk(c, out));
  return out;
}
function findAll(el, cls) { return walk(el).filter((n) => n.hasClass && n.hasClass(cls)); }
function find(el, cls) { return findAll(el, cls)[0] || null; }

const flush = async () => { for (let i = 0; i < 30; i += 1) { await Promise.resolve(); } };

function load({ online = true, visible = true, ignoreAbort = false } = {}) {
  const doc = {
    activeElement: null,
    visibilityState: visible ? 'visible' : 'hidden',
    createElement(tag) { return new FakeEl(doc, tag); },
    createTextNode(v) { return new FakeText(v); },
    // The section listens for the tab coming back (review fix IN-B06, 2026-10-29).
    listeners: {},
    addEventListener(type, fn) { (doc.listeners[type] = doc.listeners[type] || []).push(fn); },
    removeEventListener(type, fn) { doc.listeners[type] = (doc.listeners[type] || []).filter((f) => f !== fn); },
    dispatch(type) { (doc.listeners[type] || []).slice().forEach((fn) => fn({ type })); }
  };
  const fetches = [];
  const timers = { now: 0, nextId: 1, list: [] };
  class FakeAbortController {
    constructor() {
      const handlers = [];
      this.signal = { aborted: false, addEventListener(type, fn) { handlers.push(fn); } };
      this.fire = () => handlers.forEach((fn) => fn());
    }
    abort() { if (!this.signal.aborted) { this.signal.aborted = true; this.fire(); } }
  }
  const sandbox = {
    document: doc,
    navigator: { onLine: online },
    Pinalert: {
      setText(el, value) { el.textContent = value === null || value === undefined ? '' : String(value); },
      relativeTime(iso) { return 'at ' + iso; },
      severityClass(r) { return 'sev-' + ((r && r.severity) || 'low'); },
      iconClass(c) { return 'icon-glyph icon-glyph--' + c; }
    },
    AbortController: FakeAbortController,
    fetch(address, opts) {
      return new Promise((resolve, reject) => {
        const call = {
          address, opts, settled: false,
          body: opts.body ? JSON.parse(opts.body) : null,
          reply(status, body) {
            call.settled = true;
            resolve({ ok: status >= 200 && status < 300, status, json: () => Promise.resolve(body) });
          },
          fail(err) { call.settled = true; reject(err || new Error('network')); },
          // stall answers with the headers and a body read that never ends.
          stall(status) {
            call.settled = true;
            resolve({ ok: status >= 200 && status < 300, status, json: () => new Promise(() => {}) });
          }
        };
        // ignoreAbort is the WebKit case: an aborted request whose promise
        // never settles (app.js requestTimeout).
        if (!ignoreAbort && opts.signal) {
          opts.signal.addEventListener('abort', () => {
            if (!call.settled) { call.settled = true; reject(new Error('aborted')); }
          });
        }
        fetches.push(call);
      });
    },
    setTimeout(fn, ms) { const id = timers.nextId++; timers.list.push({ id, fn, at: timers.now + ms, every: 0 }); return id; },
    setInterval(fn, ms) { const id = timers.nextId++; timers.list.push({ id, fn, at: timers.now + ms, every: ms }); return id; },
    clearTimeout(id) { timers.list = timers.list.filter((t) => t.id !== id); },
    clearInterval(id) { timers.list = timers.list.filter((t) => t.id !== id); },
    console
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(limitSource, sandbox);
  sandbox.Pinalert.requestTimeout = sandbox.requestTimeout;
  vm.runInContext(source, sandbox, { filename: 'comments.js' });
  return {
    doc, fetches, sandbox, timers,
    Comments: sandbox.PinalertComments,
    advance(ms) {
      const to = timers.now + ms;
      for (;;) {
        const due = timers.list.filter((t) => t.at <= to).sort((a, b) => a.at - b.at)[0];
        if (!due) { break; }
        timers.now = due.at;
        if (due.every) { due.at += due.every; } else { timers.list = timers.list.filter((t) => t !== due); }
        due.fn();
      }
      timers.now = to;
    },
    intervals() { return timers.list.filter((t) => t.every > 0).length; }
  };
}

function report(over = {}) {
  return { id: 'r1', category: 'flood', severity: 'critical', created_at: '2026-10-29T08:00:00Z', comments_open: true, comment_count: 0, ...over };
}

function cm(id, over = {}) {
  return {
    id, author: 'ravi.k', author_kind: 'user', is_own: false, is_reporter: false, can_delete: false,
    viewer_liked: false, viewer_flagged: false, likes: 0, reply_count: 0, at: '2026-10-29T08:00:00Z',
    deleted: false, removed: false, held: false, words: 'The road is open', replies: [], ...over
  };
}

function body(comments, over = {}) {
  const count = comments.reduce((n, c) => n + 1 + (c.replies ? c.replies.length : 0), 0);
  return { comments, comment_count: count, comments_open: true, more: false, next_after: 0, ...over };
}

// start builds the section, opens it and answers the first list with comments.
async function start(env, comments, over = {}, rep = report()) {
  const mount = new FakeEl(env.doc, 'div');
  const view = env.Comments.create(rep, mount);
  view.open();
  assert.equal(env.fetches.length >= 1, true, 'open fetches the list');
  env.fetches[env.fetches.length - 1].reply(200, body(comments, over));
  await flush();
  const form = find(view.el, 'composer');
  return {
    view, mount, form,
    input: find(view.el, 'composer__input'),
    post: find(view.el, 'composer__post'),
    counter: find(view.el, 'composer__counter'),
    status: find(view.el, 'composer__status'),
    error: find(view.el, 'composer__error'),
    replying: find(view.el, 'composer__replying'),
    cancel: find(view.el, 'composer__cancel'),
    list: find(view.el, 'comments__list'),
    items: () => findAll(view.el, 'comment').filter((n) => n.tagName === 'LI' && !n.hasClass('comment--reply'))
  };
}

function type(page, value) {
  page.input.value = value;
  page.input.fire('input');
}

function wordsOf(li) { return find(li, 'comment__words'); }
function replyButton(li) { return find(li, 'comment__reply'); }
function lastFetch(env) { return env.fetches[env.fetches.length - 1]; }
function posts(env) { return env.fetches.filter((f) => f.opts.method === 'POST'); }

test('build and open', async () => {
  const env = load();
  const mount = new FakeEl(env.doc, 'div');
  const view = env.Comments.create(report(), mount);
  assert.equal(mount.children.length, 1, 'built into the mount, once');
  assert.equal(view.el.tagName, 'SECTION');
  assert.ok(view.el.hasClass('comments'));
  assert.equal(env.fetches.length, 0, 'building fetches nothing');
  for (const name of ['update', 'open', 'close', 'refresh']) { assert.equal(typeof view[name], 'function', name); }
  const heading = find(view.el, 'comments__heading');
  assert.equal(heading.tagName, 'H2');
  assert.equal(heading.getAttribute('tabindex'), '-1');
  assert.equal(heading.textContent, 'Comments');
  const input = find(view.el, 'composer__input');
  assert.equal(input.getAttribute('maxlength'), null, 'no length attribute: it counts UTF-16 units (review W2)');
  assert.equal(input.getAttribute('placeholder'), 'Add a comment…');
  assert.equal(input.getAttribute('aria-label'), 'Add a comment');
  assert.equal(input.getAttribute('rows'), '1');
  assert.equal(input.getAttribute('enterkeyhint'), 'send');
});

test('skeleton while loading then the list', async () => {
  const env = load();
  const mount = new FakeEl(env.doc, 'div');
  const view = env.Comments.create(report(), mount);
  view.open();
  const list = find(view.el, 'comments__list');
  assert.equal(findAll(list, 'comment--skeleton').length, 3);
  assert.equal(list.getAttribute('aria-busy'), 'true');
  assert.equal(find(view.el, 'composer').hidden, true, 'no box before the list is known');
  assert.match(lastFetch(env).address, /^\/api\/reports\/r1\/comments$/);
  lastFetch(env).reply(200, body([cm(1)]));
  await flush();
  assert.equal(findAll(list, 'comment--skeleton').length, 0);
  assert.equal(list.getAttribute('aria-busy'), 'false');
  assert.equal(findAll(list, 'comment').length, 1);
  assert.equal(find(view.el, 'composer').hidden, false);
});

test('empty list and the heading count', async () => {
  const env = load();
  const page = await start(env, []);
  assert.equal(find(page.view.el, 'comments__empty').hidden, false);
  assert.equal(find(page.view.el, 'comments__empty').textContent, 'No comments yet.');
  assert.equal(page.form.hidden, false, 'the box follows the empty line');
  assert.equal(find(page.view.el, 'comments__count').hidden, true, 'the count is omitted at zero');
  const env2 = load();
  const page2 = await start(env2, [cm(1, { replies: [cm(2)] }), cm(3)]);
  const count = find(page2.view.el, 'comments__count');
  assert.equal(count.hidden, false);
  assert.equal(count.textContent, '3', 'replies are counted');
  assert.equal(find(page2.view.el, 'comments__empty').hidden, true);
});

test('load failure and offline', async () => {
  const env = load();
  const mount = new FakeEl(env.doc, 'div');
  const view = env.Comments.create(report(), mount);
  view.open();
  lastFetch(env).fail();
  await flush();
  const block = find(view.el, 'comments__error');
  assert.equal(block.hidden, false);
  assert.equal(find(view.el, 'comments__error-title').textContent, "Couldn't load comments");
  assert.equal(find(view.el, 'comments__error-line').textContent, 'Check your connection and try again.');
  assert.equal(find(view.el, 'comments__retry').textContent, 'Try again');
  const before = env.fetches.length;
  find(view.el, 'comments__retry').fire('click');
  assert.equal(env.fetches.length, before + 1, 'Try again asks again');
  assert.equal(env.doc.activeElement, find(view.el, 'comments__heading'), 'focus goes to the heading');
  lastFetch(env).reply(200, body([cm(1)]));
  await flush();
  assert.equal(block.hidden, true);

  const offline = load({ online: false });
  const m2 = new FakeEl(offline.doc, 'div');
  const v2 = offline.Comments.create(report(), m2);
  v2.open();
  assert.equal(offline.fetches.length, 0, 'offline asks nothing');
  assert.equal(find(v2.el, 'comments__error-line').textContent, "You're offline, so comments can't load. Try again when you have a signal.");
});

test('text only and a strong mention', async () => {
  const env = load();
  const page = await start(env, [
    cm(1, { author: 'ndma.fan', words: '@ndma Evacuate now' }),
    cm(2, { author: 'ravi.k', words: 'Water is rising', replies: [
      cm(3, { author: 'priya.v', words: '@ravi.k yes', reply_to_author: 'ravi.k' }),
      cm(4, { author: 'priya.v', words: '@ndma yes', reply_to_author: 'ravi.k' }),
      cm(5, { author: 'priya.v', words: '@ravi.k no reply_to_author sent' }),
      cm(6, { author: 'priya.v', words: '@ravi.kk typo', reply_to_author: 'ravi.k' }),
      cm(7, { author: 'priya.v', words: '<b>bold</b> & <img src=x onerror=alert(1)>' })
    ] })
  ]);
  const top = wordsOf(page.items()[0]);
  assert.equal(top.textContent, '@ndma Evacuate now');
  assert.equal(top.children.length, 0, 'a top level @word is one plain text node, never a mention');
  const replies = findAll(page.items()[1], 'comment--reply');
  assert.equal(replies.length, 5);
  const mention = wordsOf(replies[0]);
  assert.equal(mention.children.length, 2);
  assert.equal(mention.children[0].tagName, 'STRONG');
  assert.equal(mention.children[0].textContent, '@ravi.k');
  assert.equal(mention.children[1].tagName, '#text');
  assert.equal(mention.children[1].textContent, ' yes');
  assert.equal(mention.textContent, '@ravi.k yes');
  for (const i of [1, 2, 3]) {
    const plain = wordsOf(replies[i]);
    assert.equal(plain.children.length, 0, 'reply ' + i + ' keeps its words plain');
  }
  assert.equal(wordsOf(replies[3]).textContent, '@ravi.kk typo');
  const angle = wordsOf(replies[4]);
  assert.equal(angle.textContent, '<b>bold</b> & <img src=x onerror=alert(1)>');
  assert.equal(angle.children.length, 0, 'angle brackets are literal text');
  const markup = walk(page.view.el).filter((n) => ['B', 'IMG', 'A'].includes(n.tagName));
  assert.equal(markup.length, 0);
});

test('own comment shows the username and no tag', async () => {
  const env = load();
  const page = await start(env, [
    cm(1, { author: 'priya.vja', is_own: true, can_delete: true, words: 'Mine' }),
    cm(2, { author: 'ravi.k', is_reporter: true, words: 'I posted this' }),
    cm(3, { author: 'meena.s', words: 'Plain' })
  ]);
  const [own, reporter, plain] = page.items();
  assert.equal(find(own, 'comment__author').textContent, 'priya.vja');
  assert.notEqual(find(own, 'comment__author').textContent, 'You');
  assert.equal(find(own, 'comment__tag').hidden, true, 'no Reporter tag on your own comment');
  assert.equal(find(reporter, 'comment__tag').hidden, false);
  assert.equal(find(reporter, 'comment__tag').textContent, 'Reporter');
  const disc = find(reporter, 'comment__disc');
  assert.ok(disc.hasClass('icon-badge'), 'the reporter disc is the shipped badge');
  assert.ok(disc.hasClass('sev-critical'));
  assert.ok(find(disc, 'icon-glyph--flood'), 'the report category glyph');
  assert.equal(disc.getAttribute('aria-hidden'), 'true');
  assert.equal(find(plain, 'comment__tag').hidden, true);
  const letter = find(plain, 'comment__disc');
  assert.equal(letter.textContent, 'M', 'a letter disc: the first character, upper case');
  assert.equal(letter.hasClass('icon-badge'), false);
  assert.equal(find(own, 'comment__time--head').getAttribute('datetime'), '2026-10-29T08:00:00Z');
  assert.equal(find(own, 'comment__time--meta').textContent, 'at 2026-10-29T08:00:00Z');
  assert.equal(find(page.view.el, 'comment__disc--own').textContent, 'P', 'the box shows your own letter once your name is known');
});

test('placeholders and names', async () => {
  const env = load();
  const page = await start(env, [
    cm(1, { deleted: true, author: '', author_kind: '', words: '', replies: [cm(5, { words: 'still here' })] }),
    cm(2, { removed: true, author: '', author_kind: '', words: '' }),
    cm(3, { author: 'New member', author_kind: 'new_member' }),
    cm(4, { author: 'Deleted account', author_kind: 'deleted_account' })
  ]);
  const items = page.items();
  const gone = find(items[0], 'comment__gone');
  assert.equal(gone.hidden, false);
  assert.equal(gone.textContent, 'Comment deleted');
  assert.equal(find(items[0], 'comment__meta').hidden, true, 'no Reply on a placeholder');
  assert.equal(find(items[0], 'comment__head').hidden, true);
  assert.ok(find(items[0], 'comment__disc--empty'), 'the empty neutral disc');
  assert.equal(find(items[1], 'comment__gone').textContent, 'Comment removed');
  assert.equal(find(items[0], 'comment-thread__toggle').textContent, 'View 1 reply', 'its replies stay');
  assert.equal(find(items[2], 'comment__author').textContent, 'New member');
  assert.ok(find(items[2], 'comment__disc--glyph'), 'the user glyph disc');
  assert.equal(find(items[3], 'comment__author').textContent, 'Deleted account');
  assert.ok(find(items[3], 'comment__disc--glyph'));
});

// Added 2026-10-29 by Phase 9 plan 10 (D-12, UI-SPEC P9-C31): the placeholder of
// a comment written by someone the viewer blocked and that others answered.
test('hidden placeholder', async () => {
  const env = load();
  const page = await start(env, [
    cm(1, {
      hidden: true, author: '', author_kind: '', words: '', likes: 0,
      replies: [cm(5, { words: 'still here' })]
    })
  ]);
  const item = page.items()[0];
  const gone = find(item, 'comment__gone');
  assert.equal(gone.hidden, false);
  assert.equal(gone.textContent, 'Comment hidden');
  assert.equal(find(item, 'comment__head').hidden, true, 'no author name or Reporter tag');
  assert.equal(find(item, 'comment__words').hidden, true, 'no words');
  assert.equal(find(item, 'comment__meta').hidden, true, 'no Reply, Like or Flag on a hidden placeholder');
  assert.ok(find(item, 'comment__disc--empty'), 'the empty neutral disc, like Comment deleted');
  assert.equal(find(item, 'comment-thread__toggle').textContent, 'View 1 reply', 'its replies stay');
});

test('folded replies', async () => {
  const env = load();
  const page = await start(env, [
    cm(1, { replies: [cm(2, { words: 'a' }), cm(3, { words: 'b' }), cm(4, { words: 'c' })] }),
    cm(5, { replies: [cm(6)] }),
    cm(7)
  ]);
  const [a, b, c] = page.items().filter((li) => !li.hasClass('comment--reply'));
  const toggle = find(a, 'comment-thread__toggle');
  const thread = find(a, 'comment-thread');
  assert.equal(toggle.textContent, 'View 3 replies');
  assert.equal(toggle.getAttribute('aria-expanded'), 'false');
  assert.equal(thread.hidden, true);
  assert.equal(toggle.getAttribute('aria-controls'), thread.id);
  assert.ok(thread.id !== '');
  toggle.fire('click');
  assert.equal(toggle.textContent, 'Hide replies');
  assert.equal(toggle.getAttribute('aria-expanded'), 'true');
  assert.equal(thread.hidden, false);
  assert.equal(findAll(thread, 'comment').length, 3);
  assert.equal(find(b, 'comment-thread__toggle').textContent, 'View 1 reply');
  assert.equal(find(c, 'comment__replies').hidden, true, 'no replies, no toggle');
  toggle.fire('click');
  assert.equal(toggle.textContent, 'View 3 replies');
  assert.equal(thread.hidden, true);
});

test('reply fills the box', async () => {
  const env = load();
  const page = await start(env, [
    cm(1, { author: 'ravi.k' }),
    cm(2, { author: 'priya.vja', is_own: true }),
    cm(3, { author: 'New member', author_kind: 'new_member' }),
    cm(4, { author: 'Deleted account', author_kind: 'deleted_account' })
  ]);
  const [other, own, member, deleted] = page.items();
  replyButton(other).fire('click');
  assert.equal(page.input.value, '@ravi.k ');
  assert.equal(page.replying.hidden, false);
  assert.equal(find(page.replying, 'composer__replying-name').textContent, 'ravi.k');
  assert.equal(page.replying.textContent, 'Replying to ravi.k');
  assert.equal(page.input.getAttribute('aria-label'), 'Reply to ravi.k');
  assert.equal(env.doc.activeElement, page.input, 'focus moves to the box');
  assert.equal(page.input.selectionStart, 8);
  assert.equal(page.post.getAttribute('aria-disabled'), 'false');
  for (const [li, shown] of [[own, 'priya.vja'], [member, 'New member'], [deleted, 'Deleted account']]) {
    replyButton(li).fire('click');
    assert.equal(page.input.value, '', 'no mention for ' + shown + ' (review W3)');
    assert.equal(find(page.replying, 'composer__replying-name').textContent, shown);
  }
  assert.equal(page.replying.hidden, false);
});

test('cancel removes only the inserted mention', async () => {
  const env = load();
  const page = await start(env, [cm(1, { author: 'ravi.k' })]);
  const [li] = page.items();
  replyButton(li).fire('click');
  page.cancel.fire('click');
  assert.equal(page.input.value, '', 'the untouched mention goes');
  assert.equal(page.replying.hidden, true);
  assert.equal(page.input.getAttribute('aria-label'), 'Add a comment');
  assert.equal(env.doc.activeElement, page.input);
  replyButton(li).fire('click');
  type(page, '@ravi.k thanks for this');
  page.cancel.fire('click');
  assert.equal(page.input.value, '@ravi.k thanks for this', 'typed words stay');
  assert.equal(page.replying.hidden, true);
  replyButton(li).fire('click');
  assert.equal(page.input.value, '@ravi.k thanks for this', 'a second Reply does not stack the mention');
  type(page, '@ravi.k ');
  const esc = page.input.fire('keydown', { key: 'Escape' });
  assert.equal(page.replying.hidden, true, 'Escape cancels the target');
  assert.equal(page.input.value, '');
  assert.equal(esc.defaultPrevented, true);
});

test('counter counts code points', async () => {
  const env = load();
  const page = await start(env, []);
  type(page, 'a'.repeat(239));
  assert.equal(page.counter.hidden, true, 'hidden above 40 left');
  type(page, 'a'.repeat(240));
  assert.equal(page.counter.hidden, false);
  assert.equal(page.counter.textContent, '40 left');
  assert.equal(page.status.textContent, '40 left', 'spoken when it first shows');
  type(page, '\u{1F600}'.repeat(240));
  assert.equal(page.input.value.length, 480, 'the emoji are 2 UTF-16 units each');
  assert.equal(page.counter.textContent, '40 left', 'a 4 byte emoji counts as one');
  type(page, 'తెలుగు'.repeat(40));
  assert.equal(page.counter.textContent, '40 left', 'a Telugu word counts its code points');
  page.status.textContent = 'marker';
  type(page, 'a'.repeat(241));
  assert.equal(page.counter.textContent, '39 left');
  assert.equal(page.status.textContent, 'marker', 'not spoken on every key');
  type(page, 'a'.repeat(270));
  assert.equal(page.status.textContent, '10 left');
  page.status.textContent = 'marker';
  type(page, 'a'.repeat(271));
  assert.equal(page.status.textContent, 'marker');
  type(page, 'a'.repeat(280));
  assert.equal(page.status.textContent, '0 left');
  type(page, '');
  assert.equal(page.counter.hidden, true);
});

test('enter posts and shift enter adds a line', async () => {
  const env = load();
  const page = await start(env, []);
  assert.equal(page.post.getAttribute('aria-disabled'), 'true', 'off with nothing typed');
  page.post.fire('click');
  assert.equal(posts(env).length, 0, 'a press on an off Post does nothing');
  type(page, 'Road open now');
  assert.equal(page.post.getAttribute('aria-disabled'), 'false');
  const shift = page.input.fire('keydown', { key: 'Enter', shiftKey: true });
  assert.equal(shift.defaultPrevented, false, 'Shift with Enter adds a line');
  assert.equal(posts(env).length, 0);
  const enter = page.input.fire('keydown', { key: 'Enter' });
  assert.equal(enter.defaultPrevented, true);
  assert.equal(posts(env).length, 1);
  const call = posts(env)[0];
  assert.equal(call.address, '/api/reports/r1/comments');
  assert.deepEqual(call.body, { words: 'Road open now' });
  assert.equal(call.opts.credentials, 'same-origin');
  assert.equal(page.post.textContent, 'Posting…');
  assert.equal(page.post.getAttribute('aria-disabled'), 'true', 'off while posting');
  page.input.fire('keydown', { key: 'Enter' });
  assert.equal(posts(env).length, 1, 'no second post while posting');
  call.reply(201, cm(9, { author: 'priya.vja', is_own: true, words: 'Road open now' }));
  await flush();
  assert.equal(page.input.value, '', 'the box clears');
  assert.equal(page.status.textContent, 'Comment posted.');
  assert.equal(page.post.textContent, 'Post');
  assert.equal(env.doc.activeElement, page.input, 'focus stays in the box');
  assert.equal(page.items().length, 1, 'the comment appears in place');
  assert.equal(find(page.view.el, 'comments__count').textContent, '1');
  assert.equal(page.error.hidden, true);
});

test('a reply posts to the comment and opens its thread', async () => {
  const env = load();
  const page = await start(env, [cm(1, { author: 'ravi.k' })]);
  const [li] = page.items();
  replyButton(li).fire('click');
  page.input.value = '@ravi.k yes it is';
  page.input.fire('input');
  page.input.fire('keydown', { key: 'Enter' });
  const call = posts(env)[0];
  assert.deepEqual(call.body, { words: '@ravi.k yes it is', reply_to: 1 });
  call.reply(201, cm(10, { author: 'priya.vja', is_own: true, words: '@ravi.k yes it is', reply_to_author: 'ravi.k' }));
  await flush();
  assert.equal(page.replying.hidden, true, 'the reply target clears');
  const toggle = find(li, 'comment-thread__toggle');
  assert.equal(toggle.getAttribute('aria-expanded'), 'true', 'posting a reply opens its thread');
  assert.equal(toggle.textContent, 'Hide replies');
  const reply = findAll(li, 'comment--reply')[0];
  assert.equal(wordsOf(reply).children[0].tagName, 'STRONG');
});

test('escape never reaches the map', async () => {
  const env = load();
  const map = new FakeEl(env.doc, 'div');
  let seen = 0;
  for (const type of ['keydown', 'keypress', 'keyup']) { map.addEventListener(type, () => { seen += 1; }); }
  const mount = new FakeEl(env.doc, 'div');
  map.appendChild(mount);
  const view = env.Comments.create(report(), mount);
  view.open();
  lastFetch(env).reply(200, body([cm(1)]));
  await flush();
  const input = find(view.el, 'composer__input');
  const keys = ['Escape', 'Enter', 'ArrowUp', 'ArrowLeft', '+', '-', ' ', 'a'];
  for (const key of keys) {
    for (const type of ['keydown', 'keypress', 'keyup']) { input.fire(type, { key }); }
  }
  assert.equal(seen, 0, 'no key typed in the box reaches the map');
  const cancel = find(view.el, 'composer__cancel');
  cancel.fire('keydown', { key: 'Enter' });
  assert.equal(seen, 0, 'nor a key on the box buttons');
  const reply = find(view.el, 'comment__reply');
  reply.fire('keydown', { key: 'Enter' });
  reply.fire('keydown', { key: ' ' });
  assert.equal(seen, 0, 'Enter and Space on a control act on the control');
  const outside = reply.fire('keydown', { key: 'Escape' });
  assert.equal(outside.stopped, false, 'Escape outside the box still closes the report');
  assert.equal(seen, 1);
});

test('rejected keeps the words', async () => {
  const env = load();
  const page = await start(env, []);
  type(page, 'a hateful sentence');
  page.input.fire('keydown', { key: 'Enter' });
  posts(env)[0].reply(422, { error: { field: 'words', message: "This text looks hateful, so it can't be posted. Reword it and try again." } });
  await flush();
  assert.equal(page.input.value, 'a hateful sentence');
  assert.equal(page.error.hidden, false);
  assert.equal(page.error.textContent, "This text looks hateful, so it can't be posted. Reword it and try again.");
  assert.equal(page.error.getAttribute('role'), 'alert');
  assert.ok(page.error.hasClass('inline-error'));
  assert.equal(page.post.textContent, 'Post');
  assert.equal(page.items().length, 0);
});

test('rate limited keeps the words', async () => {
  const env = load();
  const page = await start(env, []);
  type(page, 'one more thing');
  page.input.fire('keydown', { key: 'Enter' });
  const sentence = "You're commenting very quickly. You can comment again in 20 seconds. Your words are still here.";
  posts(env)[0].reply(429, { error: { field: 'rate_comment', message: sentence, retry_after_seconds: 20 } });
  await flush();
  assert.equal(page.input.value, 'one more thing');
  assert.equal(page.error.textContent, sentence);
  assert.equal(page.error.hidden, false);
  page.input.fire('keydown', { key: 'Enter' });
  assert.equal(page.error.hidden, true, 'the sentence clears when the person tries again');
});

test('offline keeps the words', async () => {
  const env = load();
  const page = await start(env, []);
  type(page, 'no signal here');
  env.sandbox.navigator.onLine = false;
  page.input.fire('keydown', { key: 'Enter' });
  assert.equal(posts(env).length, 0, 'comments are never queued');
  assert.equal(page.input.value, 'no signal here');
  assert.equal(page.error.textContent, "You're offline, so this comment wasn't posted. Your words are still here. Try again when you have a signal.");
  env.sandbox.navigator.onLine = true;
  page.input.fire('keydown', { key: 'Enter' });
  posts(env)[0].fail();
  await flush();
  assert.equal(page.input.value, 'no signal here');
  assert.equal(page.error.textContent, "Couldn't post your comment. Try again.");
});

test('other failures keep the words', async () => {
  const env = load();
  const page = await start(env, []);
  type(page, 'words to keep');
  page.input.fire('keydown', { key: 'Enter' });
  posts(env)[0].reply(500, { error: { field: 'body', message: 'internal' } });
  await flush();
  assert.equal(page.input.value, 'words to keep');
  assert.equal(page.error.textContent, "Couldn't post your comment. Try again.");
  page.input.fire('keydown', { key: 'Enter' });
  posts(env)[1].reply(409, { error: { field: 'username', message: 'Choose a username in Settings first.' } });
  await flush();
  assert.equal(page.input.value, 'words to keep');
  assert.equal(page.error.textContent, 'Choose a username in Settings first.');
  page.input.fire('keydown', { key: 'Enter' });
  posts(env)[2].reply(400, { error: { field: 'words', message: 'A comment can be up to 280 letters.' } });
  await flush();
  assert.equal(page.error.textContent, 'A comment can be up to 280 letters.');
  assert.equal(page.input.value, 'words to keep');
});

test('closed replaces the box', async () => {
  const env = load();
  const page = await start(env, [cm(1, { author: 'ravi.k' })]);
  type(page, 'typed before it closed');
  page.input.fire('keydown', { key: 'Enter' });
  posts(env)[0].reply(409, { error: { field: 'comments', message: 'Comments are closed' } });
  await flush();
  const closed = find(page.view.el, 'comments__closed');
  assert.equal(closed.hidden, false);
  assert.equal(closed.textContent, 'Comments are closed');
  assert.equal(page.form.hidden, true, 'the box is replaced');
  assert.equal(replyButton(page.items()[0]).hidden, true, 'Reply is not drawn');
  assert.equal(page.input.value, '', 'the typed words are dropped with the box');

  const env2 = load();
  const page2 = await start(env2, [cm(1)], { comments_open: false });
  assert.equal(page2.form.hidden, true);
  assert.equal(find(page2.view.el, 'comments__closed').hidden, false);
  assert.equal(replyButton(page2.items()[0]).hidden, true);
  const env3 = load();
  const page3 = await start(env3, [], { comments_open: false });
  assert.equal(find(page3.view.el, 'comments__empty').hidden, true, 'an empty closed list shows only the line');
  assert.equal(find(page3.view.el, 'comments__closed').hidden, false);
  assert.equal(page3.form.hidden, true);
});

test('held own comment', async () => {
  const env = load();
  const page = await start(env, [cm(1, { author: 'priya.vja', is_own: true, can_delete: true, held: true, words: 'Pending words' })], { comment_count: 0 });
  const [li] = page.items();
  const held = find(li, 'comment__held');
  assert.equal(held.hidden, false);
  assert.equal(held.textContent, 'Waiting for a text check. Only you can see it until then.');
  assert.equal(replyButton(li).hidden, true, 'the line takes the place of Reply');
  assert.equal(find(page.view.el, 'comments__count').hidden, true, 'a held comment is not counted');
  type(page, 'another');
  page.input.fire('keydown', { key: 'Enter' });
  posts(env)[0].reply(201, cm(2, { author: 'priya.vja', is_own: true, held: true, words: 'another' }));
  await flush();
  assert.equal(page.items().length, 2);
  assert.equal(find(page.items()[1], 'comment__held').hidden, false);
  assert.equal(find(page.view.el, 'comments__count').hidden, true);
});

test('show more', async () => {
  const env = load();
  const page = await start(env, [cm(1), cm(2)], { more: true, next_after: 2 });
  const more = find(page.view.el, 'comments__more');
  assert.equal(more.hidden, false);
  assert.equal(more.textContent, 'Show more comments');
  more.fire('click');
  assert.equal(lastFetch(env).address, '/api/reports/r1/comments?after=2');
  lastFetch(env).fail();
  await flush();
  const err = find(page.view.el, 'comments__more-error');
  assert.equal(err.hidden, false);
  assert.equal(err.textContent, "Couldn't load more comments. Try again.");
  assert.ok(err.hasClass('inline-error'));
  more.fire('click');
  assert.equal(err.hidden, true);
  lastFetch(env).reply(200, body([cm(3), cm(4)], { more: false, next_after: 0, comment_count: 4 }));
  await flush();
  assert.equal(page.items().length, 4, 'the new ones are appended');
  assert.equal(more.hidden, true);
  assert.equal(find(page.view.el, 'comments__count').textContent, '4');
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(1), cm(2)], { more: true, next_after: 2, comment_count: 4 }));
  await flush();
  assert.equal(page.items().length, 4, 'a poll never drops what Show more loaded');
  assert.equal(more.hidden, true, 'and does not reset the cursor');
});

test('poll keeps text, focus and open thread', async () => {
  const env = load();
  const page = await start(env, [
    cm(1, { replies: [cm(2, { words: 'first reply' })] }),
    cm(3, { author: 'meena.s' })
  ]);
  const before = page.items();
  const toggle = find(before[0], 'comment-thread__toggle');
  toggle.fire('click');
  type(page, 'half typed words');
  page.input.focus();
  page.input.setSelectionRange(5, 9);
  const focusCalls = page.input.focusCalls;
  const listChildren = page.list.children.slice();
  env.advance(30000);
  assert.equal(lastFetch(env).address, '/api/reports/r1/comments');
  lastFetch(env).reply(200, body([
    cm(1, { replies: [cm(2, { words: 'first reply' }), cm(8, { words: 'second reply' })] }),
    cm(3, { author: 'meena.s' }),
    cm(9, { author: 'late.one', words: 'new from someone' })
  ]));
  await flush();
  assert.equal(page.input.value, 'half typed words');
  assert.equal(env.doc.activeElement, page.input);
  assert.equal(page.input.focusCalls, focusCalls, 'a poll never calls focus');
  assert.equal(page.input.selectionStart, 5);
  assert.equal(page.input.selectionEnd, 9);
  assert.equal(page.form, find(page.view.el, 'composer'), 'the box is the same node');
  assert.equal(page.list.children[0], listChildren[0], 'the same comment nodes');
  assert.equal(page.list.children[1], listChildren[1]);
  assert.equal(page.list.children.length, 3, 'the new comment arrives in place');
  assert.equal(toggle.getAttribute('aria-expanded'), 'true', 'the open thread stays open');
  assert.equal(findAll(before[0], 'comment--reply').length, 2);
  assert.equal(page.list.children[2].children[0].children[1].children[0].children[0].textContent, 'late.one');
});

test('a poll drops a comment that no longer exists', async () => {
  const env = load();
  const page = await start(env, [cm(1, { author: 'priya.vja', is_own: true, held: true }), cm(2)]);
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(2)]));
  await flush();
  assert.equal(page.items().length, 1, 'a held comment that failed its check disappears');
});

test('open and close drive the poll', async () => {
  const env = load();
  const mount = new FakeEl(env.doc, 'div');
  const view = env.Comments.create(report(), mount);
  view.open();
  assert.equal(env.intervals(), 1);
  view.open();
  assert.equal(env.intervals(), 1, 'never a second interval');
  lastFetch(env).reply(200, body([cm(1)]));
  await flush();
  view.close();
  assert.equal(env.intervals(), 0);
  const count = env.fetches.length;
  env.advance(90000);
  assert.equal(env.fetches.length, count, 'a closed section fetches nothing');
  view.open();
  assert.equal(env.fetches.length, count + 1);
  env.doc.visibilityState = 'hidden';
  env.advance(30000);
  assert.equal(env.fetches.length, count + 1, 'a hidden tab does not poll');
});

test('update follows the card without a rebuild', async () => {
  const env = load();
  const page = await start(env, [cm(1)], {}, report({ comment_count: 1 }));
  const first = page.items()[0];
  const count = env.fetches.length;
  page.view.update(report({ comment_count: 1 }));
  assert.equal(env.fetches.length, count, 'the same count asks nothing');
  page.view.update(report({ comment_count: 2 }));
  assert.equal(env.fetches.length, count + 1, 'a changed count asks again');
  lastFetch(env).reply(200, body([cm(1), cm(2)]));
  await flush();
  assert.equal(page.items()[0], first);
  page.view.update(report({ comment_count: 2, comments_open: false }));
  assert.equal(find(page.view.el, 'comments__closed').hidden, false);
  assert.equal(page.form.hidden, true);
});

// ---- Phase 8 plan 17 (PAGE-02, D-02, D-14): Like, Delete and Flag ----

// addBoxes puts the Delete box into the fake page the way index.html.tmpl does.
function addBoxes(env) {
  const byId = {};
  const make = (tag, id) => { const el = new FakeEl(env.doc, tag); el.id = id; byId[id] = el; return el; };
  const box = make('dialog', 'comment-delete-confirm');
  const title = make('h2', 'comment-delete-title');
  const cancelButton = make('button', 'comment-delete-cancel');
  const yes = make('button', 'comment-delete-yes');
  box.open = false;
  box.showModal = () => { box.open = true; };
  box.close = () => { if (box.open) { box.open = false; box.fire('close'); } };
  // Amended 2026-10-30 by quick 261030-u78: the Block box (WR-E04) is in the
  // page too, with the same showModal and close fakes, returned as block.
  const blockBox = make('dialog', 'comment-block-confirm');
  const blockTitle = make('h2', 'comment-block-title');
  const blockText = make('div', 'comment-block-text');
  const blockCancel = make('button', 'comment-block-cancel');
  const blockYes = make('button', 'comment-block-yes');
  blockBox.open = false;
  blockBox.showModal = () => { blockBox.open = true; };
  blockBox.close = () => { if (blockBox.open) { blockBox.open = false; blockBox.fire('close'); } };
  env.doc.getElementById = (id) => byId[id] || null;
  return {
    box, title, cancel: cancelButton, yes,
    block: { box: blockBox, title: blockTitle, text: blockText, cancel: blockCancel, yes: blockYes }
  };
}

function likeButton(li) { return find(li, 'comment__like'); }
function deleteButton(li) { return find(li, 'comment__delete'); }
function flagButton(li) { return find(li, 'comment__flag'); }
function actionError(li) { return find(li, 'comment__error'); }

test('like toggles with aria-pressed', async () => {
  const env = load();
  const page = await start(env, [cm(1, { likes: 2 })]);
  const li = page.items()[0];
  const like = likeButton(li);
  assert.equal(like.getAttribute('aria-pressed'), 'false');
  like.fire('click');
  assert.equal(posts(env).length, 1);
  assert.equal(lastFetch(env).address, '/api/comments/1/like');
  assert.equal(like.getAttribute('aria-disabled'), 'true', 'busy while the request runs');
  like.fire('click');
  assert.equal(posts(env).length, 1, 'a second press sends nothing');
  assert.equal(like.getAttribute('aria-pressed'), 'false', 'nothing is guessed before the answer');
  lastFetch(env).reply(200, { likes: 3, viewer_liked: true });
  await flush();
  assert.equal(like.getAttribute('aria-pressed'), 'true');
  assert.equal(like.hasAttribute('aria-disabled'), false);
  assert.ok(like.hasClass('comment__like--on'));
  assert.equal(find(like, 'comment__like-count').textContent, '3', 'the count is the server count');
  like.fire('click');
  assert.equal(lastFetch(env).address, '/api/comments/1/unlike');
  lastFetch(env).reply(200, { likes: 2, viewer_liked: false });
  await flush();
  assert.equal(like.getAttribute('aria-pressed'), 'false');
  assert.equal(find(like, 'comment__like-count').textContent, '2');
  assert.equal(like.hasClass('comment__like--on'), false);
});

test('like names and counts', async () => {
  const env = load();
  const page = await start(env, [cm(1), cm(2, { likes: 1 }), cm(3, { likes: 3, viewer_liked: true })]);
  const [zero, one, three] = page.items();
  assert.equal(likeButton(zero).getAttribute('aria-label'), 'Like');
  assert.equal(find(likeButton(zero), 'comment__like-count').hidden, true, 'no count at zero');
  assert.equal(likeButton(one).getAttribute('aria-label'), 'Like, 1 like');
  assert.equal(find(likeButton(one), 'comment__like-count').hidden, false);
  assert.equal(likeButton(three).getAttribute('aria-label'), 'Like, 3 likes');
  assert.equal(likeButton(three).getAttribute('aria-pressed'), 'true');
  assert.equal(likeButton(zero).getAttribute('type'), 'button');
});

test('closed shows the count as text', async () => {
  const env = load();
  const page = await start(env, [cm(1, { likes: 4 }), cm(2), cm(3, { can_delete: true, is_own: true })], { comments_open: false });
  const [counted, none, own] = page.items();
  assert.equal(replyButton(counted).hidden, true);
  assert.equal(likeButton(counted).hidden, true, 'the Like button is not drawn');
  const text = find(counted, 'comment__likes');
  assert.equal(text.hidden, false);
  assert.equal(text.getAttribute('role'), 'img');
  assert.equal(text.getAttribute('aria-label'), '4 likes');
  assert.equal(text.tagName, 'SPAN');
  assert.equal(find(none, 'comment__likes').hidden, true, 'hidden at zero');
  assert.equal(flagButton(counted).hidden, false, 'Flag stays when comments are closed');
  assert.equal(deleteButton(own).hidden, false, 'Delete stays when comments are closed');
  assert.equal(flagButton(own).hidden, true);
  assert.equal(deleteButton(counted).hidden, true);
});

test('delete asks first and leaves a placeholder', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const page = await start(env, [
    cm(1, { is_own: true, can_delete: true, author: 'me', replies: [cm(2, { reply_to_author: 'me' })], reply_count: 1, likes: 2 }),
    cm(3, { is_own: true, can_delete: true, author: 'me' })
  ]);
  const [withReplies, alone] = page.items();
  deleteButton(withReplies).fire('click');
  assert.equal(boxes.box.open, true, 'the box opens');
  assert.equal(posts(env).length, 0, 'nothing is sent until the box says Delete');
  assert.equal(env.doc.activeElement, boxes.title, 'focus is on the title');
  assert.equal(boxes.cancel.focusCalls + boxes.yes.focusCalls, 0, 'no button is focused');
  boxes.cancel.fire('click');
  assert.equal(boxes.box.open, false);
  assert.equal(posts(env).length, 0, 'Cancel sends nothing');
  assert.equal(env.doc.activeElement, deleteButton(withReplies), 'focus returns to Delete');
  deleteButton(withReplies).fire('click');
  boxes.yes.fire('click');
  assert.equal(lastFetch(env).address, '/api/comments/1/delete');
  assert.equal(lastFetch(env).opts.method, 'POST');
  lastFetch(env).reply(200, { deleted: true });
  await flush();
  assert.equal(find(withReplies, 'comment__gone').hidden, false);
  assert.equal(find(withReplies, 'comment__gone').textContent, 'Comment deleted');
  assert.equal(find(withReplies, 'comment__meta').hidden, true, 'no controls on the placeholder');
  assert.equal(findAll(withReplies, 'comment--reply').length, 1, 'its replies stay');
  // the list is asked again; answer it with the server's own picture
  lastFetch(env).reply(200, body([cm(1, { deleted: true, author: '', words: '', replies: [cm(2)] }), cm(3, { is_own: true, can_delete: true, author: 'me' })]));
  await flush();
  assert.equal(page.items().length, 2);
  deleteButton(alone).fire('click');
  boxes.yes.fire('click');
  lastFetch(env).reply(200, { deleted: true });
  await flush();
  assert.equal(page.items().length, 1, 'a comment with no replies disappears');
  assert.equal(find(page.view.el, 'comments__status').textContent, 'Comment deleted.');
});

test('delete moves focus to the heading', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const page = await start(env, [cm(1, { is_own: true, can_delete: true, author: 'me' })]);
  const li = page.items()[0];
  deleteButton(li).fire('click');
  boxes.yes.fire('click');
  assert.equal(deleteButton(li).getAttribute('aria-disabled'), 'true', 'busy while it runs');
  lastFetch(env).reply(200, { deleted: true });
  await flush();
  assert.equal(env.doc.activeElement, find(page.view.el, 'comments__heading'));
  assert.equal(find(page.view.el, 'comments__empty').hidden, false, 'an empty list says so');
});

test('delete failure keeps the comment and says why', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const page = await start(env, [cm(1, { is_own: true, can_delete: true, author: 'me' })]);
  const li = page.items()[0];
  deleteButton(li).fire('click');
  boxes.yes.fire('click');
  lastFetch(env).reply(429, { error: { field: 'rate_like', message: "You're reacting very quickly. You can try again in 20 seconds." } });
  await flush();
  assert.equal(page.items().length, 1);
  assert.equal(actionError(li).hidden, false);
  assert.equal(actionError(li).textContent, "You're reacting very quickly. You can try again in 20 seconds.");
  assert.equal(env.doc.activeElement, deleteButton(li));
});

test('flag opens comment mode', async () => {
  const env = load();
  const opened = [];
  env.sandbox.PinalertFlag = { openForComment(opts) { opened.push(opts); } };
  const page = await start(env, [cm(7)]);
  const li = page.items()[0];
  const flag = flagButton(li);
  assert.equal(flag.getAttribute('aria-label'), 'Flag this comment');
  assert.equal(flag.textContent, 'Flag');
  flag.fire('click');
  assert.equal(opened.length, 1);
  assert.equal(String(opened[0].commentId), '7');
  assert.equal(opened[0].opener, flag);
  assert.equal(posts(env).length, 0, 'this control never sends a flag itself');
  assert.equal(typeof opened[0].onSent, 'function');
  opened[0].onSent();
  assert.equal(flag.textContent, 'Flagged');
  assert.equal(flag.getAttribute('aria-disabled'), 'true');
  flag.fire('click');
  assert.equal(opened.length, 1, 'Flagged opens nothing again');
});

test('flagged reads Flagged', async () => {
  const env = load();
  const page = await start(env, [cm(1, { viewer_flagged: true }), cm(2)]);
  const [flagged, plain] = page.items();
  assert.equal(flagButton(flagged).textContent, 'Flagged');
  assert.equal(flagButton(flagged).getAttribute('aria-disabled'), 'true');
  assert.equal(flagButton(flagged).getAttribute('aria-label'), null, 'its name is its word');
  assert.equal(flagButton(plain).textContent, 'Flag');
  assert.equal(flagButton(plain).hasAttribute('aria-disabled'), false);
});

test('placeholders have no controls', async () => {
  const env = load();
  const page = await start(env, [
    cm(1, { deleted: true, author: '', words: '', likes: 3, viewer_liked: true, replies: [cm(2)] }),
    cm(3, { removed: true, author: '', words: '', likes: 2 })
  ]);
  const [deleted, removed] = page.items();
  for (const li of [deleted, removed]) {
    assert.equal(find(li, 'comment__meta').hidden, true, 'the whole meta line is hidden');
  }
  // a poll never adds them back
  page.view.refresh();
  lastFetch(env).reply(200, body([
    cm(1, { deleted: true, author: '', words: '', likes: 3, can_delete: true, replies: [cm(2)] }),
    cm(3, { removed: true, author: '', words: '', likes: 2, can_delete: true })
  ]));
  await flush();
  for (const li of page.items()) {
    assert.equal(find(li, 'comment__meta').hidden, true);
  }
  // and the replies of a deleted comment keep theirs
  assert.equal(flagButton(findAll(page.items()[0], 'comment--reply')[0]).hidden, false);
});

test('held own comment keeps Delete', async () => {
  const env = load();
  const page = await start(env, [cm(1, { held: true, is_own: true, can_delete: true, author: 'me' })]);
  const li = page.items()[0];
  assert.equal(deleteButton(li).hidden, false);
  assert.equal(likeButton(li).hidden, true);
  assert.equal(flagButton(li).hidden, true);
});

test('reacting too fast keeps the thread', async () => {
  const env = load();
  const page = await start(env, [cm(1, { replies: [cm(2)], reply_count: 1 })]);
  const li = page.items()[0];
  find(li, 'comment-thread__toggle').fire('click');
  page.input.value = 'my words';
  page.input.fire('input');
  likeButton(li).fire('click');
  lastFetch(env).reply(429, { error: { field: 'rate_like', message: "You're reacting very quickly. You can try again in 3 seconds." } });
  await flush();
  assert.equal(actionError(li).hidden, false);
  assert.equal(actionError(li).textContent, "You're reacting very quickly. You can try again in 3 seconds.");
  assert.equal(likeButton(li).getAttribute('aria-pressed'), 'false');
  assert.equal(page.input.value, 'my words', 'the box keeps its words');
  assert.equal(find(li, 'comment-thread').hidden, false, 'the open thread stays open');
  likeButton(li).fire('click');
  assert.equal(actionError(li).hidden, true, 'the sentence clears on the next try');
  lastFetch(env).reply(200, { likes: 1, viewer_liked: true });
  await flush();
  assert.equal(actionError(li).hidden, true);
  assert.equal(likeButton(li).getAttribute('aria-pressed'), 'true');
});

test('offline like says so', async () => {
  const env = load();
  const page = await start(env, [cm(1)]);
  env.sandbox.navigator.onLine = false;
  const li = page.items()[0];
  const before = env.fetches.length;
  likeButton(li).fire('click');
  assert.equal(env.fetches.length, before, 'nothing is sent');
  assert.equal(actionError(li).textContent, "You're offline, so this wasn't saved. Try again when you have a signal.");
});

// ---- review fix WR-B01 (2026-10-29): every comment request ends at the page's
// own 10 s limit (Pinalert.requestTimeout), even when the aborted request never
// settles (WebKit) or the body read stalls after the headers. ----

test('WR-B01: a post whose aborted request never settles gives Post back at 10 s', async () => {
  const env = load({ ignoreAbort: true });
  const page = await start(env, [cm(1)]);
  type(page, 'Water at knee height near the bus stand');
  page.post.fire('click');
  assert.equal(page.post.textContent, 'Posting…');
  env.advance(10000);
  await flush();
  assert.equal(page.post.textContent, 'Post', 'Post is back');
  assert.equal(page.post.getAttribute('aria-disabled'), 'false');
  assert.equal(page.error.hidden, false);
  assert.equal(page.error.textContent, "Couldn't post your comment. Try again.");
  assert.equal(page.input.value, 'Water at knee height near the bus stand', 'the words stay');
  const before = posts(env).length;
  page.post.fire('click');
  assert.equal(posts(env).length, before + 1, 'a second press sends a second POST');
});

test('WR-B01: a like whose aborted request never settles gives Like back at 10 s', async () => {
  const env = load({ ignoreAbort: true });
  const page = await start(env, [cm(1)]);
  const li = page.items()[0];
  likeButton(li).fire('click');
  assert.equal(likeButton(li).getAttribute('aria-disabled'), 'true');
  env.advance(10000);
  await flush();
  assert.equal(likeButton(li).getAttribute('aria-disabled'), null, 'Like is back on');
  assert.equal(actionError(li).hidden, false);
  const before = posts(env).length;
  likeButton(li).fire('click');
  assert.equal(posts(env).length, before + 1, 'a second tap sends a second POST');
});

test('WR-B01: a like answered with a body that never ends gives Like back at 10 s', async () => {
  const env = load();
  const page = await start(env, [cm(1)]);
  const li = page.items()[0];
  likeButton(li).fire('click');
  lastFetch(env).stall(200);
  await flush();
  env.advance(10000);
  await flush();
  assert.equal(likeButton(li).getAttribute('aria-disabled'), null, 'Like is back on');
  const before = posts(env).length;
  likeButton(li).fire('click');
  assert.equal(posts(env).length, before + 1);
});

test('WR-B01: Show more whose body never ends can be pressed again at 10 s', async () => {
  const env = load();
  const page = await start(env, [cm(1)], { more: true, next_after: 1 });
  const more = find(page.view.el, 'comments__more');
  more.fire('click');
  lastFetch(env).stall(200);
  await flush();
  env.advance(10000);
  await flush();
  assert.equal(more.getAttribute('aria-disabled'), null);
  assert.equal(find(page.view.el, 'comments__more-error').hidden, false);
  const before = env.fetches.length;
  more.fire('click');
  assert.equal(env.fetches.length, before + 1, 'Show more asks again');
});

// ---- review fix WR-B03 (2026-10-29): a post that succeeds keeps the words
// typed while it was in flight, and a reply target chosen meanwhile. ----

test('WR-B03: words typed while the post was out stay in the box', async () => {
  const env = load();
  const page = await start(env, [cm(1)]);
  type(page, 'Road closed at the flyover');
  page.post.fire('click');
  const sent = lastFetch(env);
  type(page, 'Road closed at the flyover Use the service road instead');
  sent.reply(201, cm(2, { is_own: true, can_delete: true, author: 'me.k', words: 'Road closed at the flyover' }));
  await flush();
  assert.equal(page.input.value, 'Use the service road instead', 'only the sent words go');
});

test('WR-B03: a box rewritten while the post was out is left alone', async () => {
  const env = load();
  const page = await start(env, [cm(1)]);
  type(page, 'Road closed');
  page.post.fire('click');
  const sent = lastFetch(env);
  type(page, 'Something else');
  sent.reply(201, cm(2, { is_own: true, author: 'me.k', words: 'Road closed' }));
  await flush();
  assert.equal(page.input.value, 'Something else');
});

test('WR-B03: a reply target chosen while the post was out survives', async () => {
  const env = load();
  const page = await start(env, [cm(1, { author: 'ravi.k' }), cm(3, { author: 'asha.m' })]);
  type(page, 'Road closed');
  page.post.fire('click');
  const sent = lastFetch(env);
  replyButton(page.items()[1]).fire('click');
  assert.equal(page.input.value, '@asha.m Road closed');
  sent.reply(201, cm(4, { is_own: true, author: 'me.k', words: 'Road closed' }));
  await flush();
  assert.equal(page.replying.hidden, false, 'the new reply target stays');
  assert.equal(page.input.getAttribute('aria-label'), 'Reply to asha.m');
});

test('WR-B03: the sent reply clears its own target and the box', async () => {
  const env = load();
  const page = await start(env, [cm(1, { author: 'ravi.k' })]);
  replyButton(page.items()[0]).fire('click');
  type(page, '@ravi.k thanks');
  page.post.fire('click');
  lastFetch(env).reply(201, cm(5, { is_own: true, author: 'me.k', words: '@ravi.k thanks', reply_to_author: 'ravi.k' }));
  await flush();
  assert.equal(page.input.value, '');
  assert.equal(page.replying.hidden, true);
});

// ---- review fix WR-B04 (2026-10-29): after Show more the poll also reads the
// last page, so new comments keep appearing on a report with more than 50, and
// every top level comment sits in id order, your own post too. ----

function roots(from, to) {
  const out = [];
  for (let id = from; id <= to; id += 1) { out.push(cm(id, { words: 'c' + id })); }
  return out;
}
function shownIds(page) { return page.items().map((li) => wordsOf(li).textContent); }

test('WR-B04: after Show more, a new comment on the last page appears at the next poll', async () => {
  const env = load();
  const page = await start(env, roots(1, 50), { more: true, next_after: 50, comment_count: 70 });
  find(page.view.el, 'comments__more').fire('click');
  assert.ok(lastFetch(env).address.endsWith('?after=50'));
  lastFetch(env).reply(200, body(roots(51, 70), { more: false, next_after: 0 }));
  await flush();
  assert.equal(page.items().length, 70);
  env.advance(30000);
  lastFetch(env).reply(200, body(roots(1, 50), { more: true, next_after: 50, comment_count: 71 }));
  await flush();
  const tail = lastFetch(env);
  assert.ok(tail.address.endsWith('?after=50'), 'the poll reads the last page too, it asked ' + tail.address);
  tail.reply(200, body(roots(51, 71), { more: false, next_after: 0, comment_count: 71 }));
  await flush();
  const ids = shownIds(page);
  assert.equal(ids.length, 71);
  assert.equal(ids[70], 'c71', 'the new comment is last');
  assert.equal(ids[0], 'c1', 'the first page stays first');
});

test('WR-B04: a last page that grew past 50 brings Show more back', async () => {
  const env = load();
  const page = await start(env, roots(1, 50), { more: true, next_after: 50 });
  const more = find(page.view.el, 'comments__more');
  more.fire('click');
  lastFetch(env).reply(200, body(roots(51, 60), { more: false, next_after: 0 }));
  await flush();
  assert.equal(more.hidden, true);
  env.advance(30000);
  lastFetch(env).reply(200, body(roots(1, 50), { more: true, next_after: 50 }));
  await flush();
  lastFetch(env).reply(200, body(roots(51, 100), { more: true, next_after: 100 }));
  await flush();
  assert.equal(more.hidden, false, 'Show more is back for the comments past the last page');
  more.fire('click');
  assert.ok(lastFetch(env).address.endsWith('?after=100'));
});

test('WR-B04: your own post on a long report lands after the older pages', async () => {
  const env = load();
  const page = await start(env, roots(1, 50), { more: true, next_after: 50, comment_count: 70 });
  type(page, 'Shelter is full');
  page.post.fire('click');
  lastFetch(env).reply(201, cm(100, { is_own: true, can_delete: true, author: 'me.k', words: 'c100' }));
  await flush();
  lastFetch(env).reply(200, body(roots(1, 50), { more: true, next_after: 50, comment_count: 71 }));
  await flush();
  find(page.view.el, 'comments__more').fire('click');
  lastFetch(env).reply(200, body(roots(51, 70).concat([cm(100, { is_own: true, author: 'me.k', words: 'c100' })]), { more: false, next_after: 0 }));
  await flush();
  const ids = shownIds(page);
  assert.equal(ids.length, 71);
  assert.equal(ids[50], 'c51', 'page 2 follows page 1');
  assert.equal(ids[70], 'c100', 'your own post is last, in order');
});

// ---- review fix WR-B05 (2026-10-29): a list answer whose request left before
// a Like answered never writes the older like state over the newer one. ----

test('WR-B05: a poll sent before the like and answered after it keeps the like', async () => {
  const env = load();
  const page = await start(env, [cm(1, { likes: 3 })]);
  const li = page.items()[0];
  env.advance(30000);
  const poll = lastFetch(env);
  assert.equal(poll.opts.method, 'GET');
  likeButton(li).fire('click');
  lastFetch(env).reply(200, { likes: 4, viewer_liked: true });
  await flush();
  poll.reply(200, body([cm(1, { likes: 3, viewer_liked: false })]));
  await flush();
  assert.equal(likeButton(li).getAttribute('aria-pressed'), 'true');
  assert.equal(find(li, 'comment__like-count').textContent, '4');
});

test('WR-B05: a poll sent while the like was out keeps the like too', async () => {
  const env = load();
  const page = await start(env, [cm(1, { likes: 3 })]);
  const li = page.items()[0];
  // The like leaves at 25 s and the poll at 30 s, inside the like's 10 s limit.
  env.advance(25000);
  likeButton(li).fire('click');
  const like = lastFetch(env);
  env.advance(5000);
  const poll = lastFetch(env);
  assert.equal(poll.opts.method, 'GET');
  like.reply(200, { likes: 4, viewer_liked: true });
  await flush();
  poll.reply(200, body([cm(1, { likes: 3, viewer_liked: false })]));
  await flush();
  assert.equal(likeButton(li).getAttribute('aria-pressed'), 'true');
  assert.equal(find(li, 'comment__like-count').textContent, '4');
});

test('WR-B05: a poll sent after the like answered is written as it comes', async () => {
  const env = load();
  const page = await start(env, [cm(1, { likes: 3 })]);
  const li = page.items()[0];
  likeButton(li).fire('click');
  lastFetch(env).reply(200, { likes: 4, viewer_liked: true });
  await flush();
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(1, { likes: 2, viewer_liked: false })]));
  await flush();
  assert.equal(likeButton(li).getAttribute('aria-pressed'), 'false', 'a newer answer wins');
  assert.equal(find(li, 'comment__like-count').textContent, '2');
});

// ---- review fix IN-B07 (2026-10-29): the Enter that ends a composition in
// Safari (keyCode 229, isComposing false) never posts. ----

test('IN-B07: an Enter with keyCode 229 ends the composition and does not post', async () => {
  const env = load();
  const page = await start(env, [cm(1)]);
  type(page, 'నీరు');
  const before = posts(env).length;
  page.input.fire('keydown', { key: 'Enter', keyCode: 229, isComposing: false });
  assert.equal(posts(env).length, before, 'nothing is posted');
  page.input.fire('keydown', { key: 'Enter', keyCode: 13 });
  assert.equal(posts(env).length, before + 1, 'a plain Enter posts');
});

// ---- review fix IN-B02 (2026-10-29): a post with no readable answer asks for
// the list again, so a comment the server did store shows before a second press. ----

test('IN-B02: a post that timed out asks for the list again', async () => {
  const env = load();
  const page = await start(env, [cm(1)]);
  type(page, 'Shelter is open');
  page.post.fire('click');
  env.advance(10000);
  await flush();
  const last = lastFetch(env);
  assert.equal(last.opts.method, 'GET', 'the list is asked for after the unknown outcome');
  last.reply(200, body([cm(1), cm(2, { is_own: true, author: 'me.k', words: 'Shelter is open' })]));
  await flush();
  assert.equal(page.items().length, 2, 'the stored comment shows');
  assert.equal(page.input.value, 'Shelter is open', 'the words stay');
});

test('IN-B02: a 201 whose body cannot be read asks for the list again', async () => {
  const env = load();
  const page = await start(env, [cm(1)]);
  type(page, 'Shelter is open');
  page.post.fire('click');
  lastFetch(env).stall(201);
  await flush();
  env.advance(10000);
  await flush();
  assert.equal(lastFetch(env).opts.method, 'GET');
  assert.equal(page.error.textContent, "Couldn't post your comment. Try again.");
});

// ---- review fix IN-B06 (2026-10-29): the comments refresh when the tab comes
// back, and the listener goes with the section's close. ----

test('IN-B06: the comments refresh when the tab comes back, and not after close', async () => {
  const env = load();
  const page = await start(env, [cm(1)]);
  const before = env.fetches.length;
  env.doc.visibilityState = 'hidden';
  env.doc.dispatch('visibilitychange');
  assert.equal(env.fetches.length, before, 'nothing while hidden');
  env.doc.visibilityState = 'visible';
  env.doc.dispatch('visibilitychange');
  assert.equal(env.fetches.length, before + 1, 'the list is asked for again');
  page.view.close();
  assert.equal((env.doc.listeners.visibilitychange || []).length, 0, 'the listener is removed on close');
  page.view.open();
  page.view.open();
  assert.equal(env.doc.listeners.visibilitychange.length, 1, 'one listener per open section');
});

// ---- review fix round 2, SK-01 (2026-10-29): a thread longer than the reply
// cap keeps every reply reachable. The toggle counts reply_count, Show more
// replies reads the rest of the thread, and a poll that covers only the first
// replies never drops the ones after them, a reader's own new reply included. ----

function replyRange(from, to) {
  const out = [];
  for (let id = from; id <= to; id++) { out.push(cm(id, { words: 'reply ' + id })); }
  return out;
}
function threadIds(li) { return findAll(li, 'comment--reply').map((n) => wordsOf(n).textContent); }

test('SK-01: a long thread counts reply_count and Show more replies reads the rest', async () => {
  const env = load();
  const page = await start(env, [cm(1, { replies: replyRange(2, 101), reply_count: 101 })]);
  const [li] = page.items();
  const toggle = find(li, 'comment-thread__toggle');
  const more = find(li, 'comment-thread__more');
  assert.equal(toggle.textContent, 'View 101 replies', 'the toggle counts every reply, not the 100 sent');
  assert.equal(more.hidden, true, 'folded, the thread shows no Show more replies');
  toggle.fire('click');
  assert.equal(more.hidden, false, 'an open thread with replies left offers them');
  assert.equal(more.textContent, 'Show more replies');
  assert.equal(more.tagName, 'BUTTON');
  more.fire('click');
  const call = lastFetch(env);
  assert.equal(call.address, '/api/reports/r1/comments?replies_of=1&reply_after=101');
  call.reply(200, body([cm(1, { reply_count: 101, replies: [cm(102, { words: 'reply 102' })] })], { more: false, next_after: 102 }));
  await flush();
  const ids = threadIds(li);
  assert.equal(ids.length, 101);
  assert.equal(ids[100], 'reply 102', 'the 101st reply lands last, oldest first');
  assert.equal(more.hidden, true, 'nothing is left to show');

  env.advance(30000);
  lastFetch(env).reply(200, body([cm(1, { replies: replyRange(2, 101), reply_count: 101 })]));
  await flush();
  assert.equal(threadIds(li).length, 101, 'a poll that covers the first 100 keeps the 101st');
  assert.equal(threadIds(li)[100], 'reply 102');
});

test('SK-01: your own reply in a long thread stays at the next poll', async () => {
  const env = load();
  const page = await start(env, [cm(1, { author: 'ravi.k', replies: replyRange(2, 101), reply_count: 101 })]);
  const [li] = page.items();
  replyButton(li).fire('click');
  type(page, 'the hall next door has room');
  page.post.fire('click');
  posts(env)[0].reply(201, cm(103, { is_own: true, words: 'mine' }));
  await flush();
  assert.equal(threadIds(li).length, 101);
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(1, { author: 'ravi.k', replies: replyRange(2, 101), reply_count: 102 })]));
  await flush();
  const ids = threadIds(li);
  assert.equal(ids.length, 101, 'the poll did not drop the new reply');
  assert.equal(ids[100], 'mine');
  const toggle = find(li, 'comment-thread__toggle');
  toggle.fire('click');
  assert.equal(toggle.textContent, 'View 102 replies');
  toggle.fire('click');
  const more = find(li, 'comment-thread__more');
  assert.equal(more.hidden, false, 'reply 102 is still to be read');
  more.fire('click');
  lastFetch(env).reply(200, body([cm(1, { reply_count: 102, replies: [cm(102, { words: 'reply 102' }), cm(103, { is_own: true, words: 'mine' })] })], { more: false, next_after: 103 }));
  await flush();
  const after = threadIds(li);
  assert.equal(after.length, 102);
  assert.deepEqual(after.slice(99), ['reply 101', 'reply 102', 'mine'], 'the read reply sits before the newer own one');
  assert.equal(more.hidden, true);
});

test('SK-01: a failed Show more replies says so and can be pressed again', async () => {
  const env = load();
  const page = await start(env, [cm(1, { replies: replyRange(2, 101), reply_count: 101 })]);
  const [li] = page.items();
  find(li, 'comment-thread__toggle').fire('click');
  const more = find(li, 'comment-thread__more');
  more.fire('click');
  lastFetch(env).reply(500, null);
  await flush();
  const error = find(li, 'comment-thread__more-error');
  assert.equal(error.hidden, false);
  assert.equal(error.textContent, "Couldn't load more replies. Try again.");
  assert.equal(more.hidden, false);
  const before = env.fetches.length;
  more.fire('click');
  assert.equal(env.fetches.length, before + 1, 'a second press asks again');
});

test('SK-01: a thread the server sends whole still drops a reply that went', async () => {
  const env = load();
  const page = await start(env, [cm(1, { replies: [cm(2, { words: 'a' }), cm(3, { words: 'b' })], reply_count: 2 })]);
  const [li] = page.items();
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(1, { replies: [cm(3, { words: 'b' })], reply_count: 1 })]));
  await flush();
  assert.deepEqual(threadIds(li), ['b']);
  assert.equal(find(li, 'comment-thread__toggle').textContent, 'View 1 reply');
});

// ---- review fix round 3, SK2-01 (2026-10-29): an open capped thread whose
// tail was read rereads that tail at the next poll, so a reply gone from it
// leaves the screen and a new reply beyond the first 100 becomes reachable
// without reopening the panel. ----

function openedLongThread(page) {
  const [li] = page.items();
  find(li, 'comment-thread__toggle').fire('click');
  const more = find(li, 'comment-thread__more');
  more.fire('click');
  return { li, more };
}

test('SK2-01: a gone reply and a new reply in the read tail of an open thread follow the poll', async () => {
  const env = load();
  const page = await start(env, [cm(1, { replies: replyRange(2, 101), reply_count: 150 })]);
  const { li, more } = openedLongThread(page);
  lastFetch(env).reply(200, body([cm(1, { reply_count: 150, replies: replyRange(102, 151) })], { more: false, next_after: 151 }));
  await flush();
  assert.equal(threadIds(li).length, 150);
  assert.equal(more.hidden, true);

  // Reply 120 goes and reply 152 is posted by someone else: the counts agree.
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(1, { replies: replyRange(2, 101), reply_count: 150 })]));
  await flush();
  const call = lastFetch(env);
  assert.equal(call.address, '/api/reports/r1/comments?replies_of=1&reply_after=101', 'the read tail is read again');
  call.reply(200, body([cm(1, { reply_count: 150, replies: replyRange(102, 152).filter((c) => c.id !== 120) })], { more: false, next_after: 152 }));
  await flush();
  const ids = threadIds(li);
  assert.equal(ids.length, 150);
  assert.equal(ids.includes('reply 120'), false, 'the gone reply left the screen');
  assert.equal(ids[149], 'reply 152', 'the new reply is on the screen');
  assert.equal(more.hidden, true);
});

test('SK2-01: a reread tail longer than one page keeps Show more replies', async () => {
  const env = load();
  const page = await start(env, [cm(1, { replies: replyRange(2, 101), reply_count: 150 })]);
  const { li, more } = openedLongThread(page);
  lastFetch(env).reply(200, body([cm(1, { reply_count: 150, replies: replyRange(102, 151) })], { more: false, next_after: 151 }));
  await flush();
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(1, { replies: replyRange(2, 101), reply_count: 300 })]));
  await flush();
  lastFetch(env).reply(200, body([cm(1, { reply_count: 300, replies: replyRange(102, 201) })], { more: true, next_after: 201 }));
  await flush();
  assert.equal(threadIds(li).length, 200, 'the held tail stays');
  assert.equal(more.hidden, false, 'replies after the page read are still to be read');
});

test('SK2-01: a thread never read past the first 100 is not read again', async () => {
  const env = load();
  const page = await start(env, [cm(1, { replies: replyRange(2, 101), reply_count: 101 })]);
  find(page.items()[0], 'comment-thread__toggle').fire('click');
  const before = env.fetches.length;
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(1, { replies: replyRange(2, 101), reply_count: 101 })]));
  await flush();
  assert.equal(env.fetches.length, before + 1, 'only the poll itself went out');
});

// ---- Quick 261030-u78 (WR-E04, D-12): Block beside Flag ----

function blockButton(li) { return find(li, 'comment__block'); }

// recordToasts gives the sandbox the app's toast function and keeps what it was
// asked to show.
function recordToasts(env) {
  const toasts = [];
  env.sandbox.Pinalert.showToast = (message) => { toasts.push(message); };
  return toasts;
}

function blockPosts(env) { return posts(env).filter((f) => /block-author$/.test(f.address)); }

test('Block sits right after Flag with its own name', async () => {
  const env = load();
  const page = await start(env, [cm(1)]);
  const [named] = page.items();
  const block = blockButton(named);
  assert.ok(block, 'a Block button exists');
  assert.equal(block.tagName, 'BUTTON');
  assert.equal(block.getAttribute('type'), 'button');
  assert.equal(block.textContent, 'Block');
  assert.equal(block.getAttribute('aria-label'), 'Block ravi.k');
  assert.equal(block.hidden, false);
  const meta = find(named, 'comment__meta');
  assert.equal(meta.children[meta.children.indexOf(flagButton(named)) + 1], block, 'the next element after Flag');
});

test('Block is hidden where it does not belong', async () => {
  const env = load();
  const page = await start(env, [
    cm(1, { is_own: true, can_delete: true, author: 'me' }),
    cm(2, { is_own: true, held: true, author: 'me' }),
    cm(3, { author: '', author_kind: 'deleted_account' }),
    cm(4, { deleted: true, author: '', words: '' }),
    cm(5, { removed: true, author: '', words: '' }),
    cm(6, { hidden: true, author: '', words: '' })
  ]);
  const items = page.items();
  items.forEach((li, i) => assert.equal(blockButton(li).hidden, true, 'comment ' + (i + 1)));
  env.advance(30000);
  lastFetch(env).reply(200, body([
    cm(4, { deleted: true, author: '', words: '' }), cm(5, { removed: true, author: '', words: '' })
  ]));
  await flush();
  assert.equal(blockButton(page.items()[0]).hidden, true, 'still hidden after a poll');
});

test('Block stays when comments are closed', async () => {
  const env = load();
  const page = await start(env, [cm(1)], { comments_open: false });
  assert.equal(blockButton(page.items()[0]).hidden, false);
});

test('Block asks first in the box and only its Block button sends', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const page = await start(env, [cm(1)]);
  const li = page.items()[0];
  const b = boxes.block;
  blockButton(li).fire('click');
  assert.equal(b.box.open, true, 'the box opens');
  assert.equal(blockPosts(env).length, 0, 'nothing is sent until the box says Block');
  assert.equal(b.title.textContent, 'Block ravi.k?');
  const lines = b.text.children.map((p) => p.textContent);
  assert.deepEqual(lines, ['You will not see their comments. They are not told.', 'You can unblock them in Settings, Blocked people.']);
  assert.equal(env.doc.activeElement, b.title, 'focus is on the title');
  assert.equal(b.cancel.focusCalls + b.yes.focusCalls, 0, 'no button is focused');
  b.cancel.fire('click');
  assert.equal(b.box.open, false);
  assert.equal(blockPosts(env).length, 0, 'Cancel sends nothing');
  assert.equal(env.doc.activeElement, blockButton(li), 'focus returns to Block');
  blockButton(li).fire('click');
  assert.equal(b.text.children.length, 2, 'the lines are replaced, not added to');
  b.yes.fire('click');
  assert.equal(blockPosts(env).length, 1);
  assert.equal(lastFetch(env).address, '/api/comments/1/block-author');
  assert.equal(lastFetch(env).opts.method, 'POST');
  assert.equal(lastFetch(env).opts.body, undefined, 'no body');
  blockButton(li).fire('click');
  assert.equal(b.box.open, false, 'a second press while it is out opens nothing');
  assert.equal(blockPosts(env).length, 1, 'and sends nothing more');
});

// Review CR-01 (2026-10-30): Block is offered only where the name is shown, so
// a New member comment has no Block, not even after a poll, and a press on the
// hidden button opens nothing.
test('no Block on a New member comment', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const page = await start(env, [cm(1, { author: 'New member', author_kind: 'new_member' }), cm(2, { author: '', author_kind: 'new_member' })]);
  page.items().forEach((li, i) => assert.equal(blockButton(li).hidden, true, 'comment ' + (i + 1)));
  assert.equal(flagButton(page.items()[0]).hidden, false, 'Flag stays');
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(1, { author: 'New member', author_kind: 'new_member' })]));
  await flush();
  assert.equal(blockButton(page.items()[0]).hidden, true, 'still hidden after a poll');
  blockButton(page.items()[0]).fire('click');
  assert.equal(boxes.block.box.open, false, 'no box opens');
  assert.equal(blockPosts(env).length, 0, 'nothing is sent');
});

test('a name that clears brings Block back at the next poll', async () => {
  const env = load();
  const page = await start(env, [cm(1, { author: 'New member', author_kind: 'new_member' })]);
  assert.equal(blockButton(page.items()[0]).hidden, true);
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(1)]));
  await flush();
  assert.equal(blockButton(page.items()[0]).hidden, false);
  assert.equal(blockButton(page.items()[0]).getAttribute('aria-label'), 'Block ravi.k');
});

test('Block success: one toast, the list reloaded, focus on the heading', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const toasts = recordToasts(env);
  const page = await start(env, [cm(1), cm(2, { author: 'meena.r' })]);
  const [named] = page.items();
  blockButton(named).fire('click');
  boxes.block.yes.fire('click');
  assert.equal(blockButton(named).getAttribute('aria-disabled'), 'true', 'busy while it runs');
  const before = env.fetches.length;
  lastFetch(env).reply(204, null);
  await flush();
  assert.deepEqual(toasts, ['ravi.k is blocked.']);
  assert.equal(env.fetches.length, before + 1, 'the list is asked again at once');
  assert.equal(lastFetch(env).opts.method, 'GET');
  assert.equal(env.doc.activeElement, find(page.view.el, 'comments__heading'));
  lastFetch(env).reply(200, body([cm(2, { author: 'meena.r' })]));
  await flush();
  assert.equal(page.items().length, 1, 'the blocked person left the list');
  assert.equal(wordsOf(page.items()[0]).textContent, 'The road is open');
  assert.equal(blockButton(page.items()[0]).getAttribute('aria-label'), 'Block meena.r');
  assert.equal(toasts.length, 1, 'one toast');
});

test('Block failures show the calm line and change nothing', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const toasts = recordToasts(env);
  const page = await start(env, [cm(1)]);
  const li = page.items()[0];
  const attempt = async (answer) => {
    blockButton(li).fire('click');
    boxes.block.yes.fire('click');
    const before = env.fetches.length;
    answer(lastFetch(env));
    await flush();
    return env.fetches.length - before;
  };
  assert.equal(await attempt((f) => f.reply(429, { error: { field: 'rate_people', message: 'Too fast. You can try again in 20 seconds.' } })), 0, 'no list read');
  assert.equal(actionError(li).hidden, false);
  assert.equal(actionError(li).textContent, 'Too fast. You can try again in 20 seconds.');
  assert.equal(env.doc.activeElement, blockButton(li), 'focus returns to Block');
  assert.equal(blockButton(li).hasAttribute('aria-disabled'), false);
  assert.equal(await attempt((f) => f.reply(500, null)), 0);
  assert.equal(actionError(li).textContent, "Couldn't save that. Try again.");
  assert.equal(await attempt((f) => f.fail(new Error('network'))), 0);
  assert.equal(actionError(li).textContent, "Couldn't save that. Try again.");
  assert.deepEqual(toasts, [], 'no toast on any failure');
  assert.equal(page.items().length, 1);
});

test('Block on a comment that is gone reloads and shows no toast', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const toasts = recordToasts(env);
  const page = await start(env, [cm(1)]);
  blockButton(page.items()[0]).fire('click');
  boxes.block.yes.fire('click');
  const before = env.fetches.length;
  lastFetch(env).reply(404, { error: { field: 'comment', message: 'This comment is no longer available.' } });
  await flush();
  assert.equal(env.fetches.length, before + 1, 'a list read follows');
  assert.equal(lastFetch(env).opts.method, 'GET');
  assert.deepEqual(toasts, []);
  // Review WR-03: the comment is gone, so focus goes to the heading, never to
  // a Block button the reload may take away.
  assert.equal(env.doc.activeElement, find(page.view.el, 'comments__heading'));
});

test('Block offline sends nothing', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const page = await start(env, [cm(1)]);
  const li = page.items()[0];
  env.sandbox.navigator.onLine = false;
  blockButton(li).fire('click');
  boxes.block.yes.fire('click');
  assert.equal(blockPosts(env).length, 0);
  assert.equal(actionError(li).textContent, "You're offline, so this wasn't saved. Try again when you have a signal.");
  assert.equal(env.doc.activeElement, blockButton(li));
});

test('Block with no box in the page asks with window.confirm', async () => {
  const env = load();
  const asked = [];
  env.doc.getElementById = () => null;
  env.sandbox.confirm =(question) => { asked.push(question); return true; };
  const page = await start(env, [cm(1)]);
  blockButton(page.items()[0]).fire('click');
  assert.deepEqual(asked, ['Block ravi.k?']);
  assert.equal(blockPosts(env).length, 1);
  assert.equal(lastFetch(env).address, '/api/comments/1/block-author');
});

test('a username written into the Block box is plain text', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const page = await start(env, [cm(1, { author: '<b>x</b>' })]);
  blockButton(page.items()[0]).fire('click');
  assert.equal(boxes.block.title.textContent, 'Block <b>x</b>?');
  assert.equal(blockButton(page.items()[0]).getAttribute('aria-label'), 'Block <b>x</b>');
});

// ---- Review WR-03 (2026-10-30): focus never lands on a gone or hidden Block ----

function heading(page) { return find(page.view.el, 'comments__heading'); }

test('WR-03: Cancel after the comment became a placeholder focuses the heading', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const page = await start(env, [cm(1), cm(2)]);
  const li = page.items()[0];
  blockButton(li).fire('click');
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(1, { hidden: true, author: '', words: '' }), cm(2)]));
  await flush();
  assert.equal(find(li, 'comment__meta').hidden, true, 'the row is a placeholder now');
  boxes.block.cancel.fire('click');
  assert.equal(env.doc.activeElement, heading(page), 'focus is on the heading');
  assert.equal(blockButton(li).hidden, true);
});

test('WR-03: Cancel after the comment left the list focuses the heading', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const page = await start(env, [cm(1), cm(2)]);
  const li = page.items()[0];
  blockButton(li).fire('click');
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(2)]));
  await flush();
  assert.equal(page.items().length, 1, 'the comment left the list');
  boxes.block.cancel.fire('click');
  assert.equal(env.doc.activeElement, heading(page));
});

test('WR-03: a failed Block on a comment that became a placeholder keeps Block hidden and focuses the heading', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const page = await start(env, [cm(1)]);
  const li = page.items()[0];
  // The block leaves at 25 s and the poll at 30 s, inside the block's 10 s limit.
  env.advance(25000);
  blockButton(li).fire('click');
  boxes.block.yes.fire('click');
  const sent = lastFetch(env);
  assert.equal(sent.opts.method, 'POST');
  env.advance(5000);
  const poll = lastFetch(env);
  assert.equal(poll.opts.method, 'GET');
  poll.reply(200, body([cm(1, { removed: true, author: '', words: '' })]));
  await flush();
  sent.reply(500, null);
  await flush();
  assert.equal(blockButton(li).hidden, true, 'writeActions never unhides Block in a placeholder');
  assert.equal(find(li, 'comment__meta').hidden, true);
  assert.equal(env.doc.activeElement, heading(page), 'focus is on the heading');
});

test('WR-03: an offline Block on a comment that left the list focuses the heading', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const page = await start(env, [cm(1), cm(2)]);
  const li = page.items()[0];
  blockButton(li).fire('click');
  env.advance(30000);
  lastFetch(env).reply(200, body([cm(2)]));
  await flush();
  env.sandbox.navigator.onLine = false;
  boxes.block.yes.fire('click');
  assert.equal(blockPosts(env).length, 0);
  assert.equal(env.doc.activeElement, heading(page));
});

// ---- Review WR-04 (2026-10-30): after a block no loaded page keeps their words ----

// blockOn presses Block on li, says yes and answers the block with 204.
async function blockOn(env, boxes, li) {
  blockButton(li).fire('click');
  boxes.block.yes.fire('click');
  const sent = lastFetch(env);
  assert.ok(/block-author$/.test(sent.address));
  sent.reply(204, null);
  await flush();
}

test('WR-04: a block on a middle page reads the list again from the first page', async () => {
  const env = load();
  const boxes = addBoxes(env);
  recordToasts(env);
  const page = await start(env, roots(1, 50), { more: true, next_after: 50, comment_count: 110 });
  const more = find(page.view.el, 'comments__more');
  more.fire('click');
  lastFetch(env).reply(200, body(roots(51, 100).map((c) => (c.id === 60 ? cm(60, { author: 'meena.r', words: 'c60' }) : c)), { more: true, next_after: 100, comment_count: 110 }));
  await flush();
  more.fire('click');
  lastFetch(env).reply(200, body(roots(101, 110), { more: false, next_after: 0, comment_count: 110 }));
  await flush();
  assert.equal(page.items().length, 110);
  await blockOn(env, boxes, page.items()[59]);
  const reload = lastFetch(env);
  assert.equal(reload.opts.method, 'GET');
  assert.ok(reload.address.endsWith('/comments'), 'the first page, it asked ' + reload.address);
  const sentBefore = env.fetches.length;
  reload.reply(200, body(roots(1, 50), { more: true, next_after: 50, comment_count: 109 }));
  await flush();
  assert.equal(env.fetches.length, sentBefore, 'no later page is read again by itself');
  const ids = shownIds(page);
  assert.equal(ids.length, 50, 'only the first page is on the list');
  assert.equal(ids.includes('c60'), false, 'the blocked words are gone');
  assert.equal(more.hidden, false, 'Show more is back for the rest');
  more.fire('click');
  assert.ok(lastFetch(env).address.endsWith('?after=50'), 'Show more starts again after the first page');
});

test('WR-04: a Show more answer already out when the block lands is ignored', async () => {
  const env = load();
  const boxes = addBoxes(env);
  recordToasts(env);
  const page = await start(env, roots(1, 50), { more: true, next_after: 50, comment_count: 70 });
  const more = find(page.view.el, 'comments__more');
  more.fire('click');
  const stale = lastFetch(env);
  assert.ok(stale.address.endsWith('?after=50'));
  await blockOn(env, boxes, page.items()[0]);
  const reload = lastFetch(env);
  assert.equal(reload.opts.method, 'GET');
  stale.reply(200, body(roots(51, 70), { more: false, next_after: 0, comment_count: 70 }));
  await flush();
  reload.reply(200, body(roots(2, 50), { more: true, next_after: 50, comment_count: 69 }));
  await flush();
  const ids = shownIds(page);
  assert.equal(ids.length, 49, 'the stale page was not written');
  assert.equal(ids.includes('c51'), false);
  assert.equal(more.hidden, false);
  assert.equal(more.hasAttribute('aria-disabled'), false, 'Show more can be pressed again');
  more.fire('click');
  assert.ok(lastFetch(env).address.endsWith('?after=50'));
});

test('WR-04: a failed Show more already out when the block lands says nothing', async () => {
  const env = load();
  const boxes = addBoxes(env);
  recordToasts(env);
  const page = await start(env, roots(1, 50), { more: true, next_after: 50, comment_count: 70 });
  find(page.view.el, 'comments__more').fire('click');
  const stale = lastFetch(env);
  await blockOn(env, boxes, page.items()[0]);
  const reload = lastFetch(env);
  reload.reply(200, body(roots(2, 50), { more: true, next_after: 50, comment_count: 69 }));
  await flush();
  stale.fail(new Error('network'));
  await flush();
  assert.equal(find(page.view.el, 'comments__more-error').hidden, true, 'no error for a read the block made stale');
});

test('WR-04: a reply read already out when the block lands is ignored', async () => {
  const env = load();
  const boxes = addBoxes(env);
  recordToasts(env);
  const page = await start(env, [
    cm(1, { author: 'meena.r', words: 'root', replies: replyRange(2, 101), reply_count: 102 }),
    cm(300, { author: 'ravi.k', words: 'ravi root' })
  ]);
  const [threadLi, raviLi] = page.items();
  find(threadLi, 'comment-thread__toggle').fire('click');
  find(threadLi, 'comment-thread__more').fire('click');
  const stale = lastFetch(env);
  assert.ok(stale.address.includes('replies_of=1'));
  await blockOn(env, boxes, raviLi);
  const reload = lastFetch(env);
  reload.reply(200, body([cm(1, { author: 'meena.r', words: 'root', replies: [], reply_count: 0 })]));
  await flush();
  stale.reply(200, body([cm(1, { author: 'meena.r', reply_count: 102, replies: [cm(102, { words: 'reply 102' })] })], { more: false, next_after: 102 }));
  await flush();
  const [li] = page.items();
  assert.equal(page.items().length, 1);
  assert.deepEqual(threadIds(li), [], 'no reply by the blocked person came back');
});

// Review IN-01 (2026-10-30): sendBlock has its own busy guard. The box here
// fires its close event later, as a browser may, so a second fast press of its
// Block button reaches sendBlock while the first request is out.
test('IN-01: a second Block press while sending sends nothing more', async () => {
  const env = load();
  const boxes = addBoxes(env);
  const b = boxes.block;
  const later = [];
  b.box.close = () => { if (b.box.open) { b.box.open = false; later.push(() => b.box.fire('close')); } };
  const page = await start(env, [cm(1)]);
  blockButton(page.items()[0]).fire('click');
  b.yes.fire('click');
  b.yes.fire('click');
  later.forEach((fn) => fn());
  assert.equal(blockPosts(env).length, 1, 'one request');
});
