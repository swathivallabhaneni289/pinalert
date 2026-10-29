// official-alerts.js: the Official alerts page (Phase 5, plan 09, ALERT-01,
// ALERT-02, ALERT-04, ROBUST-06, D-06 to D-10, UI-SPEC Surface 1).
//
// The page asks one question of one route, GET <base>/official/alerts, and draws
// the answer. The server authors every fact (the state, the age, the group
// labels, the row count, the level words, the badge, the districts line); this
// script recomputes none of them. It only chooses which of the page's blocks to
// show, writes the sentences of UI-SPEC P5-C2 to P5-C12, and formats times in
// the reader's own time zone.
//
// Honesty (T-34-17, T-05-47). The sentence that says no official alert is in
// force is written in exactly one place, behind exactly one test: the state is
// fresh, the server knows the reader's area, and every group is empty. Every
// other shape of answer has its own words: load error, then unavailable, then
// area unknown, then stale or fresh. An answer that is not shaped like the route's
// is a load error, never an empty list. The template carries none of these
// sentences.
//
// Trust boundary (T-05-46). Feed authored strings (headline, area, issuer,
// description, instruction, the original text, the group labels) are written with
// textContent only. No markup is built from a string. A language code is written
// to a lang attribute only when it is two or three lower case letters.
//
// The area (D-09). The server keeps no reader location, so the page sends the
// place Nearby last used: the page's own fixed point (the Demo), else the tab's
// Nearby centre kept by the banner script for this account, else the stored
// device fix, else nothing, and the server then uses the home area. Latitude and
// longitude go together or not at all.
//
// One request on load and one on each Try again press (T-05-50), and, since the
// Phase 5 review fix CR-B01, one more when the answer reaches the 30 minute
// fresh limit, every five minutes while it is stale or unavailable, when a shown
// row ends, and when the tab is shown again more than a minute after the last
// request. Between those, the minutes of the status line move once a minute
// from the server's age plus the time that passed here, with no live region.
//
// The renderer is source agnostic: it reads its API base and its position from
// the page, so the Demo (window.PinalertApiBase, data-fixed-lat and
// data-fixed-lon) and GDACS rows reuse it unchanged.
(function () {
  'use strict';

  var STATUS_TICK_MS = 60000;
  // The server's fresh limit (internal/official FreshWindow), in seconds: a fresh
  // answer this old is asked about again (Phase 5 review fix CR-B01).
  var FRESH_LIMIT_SECONDS = 30 * 60;
  // A stale or unavailable answer is asked about again this often.
  var RECHECK_NOT_FRESH_MS = 5 * 60000;
  // Showing the tab again asks again when the last request is this old.
  var SHOWN_AGAIN_MS = 60000;
  // A request with no answer this long after it went out is given up on the
  // next minute tick (Phase 5 review fix WR-B01).
  var REQUEST_LIMIT_MS = 10000;
  var NEARBY_CENTRE_KEY = 'pinalert.nearbyCentre';
  var DEFAULT_BASE = '/api';

  var pageEl = document.getElementById('official-page');
  var statusEl = document.getElementById('official-status');
  var noticeEl = document.getElementById('official-notice');
  var noticeTextEl = document.getElementById('official-notice-text');
  var sectionEl = document.getElementById('official-section');
  var titleEl = document.getElementById('official-title');
  var districtsEl = document.getElementById('official-districts');
  var listEl = document.getElementById('official-list');
  var skeletonEl = document.getElementById('official-skeleton');
  var emptyEl = document.getElementById('official-empty');
  var emptyTitleEl = document.getElementById('official-empty-title');
  var emptyBodyEl = document.getElementById('official-empty-body');
  var unavailableEl = document.getElementById('official-unavailable');
  var unavailableTitleEl = document.getElementById('official-unavailable-title');
  var unavailableBodyEl = document.getElementById('official-unavailable-body');
  var noareaEl = document.getElementById('official-noarea');
  var noareaTitleEl = document.getElementById('official-noarea-title');
  var noareaBodyEl = document.getElementById('official-noarea-body');
  var errorEl = document.getElementById('official-error');
  var errorTitleEl = document.getElementById('official-error-title');
  var errorBodyEl = document.getElementById('official-error-body');
  var retryEl = document.getElementById('official-retry');
  // The second muted status line, about hazard events from GDACS (plan 05-22,
  // ALERT-04 for each official feed). It is optional on purpose: the Demo's page
  // has no such line and keeps drawing without it, so it is not in the list below.
  var hazardEl = document.getElementById('official-hazard-status');

  var required = [pageEl, statusEl, noticeEl, noticeTextEl, sectionEl, titleEl, districtsEl, listEl, skeletonEl,
    emptyEl, emptyTitleEl, emptyBodyEl, unavailableEl, unavailableTitleEl, unavailableBodyEl, noareaEl,
    noareaTitleEl, noareaBodyEl, errorEl, errorTitleEl, errorBodyEl, retryEl];
  for (var i = 0; i < required.length; i += 1) {
    if (!required[i]) {
      return;
    }
  }

  var STATES = { fresh: true, stale: true, unavailable: true };
  var LANG_CODE_PATTERN = /^[a-z]{2,3}$/;
  // The feed's own names for the languages it sends, mapped to valid codes.
  var LANG_NAMES = { telugu: 'te', marathi: 'mr', malayalam: 'ml', hindi: 'hi', tamil: 'ta', english: 'en' };

  var current = null; // the last good answer drawn
  var renderedAt = 0; // Date.now() when it was drawn
  var serverNowAtRender = 0; // the server clock when it was drawn, in ms
  var requestOut = false;
  var lastLoadAt = 0; // Date.now() when the last request was sent
  var requestSeq = 0; // moves on with each request and when one is given up
  var controller = null; // aborts the request that is out, when the browser can
  var detailCount = 0;

  // ---- Times --------------------------------------------------------------

  var clockFormat = new Intl.DateTimeFormat('en-IN', { hour: 'numeric', minute: '2-digit', hour12: true });
  var dayFormat = new Intl.DateTimeFormat('en-IN', { year: 'numeric', month: 'numeric', day: 'numeric' });
  var weekdayFormat = new Intl.DateTimeFormat('en-IN', { weekday: 'short' });

  // clockText turns an ISO time into "2:05 pm" in the reader's own time zone,
  // built from the parts so a narrow no break space or an upper case am never
  // reaches the page. An unreadable time gives an empty string.
  function clockText(iso) {
    var ms = Date.parse(iso);
    if (!isFinite(ms)) {
      return '';
    }
    var hour = '';
    var minute = '';
    var period = '';
    var parts = clockFormat.formatToParts(new Date(ms));
    for (var k = 0; k < parts.length; k += 1) {
      if (parts[k].type === 'hour') {
        hour = parts[k].value;
      } else if (parts[k].type === 'minute') {
        minute = parts[k].value;
      } else if (parts[k].type === 'dayPeriod') {
        period = parts[k].value.toLowerCase();
      }
    }
    // A no break space (U+00A0) joins the minute and am or pm, so the time never
    // breaks before pm (Phase 5 review fix WR-T02). It is the plain no break
    // space, not the narrow one Intl writes.
    return hour + ':' + minute + '\u00a0' + period;
  }

  // clockWithDay is the clock time, with the short weekday in front when the day
  // is not the day of nowMs ("Sun 8:35 pm").
  function clockWithDay(iso, nowMs) {
    var ms = Date.parse(iso);
    var clock = clockText(iso);
    if (clock === '') {
      return '';
    }
    if (dayFormat.format(new Date(ms)) === dayFormat.format(new Date(nowMs))) {
      return clock;
    }
    return weekdayFormat.format(new Date(ms)) + ' ' + clock;
  }

  // relativeAge is the status line and notice wording for an age in seconds.
  function relativeAge(seconds) {
    if (typeof seconds !== 'number' || !isFinite(seconds) || seconds < 60) {
      return 'just now';
    }
    var minutes = Math.floor(seconds / 60);
    if (minutes < 60) {
      return minutes === 1 ? '1 minute ago' : minutes + ' minutes ago';
    }
    var hours = Math.floor(seconds / 3600);
    if (hours < 24) {
      return hours === 1 ? '1 hour ago' : hours + ' hours ago';
    }
    var days = Math.floor(seconds / 86400);
    return days === 1 ? '1 day ago' : days + ' days ago';
  }

  // relativeIssued is the wording of a row's Issued time (P5-C10).
  function relativeIssued(ageSeconds) {
    if (typeof ageSeconds !== 'number' || !isFinite(ageSeconds) || ageSeconds < 60) {
      return 'Just now';
    }
    var minutes = Math.floor(ageSeconds / 60);
    if (minutes < 60) {
      return minutes + ' min ago';
    }
    var hours = Math.floor(ageSeconds / 3600);
    if (hours < 24) {
      return hours === 1 ? '1 hour ago' : hours + ' hours ago';
    }
    var days = Math.floor(ageSeconds / 86400);
    return days === 1 ? '1 day ago' : days + ' days ago';
  }

  // untilText is the whole phrase of a row's end time: "Until 8:35 pm", with
  // "about" when the end was inferred and a weekday when it is not today.
  function untilText(iso, inferred, nowMs) {
    var when = clockWithDay(iso, typeof nowMs === 'number' ? nowMs : serverNowAtRender || Date.now());
    if (when === '') {
      return '';
    }
    return 'Until ' + (inferred ? 'about ' : '') + when;
  }

  function lowerFirst(text) {
    return text.charAt(0).toLowerCase() + text.slice(1);
  }

  // serverNow is the server's clock at the moment of the answer: the last good
  // check plus its age. Reading the age from the server keeps "6 min ago" right
  // on a device whose own clock is wrong.
  function serverNow(view) {
    var last = Date.parse(view.last_success_at);
    if (isFinite(last) && typeof view.age_seconds === 'number' && isFinite(view.age_seconds)) {
      return last + Math.max(0, view.age_seconds) * 1000;
    }
    return Date.now();
  }

  function ageOf(view, elapsedMs) {
    if (typeof view.age_seconds !== 'number' || !isFinite(view.age_seconds)) {
      return null;
    }
    return Math.max(0, view.age_seconds) + Math.max(0, elapsedMs || 0) / 1000;
  }

  // ---- The area -----------------------------------------------------------

  function validPoint(lat, lon) {
    return typeof lat === 'number' && typeof lon === 'number' && isFinite(lat) && isFinite(lon) &&
      Math.abs(lat) <= 90 && Math.abs(lon) <= 180;
  }

  function pageAttr(name) {
    var value = pageEl.getAttribute(name);
    return typeof value === 'string' ? value : '';
  }

  // areaSource answers {lat, lon} or null. The page's fixed point wins (the
  // Demo), then the tab's Nearby centre when it belongs to this page's account,
  // then the stored device fix. A value that is not a real point, or belongs to
  // another account, is ignored (T-05-48).
  function areaSource() {
    var fixedLat = pageAttr('data-fixed-lat');
    var fixedLon = pageAttr('data-fixed-lon');
    if (fixedLat !== '' && fixedLon !== '') {
      var a = Number(fixedLat);
      var b = Number(fixedLon);
      if (validPoint(a, b)) {
        return { lat: a, lon: b };
      }
    }
    var account = pageAttr('data-account-id');
    if (account !== '') {
      var raw = null;
      try {
        raw = window.sessionStorage.getItem(NEARBY_CENTRE_KEY);
      } catch (err) {
        raw = null;
      }
      if (raw) {
        try {
          var centre = JSON.parse(raw);
          if (centre && String(centre.account) === account && validPoint(centre.lat, centre.lon)) {
            return { lat: centre.lat, lon: centre.lon };
          }
        } catch (err) {
          // An unreadable value counts as no centre.
        }
      }
    }
    var position = window.PinalertPosition;
    if (position && typeof position.read === 'function') {
      try {
        var fix = position.read();
        if (fix && validPoint(fix.lat, fix.lon)) {
          return { lat: fix.lat, lon: fix.lon };
        }
      } catch (err) {
        // No stored fix.
      }
    }
    return null;
  }

  // apiBase is window.PinalertApiBase (set by api-base.js), else the page's own
  // data-api-base when it passes the same check as in api-base.js (T-05-74,
  // Phase 5 review fix IN-B04), else /api. The Demo fails closed (Phase 5
  // review fix CR-B02): a page marked data-demo="1" with no usable base answers
  // null and the page asks nothing, never the real /api.
  var SAFE_BASE = /^\/(?!\/)[A-Za-z0-9\/_-]*$/;

  function apiBase() {
    var base = window.PinalertApiBase;
    if (typeof base === 'string' && base !== '') {
      return base.replace(/\/+$/, '');
    }
    var own = pageAttr('data-api-base');
    if (SAFE_BASE.test(own) && own.replace(/\/+$/, '') !== '') {
      return own.replace(/\/+$/, '');
    }
    return pageAttr('data-demo') === '1' ? null : DEFAULT_BASE;
  }

  // ---- Showing and hiding -------------------------------------------------

  function setHidden(node, hidden) {
    node.hidden = hidden;
  }

  function hideBlocks() {
    var blocks = [noticeEl, sectionEl, skeletonEl, emptyEl, unavailableEl, noareaEl, errorEl];
    for (var k = 0; k < blocks.length; k += 1) {
      setHidden(blocks[k], true);
    }
  }

  function showState(block, titleNode, title, bodyNode, body) {
    titleNode.textContent = title;
    bodyNode.textContent = body;
    setHidden(block, false);
  }

  function showLoading() {
    hideBlocks();
    clearHazardLine();
    statusEl.textContent = 'Checking for official alerts…';
    setHidden(skeletonEl, false);
  }

  function renderError() {
    current = null;
    hideBlocks();
    listEl.textContent = '';
    statusEl.textContent = 'Not checked';
    clearHazardLine();
    retryEl.disabled = false;
    showState(errorEl, errorTitleEl, "Couldn't load official alerts", errorBodyEl, 'Check your connection and try again.');
  }

  // ---- Status line and sentences that carry an age ------------------------

  // clearHazardLine empties the hazard line and hides it, so a page that has no
  // answer claims nothing about hazard events.
  function clearHazardLine() {
    if (hazardEl) {
      hazardEl.textContent = '';
      hazardEl.hidden = true;
    }
  }

  // renderHazardLine writes the second status line from the hazard fields of the
  // answer (plan 05-22): when hazard events from GDACS were last checked, or that
  // they could not be checked since a time, or not yet. The SACHET line above it
  // never changes with it. A fresh feed gives the age; a stale or unavailable
  // feed with a last good check gives the time of that check; a feed never
  // checked says so. An answer with no hazard state (an older server) draws
  // nothing, and so does the Demo, whose page has no such line.
  function renderHazardLine(view, elapsedMs) {
    if (!hazardEl) {
      return;
    }
    var state = view.hazards_state;
    if (typeof state !== 'string' || STATES[state] !== true) {
      clearHazardLine();
      return;
    }
    var last = typeof view.hazards_last_success_at === 'string' ? Date.parse(view.hazards_last_success_at) : NaN;
    var haveAge = typeof view.hazards_age_seconds === 'number' && isFinite(view.hazards_age_seconds);
    var age = haveAge ? Math.max(0, view.hazards_age_seconds) + Math.max(0, elapsedMs || 0) / 1000 : null;
    var words;
    if (state === 'fresh' && age !== null) {
      words = 'Hazard events from GDACS: last checked ' + relativeAge(age);
    } else if (state !== 'fresh' && age !== null && isFinite(last)) {
      var now = last + Math.max(0, view.hazards_age_seconds) * 1000 + Math.max(0, elapsedMs || 0);
      words = 'Hazard events from GDACS could not be checked since ' + clockWithDay(view.hazards_last_success_at, now) + '.';
    } else {
      words = 'Hazard events from GDACS have not been checked yet.';
    }
    hazardEl.textContent = words;
    hazardEl.hidden = false;
  }

  // renderStatus writes the status line and the two sentences that carry the age
  // of the last good check (the stale notice and the unavailable body). elapsedMs
  // is the time that passed here since the answer was drawn.
  function renderStatus(view, elapsedMs) {
    renderHazardLine(view, elapsedMs);
    var age = ageOf(view, elapsedMs);
    var hasLast = isFinite(Date.parse(view.last_success_at));
    statusEl.textContent = age === null ? 'Not checked yet' : 'Last checked ' + relativeAge(age);
    if (age === null || !hasLast) {
      noticeTextEl.textContent = 'We could not check for alerts for some time.';
      unavailableBodyEl.textContent = 'Pinalert has not been able to check the official feed yet. ' +
        'Check your state disaster management authority for warnings.';
      return;
    }
    var now = serverNow(view) + Math.max(0, elapsedMs || 0);
    var since = clockWithDay(view.last_success_at, now);
    noticeTextEl.textContent = 'We could not check for alerts since ' + since + ' (' + relativeAge(age) + ').';
    unavailableBodyEl.textContent = 'Pinalert could not reach the official feed since ' + since + ' (' +
      relativeAge(age) + '). Check your state disaster management authority for warnings.';
  }

  // ---- Rows ---------------------------------------------------------------

  function make(tag, className, text) {
    var node = document.createElement(tag);
    if (className) {
      node.setAttribute('class', className);
    }
    if (typeof text === 'string') {
      node.textContent = text;
    }
    return node;
  }

  function levelClass(level) {
    if (level === 'extreme') {
      return 'official-chip--extreme';
    }
    if (level === 'severe') {
      return 'official-chip--severe';
    }
    return 'official-chip--neutral';
  }

  // langCode maps the feed's own language name to a valid code, or ''.
  function langCode(label) {
    if (typeof label !== 'string') {
      return '';
    }
    return LANG_NAMES[label.trim().toLowerCase()] || '';
  }

  // validLang is the code to write to a lang attribute, or ''. The server's code
  // wins; the name is used only when the server sent no code.
  function validLang(details) {
    var code = typeof details.original_lang_code === 'string' ? details.original_lang_code : '';
    if (code === '') {
      return langCode(details.original_lang_label);
    }
    return LANG_CODE_PATTERN.test(code) ? code : '';
  }

  function prefix(text) {
    return make('span', 'official-prefix', text);
  }

  function addPart(panel, label, texts, lang) {
    var part = make('div', 'official-detail');
    part.appendChild(make('p', 'official-detail__label', label));
    for (var k = 0; k < texts.length; k += 1) {
      var line = make('p', 'official-detail__text', texts[k]);
      if (lang !== '') {
        line.setAttribute('lang', lang);
      }
      part.appendChild(line);
    }
    panel.appendChild(part);
  }

  // buildDetails draws only the parts the feed carried (UI-SPEC 1d).
  function buildDetails(row, panel) {
    var d = row.details || {};
    if (d.description) {
      addPart(panel, 'What it says', [d.description], '');
    }
    if (d.instruction) {
      addPart(panel, 'What to do', [d.instruction], '');
    }
    if (d.original_headline || d.original_description) {
      var name = d.original_lang_label ? ' (' + d.original_lang_label + ')' : '';
      var originals = [];
      if (d.original_headline) {
        originals.push(d.original_headline);
      }
      if (d.original_description) {
        originals.push(d.original_description);
      }
      addPart(panel, 'Original text' + name, originals, validLang(d));
    }
    if (d.areas) {
      addPart(panel, 'Areas', [d.areas], '');
    }
    if (d.source_line) {
      addPart(panel, 'Source', [d.source_line], '');
    }
    if (row.expires_inferred) {
      panel.appendChild(make('p', 'official-detail__note', 'No end time was given.'));
    }
    if (row.grouped_count > 1) {
      panel.appendChild(make('p', 'official-detail__note',
        row.grouped_count + ' alerts like this are in force. This shows the newest.'));
    }
  }

  // renderRow builds one item: a button of spans (the row on the laptop, the card
  // on the phone) and, after it, the details panel.
  function renderRow(row) {
    detailCount += 1;
    var panelId = 'official-details-' + detailCount;
    var item = make('div', 'official-item');
    var button = make('button', 'official-row official-card');
    button.setAttribute('type', 'button');
    button.setAttribute('aria-expanded', 'false');
    button.setAttribute('aria-controls', panelId);

    var level = make('span', 'official-row__level');
    var chip = make('span', 'official-chip ' + levelClass(row.level));
    chip.appendChild(make('span', 'official-chip__icon'));
    chip.appendChild(make('span', 'official-chip__word', String(row.level_word || '')));
    level.appendChild(chip);
    if (row.demo) {
      level.appendChild(make('span', 'demo-tag', 'Demo'));
    }
    button.appendChild(level);

    var alert = make('span', 'official-row__alert');
    var headline = make('span', 'official-row__headline', String(row.headline || ''));
    if (row.original_language_only) {
      var lang = validLang(row.details || {});
      if (lang !== '') {
        headline.setAttribute('lang', lang);
      }
    }
    alert.appendChild(headline);
    if (row.original_language_only) {
      alert.appendChild(make('span', 'official-tag', 'Original language'));
    }
    if (row.area_text) {
      alert.appendChild(make('span', 'official-row__area', String(row.area_text)));
    }
    if (row.grouped_count > 1) {
      alert.appendChild(make('span', 'official-row__grouped', row.grouped_count + ' alerts like this'));
    }
    button.appendChild(alert);

    var issuer = make('span', 'official-row__issuer');
    issuer.appendChild(prefix('Issued by '));
    if (row.badge) {
      var badge = make('span', 'official-badge');
      badge.appendChild(make('span', 'official-badge__icon'));
      badge.appendChild(make('span', 'official-badge__word', String(row.badge)));
      issuer.appendChild(badge);
    }
    issuer.appendChild(make('span', 'official-row__issuer-name', String(row.issuer || '')));
    button.appendChild(issuer);

    var issued = make('span', 'official-row__issued');
    var issuedMs = Date.parse(row.issued_at);
    if (isFinite(issuedMs)) {
      issued.appendChild(prefix('Issued '));
      issued.appendChild(make('span', 'official-row__time', lowerFirst(relativeIssued((serverNowAtRender - issuedMs) / 1000))));
    }
    button.appendChild(issued);

    var until = make('span', 'official-row__until');
    var endClock = clockWithDay(row.expires_at, serverNowAtRender);
    if (endClock !== '') {
      until.appendChild(prefix('Until '));
      if (row.expires_inferred) {
        until.appendChild(make('span', 'official-about', 'about '));
      }
      until.appendChild(make('span', 'official-row__time', endClock));
    }
    button.appendChild(until);

    button.appendChild(make('span', 'official-row__chevron'));
    button.addEventListener('click', function () {
      toggleDetails(button);
    });

    var panel = make('div', 'official-details');
    panel.setAttribute('id', panelId);
    panel.hidden = true;
    buildDetails(row, panel);
    item.appendChild(button);
    item.appendChild(panel);
    return item;
  }

  // toggleDetails opens or closes the panel under a row.
  function toggleDetails(button) {
    var open = button.getAttribute('aria-expanded') === 'true';
    button.setAttribute('aria-expanded', open ? 'false' : 'true');
    var panel = document.getElementById(button.getAttribute('aria-controls'));
    if (panel) {
      panel.hidden = open;
    }
  }

  function countRows(view) {
    var total = 0;
    for (var k = 0; k < view.groups.length; k += 1) {
      var rows = view.groups[k] && view.groups[k].rows;
      total += Array.isArray(rows) ? rows.length : 0;
    }
    return total;
  }

  // renderGroups draws the column head row (laptop) and the groups in the order
  // the server sent them. A group with no rows is not drawn.
  function renderGroups(view) {
    listEl.textContent = '';
    if (countRows(view) === 0) {
      return;
    }
    var head = make('div', 'official-columns');
    head.setAttribute('aria-hidden', 'true');
    var names = ['Level', 'Alert', 'Issued by', 'Issued', 'Until'];
    for (var k = 0; k < names.length; k += 1) {
      head.appendChild(make('span', 'official-columns__name', names[k]));
    }
    listEl.appendChild(head);
    for (var g = 0; g < view.groups.length; g += 1) {
      var source = view.groups[g];
      if (!source || !Array.isArray(source.rows) || source.rows.length === 0) {
        continue;
      }
      var box = make('div', 'official-group');
      box.appendChild(make('h3', 'official-group__label', String(source.label || '')));
      for (var r = 0; r < source.rows.length; r += 1) {
        box.appendChild(renderRow(source.rows[r]));
      }
      listEl.appendChild(box);
    }
  }

  // ---- The page -----------------------------------------------------------

  function validView(view) {
    return !!view && typeof view === 'object' && STATES[view.state] === true && Array.isArray(view.groups);
  }

  function setTitle(text, count) {
    titleEl.textContent = text;
    if (count > 0) {
      var number = make('span', 'official-count', ' ' + count);
      titleEl.appendChild(number);
    }
  }

  // render draws one answer. Which block shows follows one order: unavailable,
  // then area unknown, then stale or fresh. Only a fresh answer with a known area
  // and no rows may say that no official alert is in force.
  function render(view) {
    if (!validView(view)) {
      renderError();
      return;
    }
    current = view;
    renderedAt = Date.now();
    serverNowAtRender = serverNow(view);
    hideBlocks();
    listEl.textContent = '';
    renderStatus(view, 0);

    if (view.state === 'unavailable') {
      unavailableTitleEl.textContent = 'Official alerts could not be checked';
      setHidden(unavailableEl, false);
      return;
    }
    if (view.state === 'stale') {
      setHidden(noticeEl, false);
    }
    if (view.area_known !== true) {
      showState(noareaEl, noareaTitleEl, 'Official alerts need your area', noareaBodyEl,
        'Open Nearby and allow location, or add a home area in Settings, then come back.');
      return;
    }
    var rows = countRows(view);
    if (view.state === 'fresh' && rows === 0) {
      showState(emptyEl, emptyTitleEl, 'No official alerts in force for your area', emptyBodyEl,
        'Nothing is in force for where you are or for your home area. ' +
        'Check your state disaster management authority for anything else.');
      return;
    }
    if (view.state === 'stale') {
      setTitle('Last known alerts', rows);
      districtsEl.textContent = 'As of ' + clockWithDay(view.last_success_at, serverNowAtRender);
      setHidden(districtsEl, false);
    } else {
      setTitle('In force now', rows);
      districtsEl.textContent = typeof view.districts_line === 'string' ? view.districts_line : '';
      setHidden(districtsEl, districtsEl.textContent === '');
    }
    renderGroups(view);
    setHidden(listEl, rows === 0);
    setHidden(sectionEl, false);
  }

  // load makes the one request and draws the answer. A press while a request is
  // out does nothing.
  function load() {
    if (requestOut) {
      return;
    }
    requestOut = true;
    lastLoadAt = Date.now();
    retryEl.disabled = true;
    showLoading();
    var point = areaSource();
    var base = apiBase();
    var target = base + '/official/alerts';
    if (point) {
      target += '?lat=' + encodeURIComponent(String(point.lat)) + '&lon=' + encodeURIComponent(String(point.lon));
    }
    if (base === null || typeof fetch !== 'function') {
      requestOut = false;
      renderError();
      return;
    }
    requestSeq += 1;
    var mine = requestSeq;
    controller = typeof AbortController === 'function' ? new AbortController() : null;
    var init = { credentials: 'same-origin', headers: { Accept: 'application/json' } };
    if (controller) {
      init.signal = controller.signal;
    }
    fetch(target, init).then(function (res) {
      if (!res.ok) {
        throw new Error('status ' + res.status);
      }
      return res.json();
    }).then(function (body) {
      if (mine !== requestSeq) {
        return;
      }
      requestOut = false;
      controller = null;
      retryEl.disabled = false;
      render(body);
    }, function () {
      if (mine !== requestSeq) {
        return;
      }
      requestOut = false;
      controller = null;
      renderError();
    });
  }

  // giveUpIfHung is the time limit of a request (Phase 5 review fix WR-B01). It
  // runs on the minute tick, the page's one timer (no second timer, so no
  // refetch loop): a request out for REQUEST_LIMIT_MS or more is aborted, its
  // late answer is dropped, and the page shows the load error, so a request
  // that never ends never blocks Try again or the next ask. The tick is not the
  // request's promise, which WebKit can leave unsettled.
  function giveUpIfHung() {
    if (!requestOut || Date.now() - lastLoadAt < REQUEST_LIMIT_MS) {
      return false;
    }
    requestSeq += 1;
    requestOut = false;
    if (controller) {
      controller.abort();
      controller = null;
    }
    renderError();
    return true;
  }

  // ---- Asking again (Phase 5 review fix CR-B01) ---------------------------
  //
  // An answer is only good while it is fresh (UI-SPEC 1b, T-34-17): the page
  // never keeps "No official alerts in force for your area" or "In force now" up
  // over an answer older than the server's 30 minute fresh limit. So the minute
  // tick asks again when a fresh answer reaches that limit, or when a row it
  // shows has passed its Until time since it was drawn; a stale or unavailable
  // answer is asked about again every few minutes, so it can recover or move on.
  // load() hides every block first, so no claim waits for the new answer.
  // Showing the tab again, or a page restored from the back and forward cache,
  // asks again when the last request is more than a minute old.

  function rowEndedSinceDrawn(view, elapsedMs) {
    var now = serverNowAtRender + Math.max(0, elapsedMs);
    for (var g = 0; g < view.groups.length; g += 1) {
      var rows = view.groups[g] && view.groups[g].rows;
      if (!Array.isArray(rows)) {
        continue;
      }
      for (var r = 0; r < rows.length; r += 1) {
        var end = rows[r] ? Date.parse(rows[r].expires_at) : NaN;
        if (isFinite(end) && end > serverNowAtRender && end <= now) {
          return true;
        }
      }
    }
    return false;
  }

  function answerTooOld(view, elapsedMs) {
    if (view.state !== 'fresh') {
      return elapsedMs >= RECHECK_NOT_FRESH_MS;
    }
    var age = ageOf(view, elapsedMs);
    if (age === null) {
      age = Math.max(0, elapsedMs) / 1000;
    }
    return age >= FRESH_LIMIT_SECONDS || rowEndedSinceDrawn(view, elapsedMs);
  }

  function tick() {
    if (giveUpIfHung() || !current) {
      return;
    }
    var elapsed = Date.now() - renderedAt;
    if (answerTooOld(current, elapsed)) {
      load();
      return;
    }
    renderStatus(current, elapsed);
  }

  function askIfOld() {
    if (Date.now() - lastLoadAt >= SHOWN_AGAIN_MS) {
      load();
    }
  }

  retryEl.addEventListener('click', load);
  setInterval(tick, STATUS_TICK_MS);
  if (typeof document.addEventListener === 'function') {
    document.addEventListener('visibilitychange', function () {
      if (document.visibilityState === 'visible') {
        askIfOld();
      }
    });
  }
  if (typeof window.addEventListener === 'function') {
    window.addEventListener('pageshow', function (event) {
      if (event && event.persisted === true) {
        askIfOld();
      }
    });
  }

  window.PinalertOfficialAlerts = {
    render: render,
    renderStatus: renderStatus,
    areaSource: areaSource,
    relativeIssued: relativeIssued,
    untilText: untilText
  };

  load();
}());
