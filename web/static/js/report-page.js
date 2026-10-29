// report-page.js is the body of the report page: the Share button, the history
// called What happened so far, and an empty mount point for the comments. It is
// built once per report card and refreshed in place, and it is mount agnostic:
// it holds no breakpoint test, no panel width and no host class, because
// placement belongs to the host (map.js today, the laptop panel later; D-17,
// D-18, UI-SPEC Surface 1). It holds no trust rule and no threshold either: the
// trust block above it is trust.js's own, drawn by the card, and this file only
// reads the numbers the feed sends to notice that the history is out of date
// (A-P8-7). Comments mount into report-page__comments (comments.js, plan
// 08-16), which opens and closes with the page and polls on its own timer.
//
// The history comes from GET /api/reports/{id}/timeline. The server writes
// every sentence, so each one is shown as text, never as markup. The page
// fetches only while a host shows it and the report is open, once on open,
// once more when the trust numbers, the visibility, the view or the reader's
// own vote change, and every TIMELINE_POLL_MS while the tab is visible. A
// refresh writes into the nodes it already has: it never rebuilds the page,
// never moves focus and never replaces the comments mount point (research
// Pitfall 6).
//
// Every number in this file is a named constant on its own line below. The
// history is a record the server built; nothing here compares a count with a
// limit.
window.PinalertReportPage = (function () {
  'use strict';

  var TIMELINE_POLL_MS = 30000;
  var TIMELINE_TIMEOUT_MS = 10000;
  var JUST_NOW_MS = 60000;
  var SKELETON_ROWS = 3;
  var HALF_DAY_HOURS = 12;
  var TWO_DIGITS = 10;
  var STATUS_GONE = 404;

  var HISTORY_TITLE = 'What happened so far';
  var ERROR_TITLE = "Couldn't load the history";
  var ERROR_LINE = 'Check your connection and try again.';
  var OFFLINE_LINE = "You're offline, so the history can't load. Try again when you have a signal.";
  var RETRY_LABEL = 'Try again';
  var SHARE_LABEL = 'Share';
  var NOW_WORD = 'Now';
  var WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
  var TONES = ['good', 'neutral', 'bad'];

  var headingSeq = 0;

  function pad(n) {
    return (n < TWO_DIGITS ? '0' : '') + n;
  }

  function clockWords(at) {
    var hours = at.getHours();
    var shown = hours % HALF_DAY_HOURS;
    if (shown === 0) {
      shown = HALF_DAY_HOURS;
    }
    return shown + ':' + pad(at.getMinutes()) + ' ' + (hours >= HALF_DAY_HOURS ? 'pm' : 'am');
  }

  function sameDay(a, b) {
    return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
  }

  // formatTimelineTime builds the words of the time column from an RFC 3339
  // time: Now for the reader's own line under a minute old, the clock time
  // ("2:10 pm") on the same day and the short weekday and the clock time
  // ("Mon 2:10 pm") on another day. It reads getHours, getMinutes and a fixed
  // weekday list, so the words do not depend on the browser's locale. now may
  // be a Date or a number of milliseconds.
  function formatTimelineTime(iso, now, yours) {
    var at = new Date(iso);
    if (isNaN(at.getTime())) {
      return '';
    }
    var nowMs = now !== null && typeof now === 'object' && typeof now.getTime === 'function' ? now.getTime() :
      (typeof now === 'number' ? now : Date.now());
    if (yours === true && nowMs - at.getTime() < JUST_NOW_MS) {
      return NOW_WORD;
    }
    var clock = clockWords(at);
    if (sameDay(at, new Date(nowMs))) {
      return clock;
    }
    return WEEKDAYS[at.getDay()] + ' ' + clock;
  }

  function signatureOf(report) {
    var trust = report && report.trust ? report.trust : {};
    return [trust.confirmed_places, trust.disputed_places, report.visibility, report.view, report.your_vote].join('|');
  }

  function element(tag, className) {
    var el = document.createElement(tag);
    el.className = className;
    return el;
  }

  // create builds the page for one report card. The nodes are made here, once.
  // The returned object is { el, share, update, setHosted, open, close,
  // refresh }.
  function create(report) {
    var reportId = String(report.id);
    var current = report;
    var lastSig = signatureOf(report);
    var hosted = false;
    var opened = false;
    var gone = false;
    var mode = 'none';
    var seq = 0;
    var inflight = null;
    var inflightTimer = null;
    var pollTimer = null;
    // The history rows on the page, one record per event, in order.
    var rows = [];

    var el = element('section', 'report-page');
    el.hidden = true;

    var section = element('section', 'report-page__timeline');
    var heading = element('h2', 'report-page__heading');
    headingSeq += 1;
    heading.id = 'report-page-history-' + headingSeq;
    Pinalert.setText(heading, HISTORY_TITLE);
    var list = element('ol', 'report-timeline');
    list.setAttribute('aria-labelledby', heading.id);
    list.setAttribute('aria-busy', 'false');
    list.hidden = true;

    var errorBlock = element('div', 'report-timeline__error');
    errorBlock.hidden = true;
    var errorTitle = element('p', 'report-timeline__error-title');
    var errorLine = element('p', 'report-timeline__error-line');
    var retry = element('button', 'report-page__retry');
    retry.setAttribute('type', 'button');
    Pinalert.setText(retry, RETRY_LABEL);
    errorBlock.appendChild(errorTitle);
    errorBlock.appendChild(errorLine);
    errorBlock.appendChild(retry);

    section.appendChild(heading);
    section.appendChild(list);
    section.appendChild(errorBlock);

    var comments = element('div', 'report-page__comments');
    comments.setAttribute('data-report-id', reportId);
    // The comments section is comments.js's own (plan 08-16). It is built once
    // into the mount point, and a failed comments.js load leaves the page
    // without comments, the same guard the Share button has.
    var commentsView = null;
    if (window.PinalertComments && typeof window.PinalertComments.create === 'function') {
      commentsView = window.PinalertComments.create(report, comments);
    }

    el.appendChild(section);
    el.appendChild(comments);

    // The Share button sits in the actions group of the card, not inside el
    // (map.js places it). It is hidden until a host shows the page and the
    // Share window script has loaded.
    var share = element('button', 'vote-btn report-page__share');
    share.setAttribute('type', 'button');
    share.hidden = true;
    Pinalert.setText(share, SHARE_LABEL);
    share.addEventListener('click', function (evt) {
      evt.stopPropagation();
      if (window.PinalertShare && typeof window.PinalertShare.open === 'function') {
        window.PinalertShare.open({ reportId: reportId, category: current.category, opener: share });
      }
    });
    share.addEventListener('keydown', function (evt) {
      if (evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar') {
        evt.stopPropagation();
      }
    });

    function isActive() {
      return hosted && opened;
    }

    function syncShare() {
      share.hidden = !(hosted && window.PinalertShare && typeof window.PinalertShare.open === 'function');
    }

    function clearList() {
      rows = [];
      while (list.firstChild) {
        list.removeChild(list.firstChild);
      }
    }

    function showSection() {
      section.hidden = false;
    }

    function showSkeleton() {
      if (mode === 'skeleton') {
        return;
      }
      mode = 'skeleton';
      clearList();
      for (var i = 0; i < SKELETON_ROWS; i++) {
        var row = element('li', 'report-timeline__row report-timeline__row--skeleton');
        row.appendChild(element('span', 'report-timeline__bar report-timeline__bar--time'));
        row.appendChild(element('span', 'report-timeline__bar report-timeline__bar--dot'));
        row.appendChild(element('span', 'report-timeline__bar report-timeline__bar--text'));
        list.appendChild(row);
      }
      list.setAttribute('aria-busy', 'true');
      list.hidden = false;
      errorBlock.hidden = true;
      showSection();
    }

    function showError(line) {
      mode = 'error';
      clearList();
      list.setAttribute('aria-busy', 'false');
      list.hidden = true;
      Pinalert.setText(errorTitle, ERROR_TITLE);
      Pinalert.setText(errorLine, line);
      errorBlock.hidden = false;
      showSection();
    }

    function markGone() {
      gone = true;
      mode = 'none';
      clearList();
      list.setAttribute('aria-busy', 'false');
      list.hidden = true;
      errorBlock.hidden = true;
      section.hidden = true;
    }

    function makeRow() {
      var row = element('li', 'report-timeline__row');
      var time = element('time', 'report-timeline__time');
      var dot = element('span', 'report-timeline__dot');
      dot.setAttribute('aria-hidden', 'true');
      var text = element('p', 'report-timeline__text');
      row.appendChild(time);
      row.appendChild(dot);
      row.appendChild(text);
      return { el: row, time: time, dot: dot, text: text };
    }

    // writeRow writes one event into a row it already has. A node is written
    // only when its words change, so a refresh leaves a reader's selection
    // alone.
    function writeRow(row, event) {
      var time = row.time;
      var dot = row.dot;
      var text = row.text;
      var tone = TONES.indexOf(event.tone) >= 0 ? event.tone : 'neutral';
      var when = formatTimelineTime(event.at, Date.now(), event.yours === true);
      if (time.textContent !== when) {
        Pinalert.setText(time, when);
      }
      time.setAttribute('datetime', typeof event.at === 'string' ? event.at : '');
      var dotClass = 'report-timeline__dot report-timeline__dot--' + tone;
      if (dot.className !== dotClass) {
        dot.className = dotClass;
      }
      var words = typeof event.text === 'string' ? event.text : '';
      if (text.textContent !== words) {
        Pinalert.setText(text, words);
      }
    }

    function render(body) {
      var events = body && Array.isArray(body.events) ? body.events : null;
      if (events === null) {
        throw new Error('the history answer has no events');
      }
      if (events.length === 0) {
        markGone();
        return;
      }
      if (mode !== 'rows') {
        clearList();
        mode = 'rows';
      }
      for (var i = 0; i < events.length; i++) {
        if (!rows[i]) {
          rows[i] = makeRow();
          list.appendChild(rows[i].el);
        }
        writeRow(rows[i], events[i]);
      }
      while (rows.length > events.length) {
        list.removeChild(rows.pop().el);
      }
      list.setAttribute('aria-busy', 'false');
      list.hidden = false;
      errorBlock.hidden = true;
      showSection();
    }

    // stopRequest drops the request in flight. The sequence moves on, so the
    // answer of a dropped request is ignored whenever it arrives.
    function stopRequest() {
      seq += 1;
      if (inflightTimer !== null) {
        window.clearTimeout(inflightTimer);
        inflightTimer = null;
      }
      if (inflight !== null) {
        inflight.abort();
        inflight = null;
      }
    }

    // load asks the server for the history. The first answer shows a skeleton;
    // a later one is written over the rows already on the page, and a failed
    // later one leaves those rows alone.
    function load() {
      if (!isActive()) {
        return;
      }
      stopRequest();
      var mine = seq;
      if (navigator.onLine === false) {
        if (mode !== 'rows') {
          showError(OFFLINE_LINE);
        }
        return;
      }
      if (mode !== 'rows') {
        showSkeleton();
      }
      var controller = new AbortController();
      inflight = controller;
      inflightTimer = window.setTimeout(function () {
        controller.abort();
      }, TIMELINE_TIMEOUT_MS);
      fetch('/api/reports/' + encodeURIComponent(reportId) + '/timeline', {
        credentials: 'same-origin',
        headers: { Accept: 'application/json' },
        signal: controller.signal
      }).then(function (res) {
        if (mine !== seq) {
          return null;
        }
        if (res.status === STATUS_GONE) {
          settle();
          markGone();
          return null;
        }
        if (!res.ok) {
          throw new Error('the history answered with an error');
        }
        return res.json().then(function (body) {
          if (mine !== seq) {
            return;
          }
          settle();
          render(body);
        });
      }).catch(function () {
        if (mine !== seq) {
          return;
        }
        settle();
        if (mode !== 'rows') {
          showError(navigator.onLine === false ? OFFLINE_LINE : ERROR_LINE);
        }
      });
    }

    // settle ends the bookkeeping of the request that just answered.
    function settle() {
      if (inflightTimer !== null) {
        window.clearTimeout(inflightTimer);
        inflightTimer = null;
      }
      inflight = null;
    }

    function tick() {
      if (!isActive() || gone || document.visibilityState !== 'visible') {
        return;
      }
      load();
    }

    // onVisible asks for the history again when the tab comes back, so a
    // reader back from another app does not wait for the next tick (review fix
    // IN-B06). It is listened for only while the poll runs.
    function onVisible() {
      tick();
    }

    function startPolling() {
      if (pollTimer === null) {
        pollTimer = window.setInterval(tick, TIMELINE_POLL_MS);
        if (typeof document.addEventListener === 'function') {
          document.addEventListener('visibilitychange', onVisible);
        }
      }
    }

    function stopPolling() {
      if (pollTimer !== null) {
        window.clearInterval(pollTimer);
        pollTimer = null;
        if (typeof document.removeEventListener === 'function') {
          document.removeEventListener('visibilitychange', onVisible);
        }
      }
    }

    retry.addEventListener('click', function (evt) {
      evt.stopPropagation();
      if (!isActive()) {
        return;
      }
      // The button is about to leave the page, so focus goes to the heading
      // of its own section; this is a reader's own press, not a refresh.
      heading.setAttribute('tabindex', '-1');
      heading.focus({ preventScroll: true });
      gone = false;
      mode = 'none';
      load();
    });
    retry.addEventListener('keydown', function (evt) {
      if (evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar') {
        evt.stopPropagation();
      }
    });

    // update records what the card now shows. Counts, flags and the history are
    // written into the nodes this page already has; nothing is rebuilt. A
    // changed signature refetches once, and only when the page is showing.
    function update(next) {
      current = next;
      syncShare();
      if (commentsView) {
        commentsView.update(next);
      }
      var sig = signatureOf(next);
      if (sig === lastSig) {
        return;
      }
      lastSig = sig;
      if (isActive()) {
        gone = false;
        load();
      }
    }

    // setHosted says whether a host shows the page. A host that does not show
    // it (the laptop bubble today) leaves it hidden and fetching nothing.
    function setHosted(on) {
      var next = on === true;
      el.hidden = !next;
      var changed = next !== hosted;
      hosted = next;
      syncShare();
      if (!changed) {
        return;
      }
      if (hosted && opened) {
        gone = false;
        startPolling();
        load();
        if (commentsView) {
          commentsView.open();
        }
      } else if (!hosted) {
        stopPolling();
        stopRequest();
        if (commentsView) {
          commentsView.close();
        }
      }
    }

    // open marks the report as opened. It fetches at once and starts the one
    // interval, when a host shows the page.
    function open() {
      opened = true;
      if (!hosted) {
        return;
      }
      gone = false;
      startPolling();
      load();
      if (commentsView) {
        commentsView.open();
      }
    }

    function close() {
      opened = false;
      stopPolling();
      stopRequest();
      if (commentsView) {
        commentsView.close();
      }
    }

    // refresh asks for the history again now, if the page is showing.
    function refresh() {
      if (isActive()) {
        gone = false;
        load();
      }
    }

    return {
      el: el,
      share: share,
      update: update,
      setHosted: setHosted,
      open: open,
      close: close,
      refresh: refresh
    };
  }

  return {
    create: create,
    formatTimelineTime: formatTimelineTime
  };
})();
