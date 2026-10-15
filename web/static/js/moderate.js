// moderate.js: the client half of the moderator screen (04-26, D-08). Only
// listed moderators ever reach this page; everyone else gets the server's
// plain 404, so this file holds no authority and adds no check of its own.
// It loads the three lists, writes each heading count from the server's
// counts object, finds a report by id or pasted link, keeps a report at once,
// removes one only after an inline confirmation, and refreshes on demand.
// There is no polling and nothing is saved for a later retry.
//
// SECURITY (T-04-01): every string reaches the DOM through Pinalert.setText,
// a text node assignment. Report text, server words, flag reason labels and
// score keys are never parsed as markup, so a link inside a report stays
// inert. No server message is shown: every sentence here is client copy.

(function () {
  'use strict';

  var root = document.querySelector('.mod-page');
  if (!root || !window.Pinalert || !window.PinalertTrust) {
    // Guard the whole module behind the page frame, so it is inert elsewhere.
    return;
  }

  var REQUEST_TIMEOUT_MS = 10000;
  var TOAST_MS = 3000;
  var TOP_SCORES = 3;
  var REPORT_ID_MAX_DIGITS = 15;
  var REPORT_ID_PATTERN = /^[0-9]+$/;
  var REPORT_PARAM_PATTERN = /(?:^|[?&#])report=([0-9]+)/;
  var MAIN_PAGE_VIEWS = ['disputed', 'resolved', 'past'];

  // The thirteen provider category keys in the order of their plain words.
  var SCORE_ORDER = [
    'hate', 'hate/threatening', 'harassment', 'harassment/threatening',
    'illicit', 'illicit/violent', 'self-harm', 'self-harm/intent',
    'self-harm/instructions', 'sexual', 'sexual/minors', 'violence',
    'violence/graphic'
  ];
  var SCORE_WORDS = {
    'hate': 'Hate',
    'hate/threatening': 'Hate with threats',
    'harassment': 'Harassment',
    'harassment/threatening': 'Harassment with threats',
    'illicit': 'Illicit',
    'illicit/violent': 'Illicit and violent',
    'self-harm': 'Self harm',
    'self-harm/intent': 'Self harm intent',
    'self-harm/instructions': 'Self harm instructions',
    'sexual': 'Sexual',
    'sexual/minors': 'Sexual involving minors',
    'violence': 'Violence',
    'violence/graphic': 'Graphic violence'
  };

  // The three lists: the key the server uses, the slug the page ids use and
  // the sentence that says what the two actions do for a report in the list.
  var LISTS = [
    {
      key: 'review',
      slug: 'auto',
      does: 'Keep report clears the flag. The report stays. Remove report hides it from everyone and tells the author.'
    },
    {
      key: 'flagged',
      slug: 'people',
      does: 'Keep report clears the flags on it. The report stays. Remove report hides it from everyone and tells the author.'
    },
    {
      key: 'pending',
      slug: 'pending',
      does: 'Keep report accepts the text as it is and lets the report through the normal rules. Remove report hides it from everyone and tells the author.'
    }
  ];

  var FIND_EMPTY_MESSAGE = 'Enter a report id or a link that includes report=.';
  var FIND_FAILED_MESSAGE = "Couldn't look that up. Try again.";
  var FIND_OFFLINE_MESSAGE = 'No signal. Try again when you have a signal.';
  var ACTION_FAILED_MESSAGE = "Couldn't save that. Try again.";
  var ACTION_OFFLINE_MESSAGE = 'No signal. Nothing changed. Try again.';
  var ACTION_HANDLED_MESSAGE = 'This report was already handled. Refresh the lists.';
  var REFRESHED_TOAST = 'Lists refreshed.';
  var HINT_TEXT = 'Nothing needs a decision here. Use Find a report to look up a post by id.';
  var REMOVE_QUESTION = 'Remove this report? The author is told it was removed after review.';
  var KEEP_LABEL = 'Keep report';
  var REMOVE_LABEL = 'Remove report';
  var GO_BACK_LABEL = 'Go back';
  var CONFIRM_REMOVE_LABEL = 'Yes, remove report';
  var SENDING_LABEL = 'Sending';
  var SHOW_ALL_SCORES = 'Show all scores';
  var MAIN_PAGE_LINK_TEXT = 'View on the main page';
  var HELD_WORD = 'Held, shown to no one';
  var REMOVED_WORD = 'Removed after review';
  var REMOVED_SENTENCE = 'Removed after review.';
  var SCORED_INTRO = 'The check scored this text:';
  var UNFINISHED_INTRO = "The text check hasn't finished.";
  var HELD_LINE = "Held. It isn't shown to anyone until the check finishes or you keep it.";
  var SERIOUS_PENDING_LINE = 'Serious report, already published while the check waits.';
  var NOT_LISTED_DOES = 'Remove report hides it from everyone and tells the author.';

  // The page elements this module drives. If one is missing, nothing is wired.
  var refreshBtn = document.getElementById('mod-refresh');
  var refreshError = document.getElementById('mod-refresh-error');
  var findForm = document.getElementById('mod-find-form');
  var findInput = document.getElementById('mod-find');
  var findResult = document.getElementById('mod-find-result');
  var sections = {};
  var complete = !!(refreshBtn && refreshError && findForm && findInput && findResult);
  LISTS.forEach(function (list) {
    var heading = document.getElementById('mod-h-' + list.slug);
    var count = document.getElementById('mod-count-' + list.slug);
    var listEl = document.getElementById('mod-list-' + list.slug);
    var empty = document.getElementById('mod-empty-' + list.slug);
    if (!heading || !count || !listEl || !empty || !heading.parentNode) {
      complete = false;
      return;
    }
    sections[list.key] = { heading: heading, count: count, list: listEl, empty: empty, section: heading.parentNode };
  });
  if (!complete) {
    return;
  }

  // The state. counts holds the server's numbers and only dropReport lowers
  // them. membership maps a report id to the list keys it was in at the last
  // load. entries maps a row key to its entry object.
  var counts = { review: 0, flagged: 0, pending: 0 };
  var membership = {};
  var entries = {};
  var findState = null;
  var refreshing = false;
  var finding = false;
  var hintEl = null;

  function apiRequest(method, path, timeoutMs) {
    var options = {
      method: method,
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { 'Accept': 'application/json' }
    };
    var timer = null;
    if (timeoutMs > 0) {
      if (typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function') {
        options.signal = AbortSignal.timeout(timeoutMs);
      } else if (typeof AbortController !== 'undefined') {
        var controller = new AbortController();
        options.signal = controller.signal;
        timer = window.setTimeout(function () {
          controller.abort();
        }, timeoutMs);
      }
    }
    return fetch(path, options).then(function (res) {
      if (timer !== null) {
        window.clearTimeout(timer);
      }
      if (!res.ok) {
        return { status: res.status, ok: false, body: null };
      }
      return res.json().then(function (body) {
        return { status: res.status, ok: true, body: body };
      }, function () {
        return { status: res.status, ok: true, body: null };
      });
    }, function () {
      if (timer !== null) {
        window.clearTimeout(timer);
      }
      return Promise.reject({ noAnswer: true });
    });
  }

  function toCount(value) {
    if (typeof value === 'number' && isFinite(value) && value >= 0 && Math.floor(value) === value) {
      return value;
    }
    return 0;
  }

  function isReportId(value) {
    return typeof value === 'number' && isFinite(value) && Math.floor(value) === value && value > 0;
  }

  function parseReportId(text) {
    var value = String(text || '').trim();
    var digits = '';
    if (REPORT_ID_PATTERN.test(value)) {
      digits = value;
    } else {
      var match = REPORT_PARAM_PATTERN.exec(value);
      if (match) {
        digits = match[1];
      }
    }
    digits = digits.replace(/^0+/, '');
    if (digits === '' || digits.length > REPORT_ID_MAX_DIGITS) {
      return null;
    }
    return digits;
  }

  function notFoundMessage(id) {
    return 'No report found with id ' + id + '. Check the id and try again.';
  }

  function foundMessage(id) {
    return 'Found report ' + id + '.';
  }

  function keptToast(id) {
    return 'Report ' + id + ' kept.';
  }

  function removedToast(id) {
    return 'Report ' + id + ' removed.';
  }

  function flaggedLine(total) {
    if (total === 1) {
      return '1 person flagged this.';
    }
    return total + ' people flagged this.';
  }

  function postedLine(report) {
    return 'Posted ' + Pinalert.relativeTime(report.created_at) + '.';
  }

  function scoreWord(key) {
    if (Object.prototype.hasOwnProperty.call(SCORE_WORDS, key)) {
      return SCORE_WORDS[key];
    }
    return String(key);
  }

  function scoreRank(key) {
    var at = SCORE_ORDER.indexOf(key);
    return at === -1 ? SCORE_ORDER.length : at;
  }

  // sortedScores keeps the finite numbers, highest first, ties in the order of
  // the plain words (an unknown key sorts last, then by its own name).
  function sortedScores(scores) {
    var items = [];
    if (scores && typeof scores === 'object') {
      Object.keys(scores).forEach(function (key) {
        var value = scores[key];
        if (typeof value === 'number' && isFinite(value)) {
          items.push({ key: key, value: value });
        }
      });
    }
    items.sort(function (a, b) {
      if (b.value !== a.value) {
        return b.value - a.value;
      }
      var rankA = scoreRank(a.key);
      var rankB = scoreRank(b.key);
      if (rankA !== rankB) {
        return rankA - rankB;
      }
      if (a.key === b.key) {
        return 0;
      }
      return a.key < b.key ? -1 : 1;
    });
    return items;
  }

  function scoreLine(item) {
    return scoreWord(item.key) + ' ' + item.value.toFixed(2);
  }

  function moderationWord(report) {
    if (report.moderation === 'removed') {
      return REMOVED_WORD;
    }
    if (report.moderation === 'pending' && report.serious !== true) {
      return HELD_WORD;
    }
    return '';
  }

  // mainPageHref is the only place a link is built: a closed set of view words
  // and a numeric id, and none at all for a report no view shows.
  function mainPageHref(report) {
    if (!isReportId(report.id) || moderationWord(report) !== '') {
      return '';
    }
    if (report.view === 'active') {
      return '/?report=' + report.id;
    }
    if (MAIN_PAGE_VIEWS.indexOf(report.view) !== -1) {
      return '/?view=' + report.view + '&report=' + report.id;
    }
    return '';
  }

  function clearChildren(el) {
    while (el.firstChild) {
      el.removeChild(el.firstChild);
    }
  }

  function listByKey(key) {
    for (var i = 0; i < LISTS.length; i += 1) {
      if (LISTS[i].key === key) {
        return LISTS[i];
      }
    }
    return null;
  }

  function makeEl(tag, className) {
    var el = document.createElement(tag);
    el.className = className;
    return el;
  }

  function makeButton(className, action, label, ariaLabel) {
    var button = makeEl('button', className);
    button.setAttribute('type', 'button');
    button.setAttribute('data-mod-action', action);
    button.setAttribute('aria-label', ariaLabel);
    Pinalert.setText(button, label);
    return button;
  }

  function buildIdLine(entry, report) {
    var line = makeEl('p', 'mod-row__idline');
    var name = makeEl('span', 'mod-row__id');
    name.id = 'mod-' + entry.slug + '-' + entry.id + '-name';
    Pinalert.setText(name, 'Report ' + entry.id);
    line.appendChild(name);

    var label = PinalertTrust.createSeverityLabel();
    PinalertTrust.updateSeverityLabel(label, report);
    line.appendChild(label);

    var visibility = report.visibility_word;
    if (typeof visibility === 'string' && visibility !== '') {
      var visibilityEl = makeEl('span', 'mod-row__state');
      Pinalert.setText(visibilityEl, visibility);
      line.appendChild(visibilityEl);
    }
    var moderation = moderationWord(report);
    if (moderation !== '') {
      var moderationEl = makeEl('span', 'mod-row__state');
      Pinalert.setText(moderationEl, moderation);
      line.appendChild(moderationEl);
    }
    return line;
  }

  function buildScores(report) {
    var items = sortedScores(report.scores);
    if (items.length === 0) {
      return null;
    }
    var block = makeEl('div', 'mod-row__why');
    var intro = makeEl('p', 'mod-row__why-line');
    Pinalert.setText(intro, SCORED_INTRO);
    block.appendChild(intro);

    var top = makeEl('ul', 'mod-scores');
    items.slice(0, TOP_SCORES).forEach(function (item) {
      var li = document.createElement('li');
      Pinalert.setText(li, scoreLine(item));
      top.appendChild(li);
    });
    block.appendChild(top);

    var details = document.createElement('details');
    details.className = 'mod-row__all-scores';
    var summary = document.createElement('summary');
    Pinalert.setText(summary, SHOW_ALL_SCORES);
    details.appendChild(summary);
    var all = makeEl('ul', 'mod-scores');
    items.forEach(function (item) {
      var li = document.createElement('li');
      Pinalert.setText(li, scoreLine(item));
      all.appendChild(li);
    });
    details.appendChild(all);
    block.appendChild(details);
    return block;
  }

  // buildFlags reads the total and each reason's label and count, and nothing
  // else: no name, email or account of anyone who flagged (open item 12).
  function buildFlags(report) {
    var flags = report.flags || {};
    var block = makeEl('div', 'mod-row__why');
    var line = makeEl('p', 'mod-row__why-line');
    Pinalert.setText(line, flaggedLine(toCount(flags.total)));
    block.appendChild(line);

    var reasons = Array.isArray(flags.reasons) ? flags.reasons : [];
    var list = makeEl('ul', 'mod-scores');
    reasons.forEach(function (reason) {
      if (!reason || typeof reason.label !== 'string') {
        return;
      }
      var n = toCount(reason.count);
      if (n < 1) {
        return;
      }
      var li = document.createElement('li');
      Pinalert.setText(li, reason.label + ': ' + n);
      list.appendChild(li);
    });
    if (list.firstChild) {
      block.appendChild(list);
    }
    return block;
  }

  function buildUnfinished(report) {
    var block = makeEl('div', 'mod-row__why');
    var intro = makeEl('p', 'mod-row__why-line');
    Pinalert.setText(intro, UNFINISHED_INTRO);
    block.appendChild(intro);
    var posted = makeEl('p', 'mod-row__why-line');
    Pinalert.setText(posted, postedLine(report));
    block.appendChild(posted);
    var state = makeEl('p', 'mod-row__why-line');
    if (moderationWord(report) === HELD_WORD) {
      Pinalert.setText(state, HELD_LINE);
    } else {
      Pinalert.setText(state, SERIOUS_PENDING_LINE);
    }
    block.appendChild(state);
    return block;
  }

  // buildWhy returns the why blocks of a row: its own list's block, or for a
  // found report one block per list it is in, in the order of LISTS.
  function buildWhy(listKey, report, lists) {
    var wanted = listKey === 'find' ? (lists || []) : [listKey];
    var blocks = [];
    LISTS.forEach(function (list) {
      if (wanted.indexOf(list.key) === -1) {
        return;
      }
      var block;
      if (list.key === 'review') {
        block = buildScores(report);
      } else if (list.key === 'flagged') {
        block = buildFlags(report);
      } else {
        block = buildUnfinished(report);
      }
      if (block) {
        blocks.push(block);
      }
    });
    return blocks;
  }

  function buildRemoveConfirm(entry) {
    var confirm = makeEl('div', 'mod-confirm');
    confirm.setAttribute('role', 'group');
    var questionId = 'mod-' + entry.slug + '-' + entry.id + '-question';
    confirm.setAttribute('aria-labelledby', questionId);
    confirm.hidden = true;

    var question = makeEl('p', 'mod-confirm__question');
    question.id = questionId;
    Pinalert.setText(question, REMOVE_QUESTION);
    confirm.appendChild(question);

    var buttons = makeEl('div', 'mod-confirm__buttons');
    var goBack = makeButton('btn', 'back', GO_BACK_LABEL, GO_BACK_LABEL);
    var confirmBtn = makeButton('btn btn--destructive', 'confirm-remove', CONFIRM_REMOVE_LABEL,
      'Yes, remove report ' + entry.id);
    buttons.appendChild(goBack);
    buttons.appendChild(confirmBtn);
    confirm.appendChild(buttons);

    entry.confirm = confirm;
    entry.goBack = goBack;
    entry.confirmBtn = confirmBtn;
    return confirm;
  }

  function buildRow(report, listKey, lists) {
    var list = listByKey(listKey);
    var slug = list ? list.slug : 'find';
    var isFind = slug === 'find';
    var key = slug + ':' + report.id;

    var row = document.createElement(isFind ? 'div' : 'li');
    row.className = 'mod-row';
    if (isFind) {
      row.setAttribute('role', 'group');
    }
    row.setAttribute('data-mod-key', key);
    row.setAttribute('aria-labelledby', 'mod-' + slug + '-' + report.id + '-name');

    var entry = {
      id: report.id,
      slug: slug,
      listKey: listKey,
      lists: lists || [],
      row: row,
      actions: null,
      keepBtn: null,
      removeBtn: null,
      confirm: null,
      goBack: null,
      confirmBtn: null,
      error: null,
      busy: false
    };
    entries[key] = entry;

    row.appendChild(buildIdLine(entry, report));

    var meta = makeEl('p', 'mod-row__meta');
    var category = String(report.category || '');
    if (Object.prototype.hasOwnProperty.call(Pinalert.CATEGORY_LABELS, category)) {
      category = Pinalert.CATEGORY_LABELS[category];
    }
    Pinalert.setText(meta, category + ' · ' + Pinalert.relativeTime(report.created_at));
    row.appendChild(meta);

    var written = Pinalert.writtenLine(report);
    if (written !== '') {
      var writtenEl = makeEl('p', 'mod-row__written');
      Pinalert.setText(writtenEl, written);
      row.appendChild(writtenEl);
    }

    var text = makeEl('p', 'mod-row__text');
    Pinalert.setText(text, report.description);
    row.appendChild(text);

    buildWhy(listKey, report, entry.lists).forEach(function (block) {
      row.appendChild(block);
    });

    if (report.moderation === 'removed') {
      var done = makeEl('p', 'mod-row__done');
      Pinalert.setText(done, REMOVED_SENTENCE);
      row.appendChild(done);
    } else {
      var actions = makeEl('div', 'mod-row__actions');
      if (!isFind || entry.lists.length > 0) {
        entry.keepBtn = makeButton('btn', 'keep', KEEP_LABEL, 'Keep report ' + report.id);
        actions.appendChild(entry.keepBtn);
      }
      entry.removeBtn = makeButton('btn', 'remove', REMOVE_LABEL, 'Remove report ' + report.id);
      actions.appendChild(entry.removeBtn);
      entry.actions = actions;
      row.appendChild(actions);
      row.appendChild(buildRemoveConfirm(entry));

      var does = makeEl('p', 'mod-row__does');
      var doesText = NOT_LISTED_DOES;
      if (!isFind) {
        doesText = list.does;
      } else {
        for (var i = 0; i < LISTS.length; i += 1) {
          if (entry.lists.indexOf(LISTS[i].key) !== -1) {
            doesText = LISTS[i].does;
            break;
          }
        }
      }
      Pinalert.setText(does, doesText);
      row.appendChild(does);
    }

    var href = mainPageHref(report);
    if (href !== '') {
      var link = makeEl('a', 'mod-row__link');
      link.setAttribute('href', href);
      Pinalert.setText(link, MAIN_PAGE_LINK_TEXT);
      row.appendChild(link);
    }
    return entry;
  }

  function renderList(list, reports) {
    var listEl = sections[list.key].list;
    clearChildren(listEl);
    Object.keys(entries).forEach(function (key) {
      if (key.indexOf(list.slug + ':') === 0) {
        delete entries[key];
      }
    });
    reports.forEach(function (report) {
      if (!report || !isReportId(report.id)) {
        return;
      }
      listEl.appendChild(buildRow(report, list.key, null).row);
    });
  }

  function renderCounts() {
    LISTS.forEach(function (list) {
      Pinalert.setText(sections[list.key].count, '(' + counts[list.key] + ')');
    });
  }

  function renderEmpties() {
    var firstShown = null;
    LISTS.forEach(function (list) {
      var section = sections[list.key];
      var isEmpty = section.list.firstChild === null;
      section.empty.hidden = !isEmpty;
      if (isEmpty && firstShown === null) {
        firstShown = section.empty;
      }
    });
    if (!hintEl) {
      hintEl = makeEl('p', 'mod-hint');
      Pinalert.setText(hintEl, HINT_TEXT);
      hintEl.hidden = true;
    }
    if (firstShown === null) {
      hintEl.hidden = true;
      return;
    }
    firstShown.parentNode.insertBefore(hintEl, firstShown.nextSibling);
    hintEl.hidden = false;
  }

  function applyQueue(data) {
    counts.review = toCount(data.counts && data.counts.review);
    counts.flagged = toCount(data.counts && data.counts.flagged);
    counts.pending = toCount(data.counts && data.counts.pending);
    membership = {};
    LISTS.forEach(function (list) {
      var reports = Array.isArray(data[list.key]) ? data[list.key] : [];
      renderList(list, reports);
      reports.forEach(function (report) {
        if (!report || !isReportId(report.id)) {
          return;
        }
        if (!membership[report.id]) {
          membership[report.id] = {};
        }
        membership[report.id][list.key] = true;
      });
    });
    renderCounts();
    renderEmpties();
  }

  function loadLists(manual) {
    if (refreshing) {
      return;
    }
    refreshing = true;
    LISTS.forEach(function (list) {
      sections[list.key].section.setAttribute('aria-busy', 'true');
    });
    refreshBtn.setAttribute('aria-disabled', 'true');
    apiRequest('GET', '/api/moderation/queue', REQUEST_TIMEOUT_MS).then(function (res) {
      if (res.ok && res.body) {
        applyQueue(res.body);
        refreshError.hidden = true;
        if (manual) {
          Pinalert.showToast(REFRESHED_TOAST, null, TOAST_MS);
        }
      } else {
        refreshError.hidden = false;
      }
    }, function () {
      refreshError.hidden = false;
    }).then(function () {
      LISTS.forEach(function (list) {
        sections[list.key].section.removeAttribute('aria-busy');
      });
      refreshBtn.removeAttribute('aria-disabled');
      refreshing = false;
    });
  }

  // showFindMessage puts one note or error in the polite result region. It
  // sets no role: the region already announces, so a role would say it twice.
  function showFindMessage(text, look) {
    clearChildren(findResult);
    findState = null;
    Object.keys(entries).forEach(function (key) {
      if (key.indexOf('find:') === 0) {
        delete entries[key];
      }
    });
    var message = document.createElement('p');
    message.className = look === 'error' ? 'inline-error' : 'inline-note';
    Pinalert.setText(message, text);
    findResult.appendChild(message);
  }

  function renderFindResult(report, lists) {
    if (!report || !isReportId(report.id)) {
      showFindMessage(FIND_FAILED_MESSAGE, 'error');
      return;
    }
    var kept = [];
    (Array.isArray(lists) ? lists : []).forEach(function (key) {
      if (listByKey(key) && kept.indexOf(key) === -1) {
        kept.push(key);
      }
    });
    Object.keys(entries).forEach(function (key) {
      if (key.indexOf('find:') === 0) {
        delete entries[key];
      }
    });
    findState = { id: report.id, lists: kept };
    clearChildren(findResult);
    var status = makeEl('p', 'mod-find__status');
    Pinalert.setText(status, foundMessage(report.id));
    findResult.appendChild(status);
    findResult.appendChild(buildRow(report, 'find', kept).row);
  }

  function finishFind() {
    finding = false;
    var submit = findForm.querySelector('button[type="submit"]');
    if (submit) {
      submit.removeAttribute('aria-disabled');
    }
  }

  function onFindSubmit(evt) {
    evt.preventDefault();
    if (finding) {
      return;
    }
    var id = parseReportId(findInput.value);
    if (id === null) {
      showFindMessage(FIND_EMPTY_MESSAGE, 'note');
      return;
    }
    finding = true;
    var submit = findForm.querySelector('button[type="submit"]');
    if (submit) {
      submit.setAttribute('aria-disabled', 'true');
    }
    apiRequest('GET', '/api/moderation/reports/' + id, REQUEST_TIMEOUT_MS).then(function (res) {
      if (res.ok && res.body && res.body.report) {
        renderFindResult(res.body.report, res.body.lists);
      } else if (res.status === 404) {
        showFindMessage(notFoundMessage(id), 'note');
      } else {
        showFindMessage(FIND_FAILED_MESSAGE, 'error');
      }
    }, function () {
      showFindMessage(FIND_OFFLINE_MESSAGE, 'error');
    }).then(finishFind);
  }

  function openRemoveConfirm(entry) {
    entry.actions.hidden = true;
    entry.confirm.hidden = false;
    entry.goBack.focus();
  }

  function closeRemoveConfirm(entry, returnFocus) {
    entry.confirm.hidden = true;
    entry.actions.hidden = false;
    if (returnFocus) {
      entry.removeBtn.focus();
    }
  }

  function setBusy(entry, busy, action) {
    entry.busy = busy;
    if (busy) {
      entry.row.setAttribute('aria-busy', 'true');
      if (entry.error) {
        entry.error.hidden = true;
      }
    } else {
      entry.row.removeAttribute('aria-busy');
    }
    Array.prototype.forEach.call(entry.row.querySelectorAll('button'), function (button) {
      if (busy) {
        button.setAttribute('aria-disabled', 'true');
      } else {
        button.removeAttribute('aria-disabled');
      }
    });
    if (action === 'remove' && entry.confirmBtn) {
      if (busy) {
        Pinalert.setText(entry.confirmBtn, SENDING_LABEL);
        entry.confirmBtn.setAttribute('aria-label', 'Sending, remove report ' + entry.id);
      } else {
        Pinalert.setText(entry.confirmBtn, CONFIRM_REMOVE_LABEL);
        entry.confirmBtn.setAttribute('aria-label', 'Yes, remove report ' + entry.id);
      }
    }
  }

  function showRowError(entry, message) {
    if (!entry.error) {
      entry.error = document.createElement('p');
      entry.error.className = 'inline-error mod-row__error';
      entry.error.setAttribute('role', 'alert');
      var after = entry.confirm || entry.actions;
      if (after && after.parentNode === entry.row) {
        entry.row.insertBefore(entry.error, after.nextSibling);
      } else {
        entry.row.appendChild(entry.error);
      }
    }
    Pinalert.setText(entry.error, message);
    entry.error.hidden = false;
  }

  // firstRowButton is the first button of a row's shown actions, else of its
  // shown confirmation, else nothing.
  function firstRowButton(row) {
    var scope = row.querySelector('.mod-row__actions:not([hidden])') ||
      row.querySelector('.mod-confirm:not([hidden])');
    return scope ? scope.querySelector('button') : null;
  }

  // focusTargetAfter picks where focus goes when a list row leaves: the next
  // row's first button, else the previous row's, else the section heading.
  function focusTargetAfter(entry) {
    var next = entry.row.nextElementSibling;
    var prev = entry.row.previousElementSibling;
    var target = next ? firstRowButton(next) : null;
    if (!target && prev) {
      target = firstRowButton(prev);
    }
    if (!target) {
      target = sections[entry.listKey].heading;
      target.setAttribute('tabindex', '-1');
    }
    return target;
  }

  // dropReport lowers the count of each list that held the report, once per
  // list, with a floor of 0, and takes the report's rows off the page.
  function dropReport(id, extraKeys) {
    var seen = {};
    Object.keys(membership[id] || {}).forEach(function (key) {
      seen[key] = true;
    });
    (extraKeys || []).forEach(function (key) {
      seen[key] = true;
    });
    LISTS.forEach(function (list) {
      if (seen[list.key]) {
        counts[list.key] = Math.max(0, counts[list.key] - 1);
      }
      var rowKey = list.slug + ':' + id;
      var entry = entries[rowKey];
      if (entry) {
        if (entry.row.parentNode) {
          entry.row.parentNode.removeChild(entry.row);
        }
        delete entries[rowKey];
      }
    });
    delete membership[id];
    renderCounts();
    renderEmpties();
  }

  function actionDone(entry, action, body) {
    var report = body && body.report;
    var shownInFind = !!findState && String(findState.id) === String(entry.id);
    if (entry.slug === 'find') {
      dropReport(entry.id, entry.lists);
      if (report && isReportId(report.id)) {
        renderFindResult(report, []);
        var made = entries['find:' + report.id];
        var first = made ? firstRowButton(made.row) : null;
        (first || findInput).focus();
      } else {
        clearChildren(findResult);
        findState = null;
        findInput.focus();
      }
    } else {
      var target = focusTargetAfter(entry);
      dropReport(entry.id, shownInFind ? findState.lists : []);
      if (shownInFind && report && isReportId(report.id)) {
        renderFindResult(report, []);
      }
      target.focus();
    }
    Pinalert.showToast(action === 'remove' ? removedToast(entry.id) : keptToast(entry.id), null, TOAST_MS);
  }

  function actionFailed(entry, action, message) {
    setBusy(entry, false, action);
    showRowError(entry, message);
    var starter = action === 'remove' ? entry.confirmBtn : entry.keepBtn;
    if (starter) {
      starter.focus();
    }
  }

  // sendAction posts Keep or Remove. It carries no client timer: a timeout
  // after the server committed would make "Nothing changed" false, and a lost
  // answer is safe because a retry is answered as already handled.
  function sendAction(entry, action) {
    if (entry.busy) {
      return;
    }
    setBusy(entry, true, action);
    var suffix = action === 'remove' ? '/remove' : '/keep';
    apiRequest('POST', '/api/moderation/reports/' + entry.id + suffix, 0).then(function (res) {
      if (res.ok) {
        actionDone(entry, action, res.body);
      } else if (res.status === 404 || res.status === 409) {
        actionFailed(entry, action, ACTION_HANDLED_MESSAGE);
      } else {
        actionFailed(entry, action, ACTION_FAILED_MESSAGE);
      }
    }, function () {
      actionFailed(entry, action, ACTION_OFFLINE_MESSAGE);
    });
  }

  function entryOf(el) {
    var rowEl = el.closest('[data-mod-key]');
    return rowEl ? entries[rowEl.getAttribute('data-mod-key')] : null;
  }

  function onRootClick(evt) {
    var target = evt.target;
    if (!target || typeof target.closest !== 'function') {
      return;
    }
    if (target.closest('#mod-refresh')) {
      loadLists(true);
      return;
    }
    var button = target.closest('button[data-mod-action]');
    if (!button) {
      return;
    }
    var entry = entryOf(button);
    if (!entry || entry.busy) {
      return;
    }
    var action = button.getAttribute('data-mod-action');
    if (action === 'keep') { sendAction(entry, 'keep'); return; }
    if (action === 'remove') { openRemoveConfirm(entry); return; }
    if (action === 'confirm-remove') { sendAction(entry, 'remove'); return; }
    if (action === 'back') { closeRemoveConfirm(entry, true); }
  }

  // onRootKeydown gives the open Remove confirmation its own Escape (P4-A5).
  function onRootKeydown(evt) {
    if (evt.key !== 'Escape' || !evt.target || typeof evt.target.closest !== 'function') {
      return;
    }
    var confirmEl = evt.target.closest('.mod-confirm');
    if (!confirmEl || confirmEl.hidden) {
      return;
    }
    var entry = entryOf(confirmEl);
    if (!entry || entry.busy) {
      return;
    }
    closeRemoveConfirm(entry, true);
    evt.preventDefault();
    evt.stopPropagation();
  }

  root.addEventListener('click', onRootClick);
  root.addEventListener('keydown', onRootKeydown);
  findForm.addEventListener('submit', onFindSubmit);
  loadLists(false);
}());
