// share_window_test.mjs - runs the real web/static/js/share.js in a Node
// sandbox and checks the Share window of Phase 8 plan 06 (COORD-08, D-04),
// which the Go source tests cannot:
//
//   - the window opens in the same turn as the tap, with the interim title and
//     the fixed line, a link skeleton and both buttons off, and starts one POST;
//   - the ready state writes the server's title, line and link, turns the
//     buttons on and holds no count and no claim;
//   - Copy link writes the link and reads Link copied for three seconds, or
//     selects the link when the clipboard is refused;
//   - the WhatsApp link is the wa.me chooser with the title and the link,
//     encoded, and never opens the window's close;
//   - every error has its sentence: no link yet, offline, rate limited (with
//     the server's own wait, Try again held back until it ends) and report gone;
//   - the X and Escape close it, a backdrop tap does not, focus goes to the
//     title and back to the opener, and the page lock is paired.
//
// Run it with: node --test web/share_window_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what share.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const shareSource = fs.readFileSync(path.join(here, 'static', 'js', 'share.js'), 'utf8');

const LINK = 'https://pinalert.example/r/AbCdEfGhIjKlMnOpQrStUv';
const TITLE = 'Flood report near Benz Circle';
const LINE = 'Open to see if this is still true right now';

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
  getClientRects() { return this.hidden ? [] : [{}]; }
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  all() { return this.children.flatMap((c) => [c, ...c.all()]); }
  querySelector(sel) {
    if (sel === '.modal-panel') { return this.all().find((e) => e.className.split(' ').includes('modal-panel')) || null; }
    return null;
  }
  fire(type, evt = {}) {
    const event = { type, target: this, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; }, ...evt };
    for (let n = this; n; n = n.parentNode) {
      (n.handlers[type] || []).slice().forEach((fn) => fn(event));
    }
    return event;
  }
}

const flush = async () => { for (let i = 0; i < 16; i += 1) { await Promise.resolve(); } };

// load builds the window markup of share_window.html.tmpl as fakes, then runs
// share.js. fetches keeps each request with a way to answer it. The clock is
// fake: advance(ms) runs the timers that fall due.
function load({ onLine = true, clipboard } = {}) {
  const doc = { activeElement: null, byId: {} };
  const el = (tag, id, cls) => { const e = new FakeEl(doc, tag, id); if (cls) { e.className = cls; } return e; };
  const page = el('main', 'page-behind');
  const backdrop = el('div', 'share-window', 'modal-backdrop share-window');
  backdrop.hidden = true;
  const panel = backdrop.appendChild(el('div', '', 'modal-panel share-window__panel'));
  const title = panel.appendChild(el('h2', 'share-window-title'));
  title.textContent = 'Share this report';
  const close = panel.appendChild(el('button', 'share-window-close'));
  const glyph = panel.appendChild(el('span', 'share-card-glyph', 'icon-glyph icon-glyph--other share-card__glyph'));
  const cardTitle = panel.appendChild(el('p', 'share-card-title'));
  const cardLine = panel.appendChild(el('p', 'share-card-line'));
  const link = panel.appendChild(el('p', 'share-card-link'));
  link.hidden = true;
  const bar = panel.appendChild(el('span', 'share-card-bar'));
  const brand = panel.appendChild(el('span', '', 'share-card__brand'));
  brand.textContent = 'Pinalert';
  const status = panel.appendChild(el('p', 'share-status'));
  const error = panel.appendChild(el('div', 'share-window-error', 'inline-error share-window__error'));
  error.hidden = true;
  const errorText = error.appendChild(el('p', 'share-window-error-text'));
  const retry = error.appendChild(el('button', 'share-window-retry'));
  retry.textContent = 'Try again';
  const whatsapp = panel.appendChild(el('a', 'share-window-whatsapp', 'btn btn--primary btn--off share-window__whatsapp'));
  whatsapp.textContent = 'WhatsApp';
  whatsapp.setAttribute('aria-disabled', 'true');
  const copy = panel.appendChild(el('button', 'share-window-copy', 'btn btn--off share-window__copy'));
  copy.setAttribute('aria-disabled', 'true');
  const copyGlyph = copy.appendChild(el('span', 'share-window-copy-glyph', 'app-icon share-window__glyph share-window__glyph--link'));
  const copyWord = copy.appendChild(el('span', 'share-window-copy-word'));
  copyWord.textContent = 'Copy link';
  const opener = el('button', 'share-opener', 'report-page__share');

  const fetches = [];
  const locks = [];
  const aborts = [];
  const writes = [];
  const shareCalls = [];
  const timers = [];
  let now = 0;
  let nextTimer = 1;
  const sandbox = {};
  sandbox.window = sandbox;
  sandbox.setTimeout = (fn, ms) => { const id = nextTimer; nextTimer += 1; timers.push({ id, at: now + ms, fn }); return id; };
  sandbox.clearTimeout = (id) => { const at = timers.findIndex((t) => t.id === id); if (at !== -1) { timers.splice(at, 1); } };
  sandbox.document = {
    get activeElement() { return doc.activeElement; },
    getElementById: (id) => doc.byId[id] || null,
    createRange: () => ({ selectNodeContents(node) { this.node = node; } }),
  };
  const selection = { range: null, removeAllRanges() { this.range = null; }, addRange(r) { this.range = r; } };
  sandbox.getSelection = () => selection;
  sandbox.navigator = { onLine, share: () => { shareCalls.push('share'); }, canShare: () => { shareCalls.push('canShare'); return true; } };
  sandbox.navigator.clipboard = clipboard === undefined
    ? { writeText: (text) => { writes.push(text); return Promise.resolve(); } }
    : clipboard;
  sandbox.fetch = (u, opts) => new Promise((resolve, reject) => { fetches.push({ url: u, opts, resolve, reject }); });
  sandbox.Pinalert = {
    setText(node, s) { node.textContent = s; },
    lockPage(b) { locks.push(['lock', b.id]); },
    unlockPage() { locks.push(['unlock']); },
    iconClass: (c) => `icon-glyph icon-glyph--${c}`,
    categoryLabel: (c) => ({ flood: 'Flood', fire: 'Fire' }[c] || 'Other'),
    requestTimeout(ms) {
      let reject;
      const expired = new Promise((_, rej) => { reject = rej; });
      expired.catch(() => {});
      const id = sandbox.setTimeout(() => reject(Object.assign(new Error('limit'), { name: 'TimeoutError' })), ms);
      return { signal: null, controller: { abort() { aborts.push(ms); } }, expired, clear() { sandbox.clearTimeout(id); } };
    },
  };
  vm.runInNewContext(shareSource, sandbox);

  const advance = async (ms) => {
    now += ms;
    for (;;) {
      const due = timers.filter((t) => t.at <= now).sort((a, b) => a.at - b.at)[0];
      if (!due) { break; }
      timers.splice(timers.indexOf(due), 1);
      due.fn();
    }
    await flush();
  };
  const answer = async (f, status, body, headers = {}) => {
    f.resolve({ status, json: async () => body, headers: { get: (n) => headers[n] ?? null } });
    await flush();
  };
  const ok = { url: LINK, title: TITLE, line: LINE };
  return {
    api: sandbox.window.PinalertShare, sandbox, doc, backdrop, panel, title, close, glyph, cardTitle, cardLine, link, bar, status,
    error, errorText, retry, whatsapp, copy, copyGlyph, copyWord, opener, page, brand,
    fetches, locks, aborts, writes, shareCalls, timers, selection, advance, answer, ok,
  };
}

const openIt = (w, over = {}) => w.api.open({ reportId: 41, category: 'flood', opener: w.opener, ...over });
const isOff = (e) => e.getAttribute('aria-disabled') === 'true';

test('opens at once with the interim title', async () => {
  const w = load();
  w.opener.focus();
  openIt(w);
  // Same turn, before any answer: the window is shown, locked, and focused.
  assert.equal(w.backdrop.hidden, false);
  assert.deepEqual(w.locks, [['lock', 'share-window']]);
  assert.equal(w.doc.activeElement, w.title);
  assert.equal(w.cardTitle.textContent, 'Flood report');
  assert.equal(w.cardLine.textContent, LINE);
  assert.equal(w.link.hidden, true);
  assert.equal(w.bar.hidden, false);
  assert.ok(isOff(w.whatsapp) && isOff(w.copy), 'both buttons start off');
  assert.equal(w.whatsapp.getAttribute('href'), null);
  assert.equal(w.error.hidden, true);
  // One POST, no body, to this report's share route, with the session cookie.
  assert.equal(w.fetches.length, 1);
  assert.equal(w.fetches[0].url, '/api/reports/41/share');
  assert.equal(w.fetches[0].opts.method, 'POST');
  assert.equal(w.fetches[0].opts.credentials, 'same-origin');
  assert.equal(Object.hasOwn(w.fetches[0].opts, 'body'), false);
  // A glyph class comes from the validated category helper.
  assert.equal(w.glyph.className, 'icon-glyph icon-glyph--flood share-card__glyph');
});

test('ready state', async () => {
  const w = load();
  openIt(w);
  await w.answer(w.fetches[0], 200, w.ok);
  assert.equal(w.cardTitle.textContent, TITLE);
  assert.equal(w.cardLine.textContent, LINE);
  assert.equal(w.link.textContent, LINK);
  assert.equal(w.link.hidden, false);
  assert.equal(w.bar.hidden, true);
  assert.equal(w.error.hidden, true);
  assert.ok(!isOff(w.whatsapp) && !isOff(w.copy), 'both buttons are on');
});

test('no count in the window', async () => {
  const w = load();
  openIt(w);
  const words = () => w.panel.all().filter((e) => e !== w.link && e !== w.error && !w.error.contains(e)).map((e) => e.own).join(' ');
  // Opening state and ready state: no digit anywhere but the link, and never
  // the word verified.
  assert.doesNotMatch(words(), /[0-9]/);
  assert.doesNotMatch(words(), /verified/i);
  await w.answer(w.fetches[0], 200, w.ok);
  assert.doesNotMatch(words(), /[0-9]/);
  assert.doesNotMatch(words(), /verified/i);
  assert.doesNotMatch(w.status.textContent, /[0-9]/);
  // The script itself holds no count or claim word for the card.
  assert.doesNotMatch(shareSource.replace(/\/\/.*$/gm, ''), /verified|confirmed by|people nearby/i);
});

test('copy link', async () => {
  const w = load();
  openIt(w);
  await w.answer(w.fetches[0], 200, w.ok);
  w.copy.fire('click');
  await flush();
  assert.deepEqual(w.writes, [LINK]);
  assert.equal(w.copyWord.textContent, 'Link copied');
  assert.match(w.copyGlyph.className, /app-icon--check/);
  assert.equal(w.status.textContent, 'Link copied.');
  await w.advance(2999);
  assert.equal(w.copyWord.textContent, 'Link copied');
  await w.advance(1);
  assert.equal(w.copyWord.textContent, 'Copy link');
  assert.match(w.copyGlyph.className, /share-window__glyph--link/);
  // A press on a button that is still off does nothing.
  const off = load();
  openIt(off);
  off.copy.fire('click');
  await flush();
  assert.deepEqual(off.writes, []);
  assert.equal(off.copyWord.textContent, 'Copy link');
});

test('copy refused selects the link', async () => {
  for (const clipboard of [{ writeText: () => Promise.reject(new Error('denied')) }, { writeText: () => { throw new Error('sync'); } }, null]) {
    const w = load({ clipboard });
    openIt(w);
    await w.answer(w.fetches[0], 200, w.ok);
    w.copy.fire('click');
    await flush();
    assert.equal(w.status.textContent, "Couldn't copy. Select the link and copy it.");
    assert.ok(w.selection.range, 'a selection was made');
    assert.equal(w.selection.range.node, w.link);
    assert.equal(w.copyWord.textContent, 'Copy link');
  }
});

test('whatsapp link', async () => {
  const w = load();
  openIt(w);
  // Off and with no address, a press does nothing.
  const early = w.whatsapp.fire('click');
  assert.equal(early.defaultPrevented, true);
  await w.answer(w.fetches[0], 200, w.ok);
  assert.equal(w.whatsapp.getAttribute('href'), `https://wa.me/?text=${encodeURIComponent(`${TITLE} ${LINK}`)}`);
  // A ready tap is left to the browser and the window stays open.
  const ready = w.whatsapp.fire('click');
  assert.equal(ready.defaultPrevented, false);
  assert.equal(w.backdrop.hidden, false);
  // A title with an ampersand or a hash cannot break out of the text parameter.
  const odd = load();
  openIt(odd);
  await odd.answer(odd.fetches[0], 200, { url: LINK, title: 'Fire & smoke #1 report', line: LINE });
  const href = odd.whatsapp.getAttribute('href');
  assert.equal(href.indexOf('&'), -1);
  assert.equal(href.indexOf('#'), -1);
  assert.equal(decodeURIComponent(href.slice('https://wa.me/?text='.length)), `Fire & smoke #1 report ${LINK}`);
});

test('error and retry', async () => {
  const w = load();
  openIt(w);
  await w.answer(w.fetches[0], 500, {});
  assert.equal(w.error.hidden, false);
  assert.equal(w.errorText.textContent, "Couldn't make a link. Try again.");
  assert.equal(w.error.getAttribute('role'), 'alert');
  assert.equal(w.retry.hidden, false);
  assert.ok(isOff(w.whatsapp) && isOff(w.copy));
  assert.equal(w.link.hidden, true);
  assert.equal(w.bar.hidden, true);
  // Try again refetches, shows the skeleton again and keeps focus in the panel.
  w.retry.focus();
  w.retry.fire('click');
  assert.equal(w.fetches.length, 2);
  assert.equal(w.error.hidden, true);
  assert.equal(w.bar.hidden, false);
  assert.ok(w.panel.contains(w.doc.activeElement));
  await w.answer(w.fetches[1], 200, w.ok);
  assert.ok(!isOff(w.copy));
  assert.equal(w.link.textContent, LINK);
  // A timeout shows the same sentence, and a late answer is then dropped.
  const t = load();
  openIt(t);
  await t.advance(10000);
  assert.equal(t.errorText.textContent, "Couldn't make a link. Try again.");
  await t.answer(t.fetches[0], 200, t.ok);
  assert.equal(t.error.hidden, false, 'the late answer after the limit is not painted');
  // A 200 with no link is a failed answer too.
  const e = load();
  openIt(e);
  await e.answer(e.fetches[0], 200, {});
  assert.equal(e.errorText.textContent, "Couldn't make a link. Try again.");
});

test('offline', async () => {
  const w = load({ onLine: false });
  openIt(w);
  w.fetches[0].reject(new TypeError('Failed to fetch'));
  await flush();
  assert.equal(w.errorText.textContent, "You're offline, so there is no link yet. Try again when you have a signal.");
  assert.equal(w.retry.hidden, false);
  assert.ok(isOff(w.whatsapp) && isOff(w.copy));
  // A rejection while online is the generic sentence.
  const on = load({ onLine: true });
  openIt(on);
  on.fetches[0].reject(new TypeError('Failed to fetch'));
  await flush();
  assert.equal(on.errorText.textContent, "Couldn't make a link. Try again.");
});

test('rate limited', async () => {
  const w = load();
  openIt(w);
  const sentence = "You've shared several reports in a short time. You can share again in 1 minute.";
  await w.answer(w.fetches[0], 429, { error: { field: 'rate_share', message: sentence }, retry_after_seconds: 60 }, { 'Retry-After': '60' });
  assert.equal(w.errorText.textContent, sentence);
  assert.equal(w.retry.hidden, true, 'no Try again until the wait ends');
  assert.ok(isOff(w.whatsapp) && isOff(w.copy));
  await w.advance(59999);
  assert.equal(w.retry.hidden, true);
  await w.advance(1);
  assert.equal(w.retry.hidden, false);
  // The wait comes from the header when the body has none.
  const h = load();
  openIt(h);
  await h.answer(h.fetches[0], 429, { error: { message: sentence } }, { 'Retry-After': '30' });
  assert.equal(h.retry.hidden, true);
  await h.advance(30000);
  assert.equal(h.retry.hidden, false);
  // Closing clears the wait timer.
  const c = load();
  openIt(c);
  await c.answer(c.fetches[0], 429, { error: { message: sentence }, retry_after_seconds: 5 });
  c.close.fire('click');
  assert.equal(c.timers.length, 0);
});

test('report gone', async () => {
  const w = load();
  openIt(w);
  await w.answer(w.fetches[0], 404, { error: { field: 'report', message: 'Not found' } });
  assert.equal(w.errorText.textContent, 'This report is no longer available.');
  assert.equal(w.retry.hidden, true);
  assert.ok(isOff(w.whatsapp) && isOff(w.copy));
  assert.equal(w.whatsapp.getAttribute('href'), null);
});

test('close returns focus', async () => {
  const w = load();
  w.opener.focus();
  openIt(w);
  assert.equal(w.doc.activeElement, w.title);
  // A tap on the backdrop does not close it.
  w.backdrop.fire('click');
  assert.equal(w.backdrop.hidden, false);
  // The X closes: page unlocked, focus back on the opener.
  w.close.fire('click');
  assert.equal(w.backdrop.hidden, true);
  assert.deepEqual(w.locks, [['lock', 'share-window'], ['unlock']]);
  assert.equal(w.doc.activeElement, w.opener);
  // Escape closes too, from a focused control, and does not leave the lock.
  openIt(w);
  w.copy.focus();
  const esc = w.copy.fire('keydown', { key: 'Escape' });
  assert.equal(esc.defaultPrevented, true);
  assert.equal(w.backdrop.hidden, true);
  assert.equal(w.doc.activeElement, w.opener);
  assert.deepEqual(w.locks.slice(-2), [['lock', 'share-window'], ['unlock']]);
  // A closed window ignores a late answer.
  const late = w.fetches[1];
  await w.answer(late, 200, w.ok);
  assert.equal(w.backdrop.hidden, true);
  assert.equal(w.cardTitle.textContent, 'Flood report');
});

test('a second open replaces the report and ends the first request', async () => {
  const w = load();
  openIt(w);
  const first = w.fetches[0];
  openIt(w, { reportId: 52, category: 'fire' });
  assert.equal(w.aborts.length >= 1, true, 'the first request was ended');
  assert.equal(w.fetches.length, 2);
  assert.equal(w.fetches[1].url, '/api/reports/52/share');
  assert.equal(w.cardTitle.textContent, 'Fire report');
  assert.equal(w.glyph.className, 'icon-glyph icon-glyph--fire share-card__glyph');
  // The window is locked once, not twice, and the old answer is dropped.
  assert.equal(w.locks.filter((l) => l[0] === 'lock').length, 1);
  await w.answer(first, 200, w.ok);
  assert.equal(w.cardTitle.textContent, 'Fire report');
  assert.ok(isOff(w.copy));
  await w.answer(w.fetches[1], 200, { url: `${LINK}x`, title: 'Fire report', line: LINE });
  assert.ok(!isOff(w.copy));
  // Reopening after a ready state starts clean again.
  openIt(w, { reportId: 41, category: 'flood' });
  assert.ok(isOff(w.copy) && isOff(w.whatsapp));
  assert.equal(w.whatsapp.getAttribute('href'), null);
  assert.equal(w.link.hidden, true);
});

test('Tab stays inside the window', async () => {
  const w = load();
  openIt(w);
  // Before the link is known: the X and Copy link (a link with no address
  // cannot take focus).
  w.close.focus();
  const first = w.close.fire('keydown', { key: 'Tab' });
  assert.equal(first.defaultPrevented, true);
  assert.equal(w.doc.activeElement, w.copy);
  w.copy.fire('keydown', { key: 'Tab' });
  assert.equal(w.doc.activeElement, w.close, 'wraps from the last stop to the first');
  w.close.fire('keydown', { key: 'Tab', shiftKey: true });
  assert.equal(w.doc.activeElement, w.copy, 'Shift+Tab wraps backwards');
  // Once ready the WhatsApp link joins, between the X and Copy link.
  await w.answer(w.fetches[0], 200, w.ok);
  w.close.focus();
  w.close.fire('keydown', { key: 'Tab' });
  assert.equal(w.doc.activeElement, w.whatsapp);
  w.whatsapp.fire('keydown', { key: 'Tab' });
  assert.equal(w.doc.activeElement, w.copy);
  // Focus that escaped to the page behind comes back to the X.
  w.page.focus();
  w.backdrop.fire('keydown', { key: 'Tab' });
  assert.equal(w.doc.activeElement, w.close);
});

test('the browser share sheet is never called', async () => {
  const w = load();
  openIt(w);
  await w.answer(w.fetches[0], 200, w.ok);
  w.copy.fire('click');
  w.whatsapp.fire('click');
  await flush();
  assert.deepEqual(w.shareCalls, []);
  assert.doesNotMatch(shareSource.replace(/\/\/.*$/gm, ''), /navigator\.(share|canShare)/);
});
