// share.js owns the Share window that the Share button of an opened report
// opens (Phase 8 plan 06, COORD-08, D-04).
//
// What people receive is exactly what the window shows: the title, the fixed
// line and the link. The title and the fixed line come from the server in the
// same call that makes the link, and they are the same strings as the public
// page's og tags, so the card is a true preview of a forward. The window never
// shows a count, a state word, a severity colour or a claim, because a forward
// can never be updated and must not carry a stale fact. The page it links to
// answers the question live.
//
// The window opens at once on the tap and fetches the link while it is open;
// WhatsApp and Copy link stay off until the link is known. WhatsApp is a plain
// link to WhatsApp's own chooser with the title and the link, opened in a new
// tab with no opener. The browser's own share sheet is never called: nothing
// here runs a share outside a tap on one of the window's own buttons. Copy
// link writes the link to the clipboard, or selects it when the clipboard is
// refused. The window is the app's other windows again: the X and Escape close
// it, a tap on the backdrop does not, focus goes to the title and back to the
// button that opened it, and the page behind stays still (Pinalert.lockPage).
//
// Every string reaches the page through Pinalert.setText or setAttribute, and
// the window is static template markup, so nothing here parses markup. The
// server's own sentences (the wait of a rate limit) are shown as text.
window.PinalertShare = (function () {
  'use strict';

  var SHARE_TIMEOUT_MS = 10000;
  var COPIED_MS = 3000;
  var MS_PER_SECOND = 1000;

  var WHATSAPP_BASE = 'https://wa.me/?text=';
  var FIXED_LINE = 'Open to see if this is still true right now';
  var REPORT_WORD = ' report';
  var COPY_LABEL = 'Copy link';
  var COPIED_LABEL = 'Link copied';
  var COPIED_STATUS = 'Link copied.';
  var COPY_REFUSED_STATUS = "Couldn't copy. Select the link and copy it.";
  var ERROR_MESSAGE = "Couldn't make a link. Try again.";
  var OFFLINE_MESSAGE = "You're offline, so there is no link yet. Try again when you have a signal.";
  var GONE_MESSAGE = 'This report is no longer available.';
  var STATUS_GONE = 404;
  var STATUS_RATE = 429;
  var STATUS_OK = 200;

  var GLYPH_BASE = 'app-icon share-window__glyph';
  var GLYPH_LINK = GLYPH_BASE + ' share-window__glyph--link';
  var GLYPH_CHECK = GLYPH_BASE + ' app-icon--check';

  var refs = null;
  // The open window: reportId, category, opener, the known url and title.
  var current = null;
  // Counts every request and every close, so a late answer for a report the
  // window has moved on from is dropped.
  var sequence = 0;
  var limit = null;
  var copiedTimer = null;
  var waitTimer = null;

  function isShown(el) {
    return !!el && el.isConnected && !el.hidden && el.getClientRects().length > 0;
  }

  // windowRefs finds the static window once and binds its listeners once.
  function windowRefs() {
    if (refs) {
      return refs;
    }
    var backdrop = document.getElementById('share-window');
    if (!backdrop) {
      return null;
    }
    refs = {
      backdrop: backdrop,
      panel: backdrop.querySelector('.modal-panel'),
      title: document.getElementById('share-window-title'),
      close: document.getElementById('share-window-close'),
      glyph: document.getElementById('share-card-glyph'),
      cardTitle: document.getElementById('share-card-title'),
      cardLine: document.getElementById('share-card-line'),
      link: document.getElementById('share-card-link'),
      bar: document.getElementById('share-card-bar'),
      status: document.getElementById('share-status'),
      error: document.getElementById('share-window-error'),
      errorText: document.getElementById('share-window-error-text'),
      retry: document.getElementById('share-window-retry'),
      whatsapp: document.getElementById('share-window-whatsapp'),
      copy: document.getElementById('share-window-copy'),
      copyGlyph: document.getElementById('share-window-copy-glyph'),
      copyWord: document.getElementById('share-window-copy-word')
    };
    refs.close.addEventListener('click', closeWindow);
    refs.retry.addEventListener('click', function () {
      if (current) {
        requestLink();
      }
    });
    refs.copy.addEventListener('click', onCopyClick);
    refs.whatsapp.addEventListener('click', function (evt) {
      // A link with no address yet must do nothing. A ready link is left alone,
      // so the browser opens it in its new tab, and the window stays open.
      if (refs.whatsapp.getAttribute('aria-disabled') === 'true') {
        evt.preventDefault();
      }
    });
    backdrop.addEventListener('keydown', onWindowKeydown);
    return refs;
  }

  function setOff(el, off) {
    if (off) {
      el.setAttribute('aria-disabled', 'true');
    } else {
      el.removeAttribute('aria-disabled');
    }
  }

  function clearCopiedTimer() {
    if (copiedTimer !== null) {
      window.clearTimeout(copiedTimer);
      copiedTimer = null;
    }
  }

  function clearWaitTimer() {
    if (waitTimer !== null) {
      window.clearTimeout(waitTimer);
      waitTimer = null;
    }
  }

  function endRequest() {
    sequence += 1;
    if (limit !== null) {
      limit.clear();
      if (limit.controller) {
        limit.controller.abort();
      }
      limit = null;
    }
  }

  function showCopyLabel() {
    Pinalert.setText(refs.copyWord, COPY_LABEL);
    refs.copyGlyph.className = GLYPH_LINK;
  }

  // resetWindow puts the window back to its opening state for one report: the
  // interim title, the fixed line, the link as a skeleton bar, both buttons
  // off, no error and no status.
  function resetWindow() {
    var r = refs;
    clearCopiedTimer();
    clearWaitTimer();
    current.url = '';
    current.title = '';
    r.glyph.className = Pinalert.iconClass(current.category) + ' share-card__glyph';
    Pinalert.setText(r.cardTitle, Pinalert.categoryLabel(current.category) + REPORT_WORD);
    Pinalert.setText(r.cardLine, FIXED_LINE);
    Pinalert.setText(r.link, '');
    r.link.hidden = true;
    r.bar.hidden = false;
    hideError();
    Pinalert.setText(r.status, '');
    r.whatsapp.removeAttribute('href');
    setOff(r.whatsapp, true);
    setOff(r.copy, true);
    showCopyLabel();
  }

  // open shows the window in the same turn as the tap, then asks the server for
  // the link. A second open while it is open replaces the report and ends the
  // first request.
  function open(opts) {
    var r = windowRefs();
    if (!r || !opts || opts.reportId === undefined || opts.reportId === null) {
      return;
    }
    var already = current !== null;
    endRequest();
    current = {
      reportId: String(opts.reportId),
      category: opts.category,
      opener: opts.opener || (already ? current.opener : null),
      url: '',
      title: ''
    };
    resetWindow();
    if (!already) {
      r.backdrop.hidden = false;
      Pinalert.lockPage(r.backdrop);
    }
    r.title.focus();
    requestLink();
  }

  // closeWindow unlocks the page before it returns focus, because an inert
  // element cannot take focus.
  function closeWindow() {
    if (!current) {
      return;
    }
    var state = current;
    current = null;
    endRequest();
    clearCopiedTimer();
    clearWaitTimer();
    refs.backdrop.hidden = true;
    Pinalert.unlockPage();
    returnFocus(state);
  }

  function returnFocus(state) {
    if (isShown(state.opener)) {
      state.opener.focus();
    }
  }

  // requestLink makes the one POST that makes the link. It uses the shared time
  // limit of app.js (Pinalert.requestTimeout): the page's own timer raced
  // against the post and its body read, so a post whose aborted request never
  // ends still shows the error at SHARE_TIMEOUT_MS. The post has no body and
  // the browser sends the session cookie itself.
  function requestLink() {
    endRequest();
    var mine = sequence;
    hideError();
    refs.link.hidden = true;
    refs.bar.hidden = false;
    // The Try again button may be the focused one and is about to be hidden.
    if (!refs.panel.contains(document.activeElement) || document.activeElement === refs.retry) {
      refs.title.focus();
    }
    var request = Pinalert.requestTimeout(SHARE_TIMEOUT_MS, 'share');
    limit = request;
    Promise.race([fetch('/api/reports/' + encodeURIComponent(current.reportId) + '/share', {
      method: 'POST',
      credentials: 'same-origin',
      signal: request.signal
    }), request.expired]).then(function (res) {
      return Promise.race([res.json(), request.expired]).catch(function () {
        return {};
      }).then(function (body) {
        request.clear();
        if (mine === sequence) {
          handleAnswer(res.status, body, res.headers);
        }
      });
    }, function () {
      request.clear();
      if (mine !== sequence) {
        return;
      }
      showError(navigator.onLine === false ? OFFLINE_MESSAGE : ERROR_MESSAGE, true);
    });
  }

  function handleAnswer(status, body, headers) {
    if (status === STATUS_OK) {
      showReady(body);
      return;
    }
    if (status === STATUS_GONE) {
      showError(GONE_MESSAGE, false);
      return;
    }
    if (status === STATUS_RATE) {
      showRateLimited(body, headers);
      return;
    }
    showError(ERROR_MESSAGE, true);
  }

  // showReady writes the server's title, fixed line and link into the card and
  // turns both buttons on. An answer with no link is a failed answer.
  function showReady(body) {
    if (!body || typeof body.url !== 'string' || body.url === '') {
      showError(ERROR_MESSAGE, true);
      return;
    }
    var title = typeof body.title === 'string' && body.title !== '' ? body.title : refs.cardTitle.textContent;
    var line = typeof body.line === 'string' && body.line !== '' ? body.line : FIXED_LINE;
    current.url = body.url;
    current.title = title;
    Pinalert.setText(refs.cardTitle, title);
    Pinalert.setText(refs.cardLine, line);
    Pinalert.setText(refs.link, body.url);
    refs.link.hidden = false;
    refs.bar.hidden = true;
    refs.whatsapp.setAttribute('href', WHATSAPP_BASE + encodeURIComponent(title + ' ' + body.url));
    setOff(refs.whatsapp, false);
    setOff(refs.copy, false);
  }

  // showRateLimited shows the server's own wait sentence as text and keeps Try
  // again away until the wait has passed.
  function showRateLimited(body, headers) {
    var message = ERROR_MESSAGE;
    if (body && body.error && typeof body.error.message === 'string' && body.error.message !== '') {
      message = body.error.message;
    }
    var seconds = 0;
    if (body && typeof body.retry_after_seconds === 'number' && body.retry_after_seconds > 0) {
      seconds = body.retry_after_seconds;
    } else if (headers && typeof headers.get === 'function') {
      seconds = parseInt(headers.get('Retry-After'), 10);
    }
    if (!(seconds > 0)) {
      showError(message, true);
      return;
    }
    showError(message, false);
    var mine = sequence;
    waitTimer = window.setTimeout(function () {
      waitTimer = null;
      if (current && mine === sequence) {
        refs.retry.hidden = false;
      }
    }, seconds * MS_PER_SECOND);
  }

  // showError replaces the link line with the sentence. The role is set here
  // and removed when the line is hidden, so only a shown failure is announced.
  // Both buttons stay off.
  function showError(message, withRetry) {
    refs.link.hidden = true;
    refs.bar.hidden = true;
    Pinalert.setText(refs.errorText, message);
    refs.error.setAttribute('role', 'alert');
    refs.error.hidden = false;
    refs.retry.hidden = !withRetry;
    setOff(refs.whatsapp, true);
    setOff(refs.copy, true);
    refs.whatsapp.removeAttribute('href');
    // A button that was pressed can be the one that just left the page; keep
    // focus inside the panel so Escape and Tab still reach the window.
    if (!refs.panel.contains(document.activeElement)) {
      refs.title.focus();
    }
  }

  function hideError() {
    clearWaitTimer();
    refs.error.hidden = true;
    refs.error.removeAttribute('role');
    Pinalert.setText(refs.errorText, '');
    refs.retry.hidden = false;
  }

  // onCopyClick copies the link from the tap itself. A press on a button that
  // is still off does nothing.
  function onCopyClick() {
    if (!current || refs.copy.getAttribute('aria-disabled') === 'true' || current.url === '') {
      return;
    }
    var mine = sequence;
    var url = current.url;
    var written;
    if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      try {
        written = Promise.resolve(navigator.clipboard.writeText(url));
      } catch (err) {
        written = Promise.reject(err);
      }
    } else {
      written = Promise.reject(new Error('no clipboard'));
    }
    written.then(function () {
      if (current && mine === sequence) {
        showCopied();
      }
    }, function () {
      if (current && mine === sequence) {
        selectLink();
        Pinalert.setText(refs.status, COPY_REFUSED_STATUS);
      }
    });
  }

  function showCopied() {
    clearCopiedTimer();
    Pinalert.setText(refs.copyWord, COPIED_LABEL);
    refs.copyGlyph.className = GLYPH_CHECK;
    Pinalert.setText(refs.status, COPIED_STATUS);
    copiedTimer = window.setTimeout(function () {
      copiedTimer = null;
      if (refs) {
        showCopyLabel();
      }
    }, COPIED_MS);
  }

  // selectLink selects the words of the link line, so a refused clipboard
  // leaves the person one gesture from copying it by hand.
  function selectLink() {
    var range = document.createRange();
    range.selectNodeContents(refs.link);
    var selection = window.getSelection();
    if (selection) {
      selection.removeAllRanges();
      selection.addRange(range);
    }
  }

  // focusableInPanel lists the Tab stops in order: the X, the WhatsApp link
  // once it has an address (a link with none cannot take focus), Copy link
  // and Try again when it is shown. The trap moves focus one stop at a time
  // itself, because Safari's default Tab skips buttons and links and let focus
  // fall to the locked page behind the window.
  function focusableInPanel() {
    var list = [refs.close];
    if (refs.whatsapp.getAttribute('href')) {
      list.push(refs.whatsapp);
    }
    list.push(refs.copy);
    if (isShown(refs.retry) && isShown(refs.error)) {
      list.push(refs.retry);
    }
    return list;
  }

  function onWindowKeydown(evt) {
    if (evt.key === 'Escape') {
      evt.stopPropagation();
      evt.preventDefault();
      closeWindow();
      return;
    }
    if (evt.key !== 'Tab') {
      return;
    }
    var list = focusableInPanel();
    var firstEl = list[0];
    var lastEl = list[list.length - 1];
    var active = document.activeElement;
    evt.preventDefault();
    if (!refs.panel.contains(active)) {
      firstEl.focus();
      return;
    }
    var at = list.indexOf(active);
    if (evt.shiftKey) {
      (at <= 0 ? lastEl : list[at - 1]).focus();
    } else {
      (at === -1 || at === list.length - 1 ? firstEl : list[at + 1]).focus();
    }
  }

  return {
    open: open
  };
}());
