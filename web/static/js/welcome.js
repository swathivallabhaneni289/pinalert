// welcome.js: the first-time flow at /welcome (Phase 3.4, plan 10, UI-SPEC
// Surface 7, D-27, D-28, D-24b).
//
// Self contained, like settings.js: it needs no other script of the app, only
// the ids welcome.html.tmpl renders, and it leaves quietly on a page that has no
// flow. It shows one of five steps at a time, moves focus to the step title,
// keeps Tab inside the dialog, and ignores Escape, because the flow is completed
// and not closed.
//
// What it records, and when:
//   - Agree (step 2) sends POST /api/me/consent and moves on only on a 204.
//     Disagree records nothing and shows a small box the page already holds
//     (its words are in welcome.html.tmpl): Read again scrolls the text box to
//     the top and focuses it, and Log out is a plain form posting to the app's
//     own /auth/logout, which this script never touches.
//   - Continue on step 4 sends PATCH /api/me/profile with only the fields the
//     person filled and then moves to step 5. Skip moves to step 5 and sends
//     nothing. Nothing is saved before that.
//   - The flow is recorded as done only from step 5 (Phase 3.3 plan 13, 03.4
//     D-27, decision 15): Allow alerts (once it has finished or been refused),
//     Not now and every Continue send POST /api/me/first-run-done.
//
// Step 2 holds Agree and Disagree back until the end of the text has been seen
// (quick 261027-jhd, owner, 2026-10-27), like the terms screen of any other app.
// The end is a marker inside the text box, watched by an IntersectionObserver
// whose root is the text box, so reading to the last line with VoiceOver reveals
// the buttons the same way scrolling does. This script alone sets the hold, and
// only when the reveal can run: the account has not agreed, the browser has an
// IntersectionObserver, and the row and the marker are on the page. Once shown,
// the buttons stay shown for the rest of the visit.
//
// Step 5 (Alerts) asks window.PinalertNotify (notify.js) what this device can
// do, on arrival and without a prompt, and shows one of four parts: ready,
// needs-install, blocked or unsupported. The needs-install part is one line
// (written in the template) under the title Alerts, with no install guide, which
// now lives only in Settings. It writes the words and notify.js writes none.
//
// What it never does:
//   - It never runs the browser's notification prompt except from the Allow
//     alerts tap: enable is called inside that click handler with nothing
//     awaited before it, and Not now and the Continue buttons never reach it.
//   - It asks the browser where the device is in one place only, Allow location
//     on step 3. The fix is saved only through PinalertPosition, in this browser,
//     and is sent nowhere. The server stores no reader location. Not now, the
//     other button of step 3, asks nothing and sends nothing.
//   - It sends no place search from step 3 (plan 33: step 3 is only Allow
//     location or Not now) and never fills the home area from the device fix or
//     from step 3. The home area is a place the person picked from the step 4
//     search. There is no reverse lookup. Text that was typed but never picked
//     is not a home area and is not sent.
//   - It never parses markup. Every string, the person's own name and place
//     names from the geocoder included, reaches the page through textContent or
//     an input value.
(function () {
  'use strict';

  var root = document.getElementById('welcome');
  if (!root) {
    return;
  }

  var CONSENT_URL = '/api/me/consent';
  var PROFILE_URL = '/api/me/profile';
  var DONE_URL = '/api/me/first-run-done';
  // Step 4 uses the place search of Settings, asking for areas only (kind=area:
  // neighbourhoods, localities, towns, cities, districts, never a street or a
  // building, since the stored home area is a cell about 5 km wide). It has its
  // own budget on the server.
  var STEP4_SEARCH_URL = '/api/geocode/place?kind=area&q=';
  var FOCUS_SEARCH_KEY = 'pinalert.focusSearch';
  var LAND_ON = '/';
  var SAVE_TIMEOUT_MS = 10000;
  var SEARCH_TIMEOUT_MS = 10000;
  var LOCATION_TIMEOUT_MS = 8000;
  var MIN_QUERY_RUNES = 3;
  var MAX_RESULTS = 5;
  // Suggestions while typing in the home area (step 4 only). The search runs
  // once the person stops typing for TYPING_PAUSE_MS, and typed searches go out
  // at least TYPING_GAP_MS apart, so typing alone spends the per account search
  // budget (a burst of 8, then 1 a second) no faster than it refills and can
  // never reach Too many searches. A typed search the server refuses or finds
  // busy (429 or 503) is tried again quietly, at most QUIET_RETRIES times,
  // RETRY_WAIT_MS apart.
  var TYPING_PAUSE_MS = 400;
  var TYPING_GAP_MS = 1000;
  var QUIET_RETRIES = 3;
  var RETRY_WAIT_MS = 1000;

  var MESSAGE_AGREE_FAILED = 'Could not save your agreement. Try again in a moment.';
  var MESSAGE_AGREE_FIRST = 'Please agree to the Disclaimer and policy to continue.';
  var MESSAGE_SAVE_FAILED = 'Could not save. Try again in a moment.';
  var MESSAGE_SESSION_ENDED = 'Your session has ended. Reload the page and log in again.';
  // The search wording is the same as Settings and the map search, so one
  // sentence means one thing across the app.
  var MESSAGE_SEARCHING = 'Searching…';
  var MESSAGE_TOO_SHORT = 'Type at least 3 letters.';
  // Shown when no area matches closely and no typed row could be placed (plan
  // 34), with the rows still listed when there are any. The example is the
  // owner's own neighbourhood.
  var MESSAGE_NOT_FOUND = 'Not found. Add your town, for example: Gayathri Nagar, Vijayawada';
  var MESSAGE_UNAVAILABLE = 'Search unavailable. Try again in a moment.';
  var MESSAGE_RATE_LIMITED = 'Too many searches. Wait a few seconds and try again.';
  // Step 5 (UI-SPEC P33-C7, C9, C10, C15). The sentences of the ready,
  // needs-install, blocked and unsupported parts, and the example notification,
  // are in the template.
  var ALERTS_STEP = 5;
  // The owner, on 2026-10-27, made the needs-install part gentler: the step's own
  // name above the one line the template holds.
  var TITLE_LATER = 'Alerts';
  // Step 2: the attribute that holds Agree and Disagree back, and the margin the
  // end marker is watched with. The margin is about the text box's bottom
  // padding, so the 1px marker under the last line counts as reached when
  // VoiceOver scrolls that line just into view.
  var UNREAD_ATTR = 'data-unread';
  var END_MARGIN = '0px 0px 24px 0px';
  var MESSAGE_ALERTS_OFF = 'Alerts are off on this device.';
  var MESSAGE_ALERTS_FAILED = 'Couldn\'t turn alerts on. Try again.';
  var MESSAGE_ALERTS_OFFLINE = 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.';

  var FOCUSABLE_SELECTOR = 'button, input, [tabindex]:not([tabindex="-1"])';

  function el(id) {
    return document.getElementById(id);
  }

  // The steps that render. The count is read from the page, so a step added
  // later is counted without a change here.
  var stepCount = 0;
  while (el('welcome-step-' + (stepCount + 1))) {
    stepCount += 1;
  }

  var stepItems = root.querySelectorAll('.welcome__steps-item');
  // The laptop side panel, which is not drawn on the phone (CSS), and the
  // buttons in its step list. A button in a panel that is not drawn is not in
  // the Tab loop.
  var sideNode = root.querySelectorAll('.welcome__side')[0] || null;

  var disagreeBox = el('welcome-disagree-message');
  var scrollbox = el('welcome-scrollbox');
  var consentActions = el('welcome-consent-actions');
  var scrollEnd = el('welcome-scrollbox-end');
  var endWatcher = null;
  var consentError = el('welcome-error-2');
  var agreeButton = el('welcome-agree');
  var allowButton = el('welcome-allow-location');
  var notNowButton = el('welcome-not-now');
  var nameInput = el('welcome-name-input');
  var homeInput = el('welcome-home-input');
  var profileError = el('welcome-error-4');
  var continueButton = el('welcome-continue-4');
  var skipButton = el('welcome-skip');

  // Step 5, Alerts. The public key rides on the flow root; it is empty when the
  // server has no key, which step 5 shows as a device that cannot show alerts.
  var alertsStep = el('welcome-step-5');
  var alertsTitle = el('welcome-step-5-title');
  var alertsReady = el('welcome-alerts-ready');
  var alertsInstall = el('welcome-alerts-install');
  var alertsBlocked = el('welcome-alerts-blocked');
  var alertsUnsupported = el('welcome-alerts-unsupported');
  var alertsNoHome = el('welcome-alerts-nohome');
  var alertsStatus = el('welcome-alerts-status');
  var alertsError = el('welcome-alerts-error');
  var alertsAllow = el('welcome-alerts-allow');
  var alertsLater = el('welcome-alerts-later');
  var alertsContinue = el('welcome-alerts-continue');
  var pushKey = root.getAttribute('data-push-key') || '';
  // The title the template draws, kept so the install variant can put it back.
  var alertsTitleReady = alertsTitle ? alertsTitle.textContent : '';

  // The step a person must agree on, and the highest step an address may name
  // for an account that has not agreed yet. A person may look at any step in
  // any order, but finishing needs the agreement (the server holds the same rule).
  var CONSENT_STEP = 2;

  var current = 0;
  // Whether the current terms are agreed: true when the server started the flow
  // past the agreement step, or after Agree was saved on this page.
  var agreed = false;
  var agreeing = false;
  var locating = false;
  var finishing = false;

  // The values the page was drawn with, so only what the person changed is sent.
  var storedName = nameInput ? nameInput.value.trim() : '';

  // The place picked for the home area from step 4's own search. Only a picked
  // place is ever sent as a home area.
  var homePick = null;

  // Whether the account has a home area: the page says so (the line "Set your
  // home area to get these." is hidden when it does), and it turns true once a
  // picked place is saved in step 4. It decides that line on step 5.
  var hasHomeArea = alertsNoHome ? alertsNoHome.hidden : false;

  // Step 5 state. alertsSeq numbers each arrival at step 5, so a late device
  // answer is dropped when the person has left or the step is busy or refused.
  var alertsSeq = 0;
  var alertsBusy = false;
  var alertsRefused = false;

  // ---- Small helpers --------------------------------------------------------

  function setText(node, value) {
    node.textContent = value === null || value === undefined ? '' : String(value);
  }

  function showLine(node, message) {
    setText(node, message);
    node.hidden = false;
  }

  function hideLine(node) {
    setText(node, '');
    node.hidden = true;
  }

  function runeCount(text) {
    return Array.from(text).length;
  }

  // isShown says whether node and every ancestor up to the root are visible, so
  // the focus trap skips the controls of a hidden step and a hidden search, and
  // the Agree and Disagree row while it is held back (it carries UNREAD_ATTR).
  function isShown(node) {
    for (var n = node; n && n !== root; n = n.parentNode) {
      if (n.hidden || (typeof n.hasAttribute === 'function' && n.hasAttribute(UNREAD_ATTR))) {
        return false;
      }
    }
    return true;
  }

  // sideShown says whether the laptop side panel is drawn. Without a way to ask
  // the browser, it counts as drawn.
  function sideShown() {
    if (sideNode === null) {
      return false;
    }
    if (typeof window.getComputedStyle !== 'function') {
      return true;
    }
    return window.getComputedStyle(sideNode).display !== 'none';
  }

  function focusables() {
    var all = root.querySelectorAll(FOCUSABLE_SELECTOR);
    var panel = sideShown();
    var out = [];
    for (var i = 0; i < all.length; i++) {
      if (all[i].disabled || !isShown(all[i])) {
        continue;
      }
      if (!panel && sideNode !== null && sideNode.contains(all[i])) {
        continue;
      }
      out.push(all[i]);
    }
    return out;
  }

  // request sends one request with a time limit that also covers reading the
  // body, so a request WebKit leaves unsettled still ends. It resolves to
  // {status, ok, body} and rejects on a network failure or the limit.
  function request(url, init, ms) {
    var controller = typeof AbortController === 'function' ? new AbortController() : null;
    if (controller !== null) {
      init.signal = controller.signal;
    }
    var timer = null;
    var expired = new Promise(function (resolve, reject) {
      timer = window.setTimeout(function () {
        timer = null;
        if (controller !== null) {
          controller.abort();
        }
        reject(new Error('no answer in time'));
      }, ms);
    });
    expired.catch(function () {});
    function clear() {
      if (timer !== null) {
        window.clearTimeout(timer);
        timer = null;
      }
    }
    var work = fetch(url, init).then(function (res) {
      return res.json().then(function (body) {
        return body;
      }, function () {
        return {};
      }).then(function (body) {
        return { status: res.status, ok: res.ok, body: body };
      });
    });
    var promise = Promise.race([work, expired]).then(function (answer) {
      clear();
      return answer;
    }, function (err) {
      clear();
      throw err;
    });
    return {
      promise: promise,
      abort: function () {
        clear();
        if (controller !== null) {
          controller.abort();
        }
      }
    };
  }

  // serverSentence is the server's own sentence for a refused request, when it
  // sent one of a sensible length.
  function serverSentence(body) {
    var sent = body && body.error && body.error.message;
    if (typeof sent === 'string' && sent !== '' && sent.length <= 200) {
      return sent;
    }
    return '';
  }

  function saveMessage(answer) {
    if (answer.status === 400 && serverSentence(answer.body) !== '') {
      return serverSentence(answer.body);
    }
    if (answer.status === 401) {
      return MESSAGE_SESSION_ENDED;
    }
    return MESSAGE_SAVE_FAILED;
  }

  // ---- Steps ----------------------------------------------------------------

  function renderStepList(step) {
    for (var i = 0; i < stepItems.length; i++) {
      var item = stepItems[i];
      var n = parseInt(item.getAttribute('data-step'), 10);
      item.setAttribute('data-state', n < step ? 'done' : (n === step ? 'current' : 'waiting'));
      // The current step is marked on its button, which is what a screen
      // reader lands on.
      var mark = item.querySelectorAll('button')[0] || item;
      if (n === step) {
        mark.setAttribute('aria-current', 'step');
      } else {
        mark.removeAttribute('aria-current');
      }
    }
  }

  // ---- History: the step is in the address ------------------------------------

  var canPush = !!(window.history && typeof window.history.pushState === 'function' &&
    typeof window.history.replaceState === 'function');

  function stepAddress(n) {
    return '?step=' + n;
  }

  // addressStep reads ?step=N from the address, or 0 when there is none.
  function addressStep() {
    var search = window.location && typeof window.location.search === 'string' ? window.location.search : '';
    var found = /[?&]step=(\d+)(?:&|$)/.exec(search);
    return found ? parseInt(found[1], 10) : 0;
  }

  // The first step to show. The server names it (data-start-step). An address
  // step is honoured only when the account may see it: a person who has not
  // agreed can be sent to step 1 or 2 and never past the agreement, so a reload
  // never skips it. The server's rule for finishing is unchanged either way.
  function firstStep(serverStep) {
    var wanted = addressStep();
    var most = serverStep > CONSENT_STEP ? stepCount : CONSENT_STEP;
    if (wanted >= 1 && wanted <= most && wanted <= stepCount) {
      return wanted;
    }
    return serverStep;
  }

  // goTo shows one step. A step that does not exist is refused. Changing step
  // puts the Disagree box away and clears the agree line, and focus moves to the new
  // step's title. how says what to do with the browser history: undefined adds
  // an entry when the step changes, 'replace' rewrites the current entry (the
  // first load), 'none' leaves it alone (the browser's own back and forward).
  function goTo(step, how) {
    var n = parseInt(step, 10);
    if (!isFinite(n) || n < 1 || n > stepCount) {
      return;
    }
    var previous = current;
    current = n;
    if (canPush && how !== 'none') {
      if (how === 'replace') {
        window.history.replaceState({ pinalertStep: n }, '', stepAddress(n));
      } else if (n !== previous) {
        window.history.pushState({ pinalertStep: n }, '', stepAddress(n));
      }
    }
    for (var i = 1; i <= stepCount; i++) {
      el('welcome-step-' + i).hidden = i !== n;
    }
    root.setAttribute('data-step', String(n));
    root.setAttribute('aria-labelledby', 'welcome-step-' + n + '-title');
    renderStepList(n);
    hideDisagreeBox();
    hideLine(consentError);
    if (previous === ALERTS_STEP && n !== ALERTS_STEP) {
      // An answer still on its way about this device is for a step now left.
      alertsSeq += 1;
    }
    if (n === ALERTS_STEP) {
      // Drawn before focus moves, so a screen reader reads the right title.
      enterAlerts();
    }
    el('welcome-step-' + n + '-title').focus();
  }

  // ---- Step 2: Agree and Disagree -------------------------------------------

  // The Disagree box is markup the template draws, with its own buttons, so it is
  // only shown and hidden here. It is never passed to showLine or hideLine, which
  // would replace everything inside it with text.
  function hideDisagreeBox() {
    disagreeBox.hidden = true;
  }

  // Disagree records nothing. It shows the box and moves focus into it, so a
  // screen reader reads it out, and brings it into view on a short screen.
  function disagree() {
    disagreeBox.hidden = false;
    disagreeBox.focus();
    if (typeof disagreeBox.scrollIntoView === 'function') {
      disagreeBox.scrollIntoView({ block: 'nearest' });
    }
  }

  // Read again takes the person back to the start of the text, with focus in the
  // text box so the arrow keys scroll it. It sends nothing.
  function readAgain() {
    scrollbox.scrollTop = 0;
    scrollbox.focus();
  }

  // revealConsent shows Agree and Disagree for good: the hold comes off and the
  // watcher is dropped. Nothing ever sets the hold again, and focus is left where
  // the person is.
  function revealConsent() {
    if (consentActions) {
      consentActions.removeAttribute(UNREAD_ATTR);
    }
    if (endWatcher !== null) {
      try {
        endWatcher.disconnect();
      } catch (err) {
        // The buttons are shown either way.
      }
      endWatcher = null;
    }
  }

  // watchForEnd holds Agree and Disagree back until the end marker has been in
  // view. It does nothing where the reveal could not run (the account has agreed,
  // the page lacks the row or the marker, or the browser has no
  // IntersectionObserver), so a page whose script fails never hides Agree. A
  // browser that cannot watch shows the buttons at once.
  function watchForEnd() {
    if (agreed || !consentActions || !scrollEnd || typeof window.IntersectionObserver !== 'function') {
      return;
    }
    consentActions.setAttribute(UNREAD_ATTR, '');
    try {
      endWatcher = new window.IntersectionObserver(function (entries) {
        for (var i = 0; i < entries.length; i++) {
          if (entries[i].isIntersecting) {
            revealConsent();
            return;
          }
        }
      }, { root: scrollbox, rootMargin: END_MARGIN, threshold: 0 });
      endWatcher.observe(scrollEnd);
    } catch (err) {
      revealConsent();
    }
  }

  function agree() {
    if (agreeing) {
      return Promise.resolve();
    }
    hideDisagreeBox();
    agreeing = true;
    agreeButton.disabled = true;
    hideLine(consentError);
    return request(CONSENT_URL, { method: 'POST', credentials: 'same-origin' }, SAVE_TIMEOUT_MS).promise.then(function (answer) {
      agreeing = false;
      agreeButton.disabled = false;
      if (answer.status === 204) {
        agreed = true;
        goTo(3);
        return;
      }
      showLine(consentError, answer.status === 401 ? MESSAGE_SESSION_ENDED : MESSAGE_AGREE_FAILED);
    }, function () {
      agreeing = false;
      agreeButton.disabled = false;
      showLine(consentError, MESSAGE_AGREE_FAILED);
    });
  }

  // ---- Step 3: location ------------------------------------------------------

  // allowLocation is the only place this file asks for the device position. The
  // fix is saved in this browser through PinalertPosition and goes nowhere
  // else. A refusal, a failure or a missing API changes nothing but the step.
  function allowLocation() {
    if (locating) {
      return;
    }
    if (!navigator.geolocation) {
      goTo(4);
      return;
    }
    locating = true;
    allowButton.disabled = true;
    // The question is about to be asked, so Nearby and the Low data view will
    // not ask it again on their own (plan 36).
    if (window.PinalertPosition && typeof window.PinalertPosition.markAsked === 'function') {
      try {
        window.PinalertPosition.markAsked();
      } catch (err) {
        // The flow does not depend on the marker.
      }
    }
    function done() {
      locating = false;
      allowButton.disabled = false;
    }
    navigator.geolocation.getCurrentPosition(function (pos) {
      done();
      var coords = pos && pos.coords;
      if (coords && window.PinalertPosition) {
        try {
          window.PinalertPosition.save(coords.latitude, coords.longitude);
        } catch (err) {
          // The flow does not depend on the remembered fix.
        }
      }
      goTo(4);
    }, function () {
      done();
      goTo(4);
    }, { timeout: LOCATION_TIMEOUT_MS });
  }

  // notNow is the other choice of step 3: nothing is asked of the browser,
  // nothing is saved and nothing is sent, and the flow goes on to step 4.
  function notNow() {
    goTo(4);
  }

  // ---- Place search (step 4) ------------------------------------------------

  function placeShortName(result) {
    if (typeof result.name === 'string' && result.name.trim() !== '') {
      return result.name.trim();
    }
    if (typeof result.display_name === 'string') {
      return result.display_name.split(',')[0].trim();
    }
    return '';
  }

  // makeSearch builds one place search over a field, a status line and a list.
  // cfg: {input, form, status, results, url, typing, onPick(place), onClear()}.
  // A pick is only ever a result of the search, so a place is never anything the
  // person typed or the device reported. A failure shows one plain line and
  // never blocks the step.
  //
  // With cfg.typing the search also runs by itself while the person types (see
  // TYPING_PAUSE_MS above). A typed search is quiet: no Searching line, no too
  // short line and no failure line, since the person did not ask for it and
  // their own typing must never read as Search unavailable. Enter shows every
  // line, as before, and takes over a typed search already out for the same
  // text instead of asking again.
  function makeSearch(cfg) {
    var picked = null;
    var seq = 0;
    var inFlight = null;
    var inFlightText = '';
    // Whether the search in flight was started by typing and not by Enter.
    var inFlightQuiet = false;
    // The pending typing pause or quiet retry, or null.
    var typingTimer = null;
    // Set while the gap after the last search runs; a typed search that comes
    // due meanwhile waits in waitingText.
    var coolTimer = null;
    var waitingText = null;
    // The text the last request was sent for, so the same text is never sent
    // twice in a row by typing.
    var lastSentText = '';
    // How many quiet retries the text typed last has used.
    var quietFailures = 0;

    function clearPick() {
      if (picked === null) {
        return;
      }
      picked = null;
      var options = cfg.results.querySelectorAll('button');
      for (var i = 0; i < options.length; i++) {
        options[i].setAttribute('aria-selected', 'false');
      }
      cfg.onClear();
    }

    function cancel() {
      seq += 1;
      if (inFlight !== null) {
        inFlight.abort();
        inFlight = null;
      }
      inFlightText = '';
      inFlightQuiet = false;
    }

    // cancelTyping drops a pending typing pause, quiet retry or search waiting
    // for the gap. The gap itself keeps running.
    function cancelTyping() {
      if (typingTimer !== null) {
        window.clearTimeout(typingTimer);
        typingTimer = null;
      }
      waitingText = null;
    }

    function pick(option, place) {
      // A pick ends a typing pause or a quiet retry, so a late answer never
      // reopens the list over the pick.
      cancelTyping();
      cancel();
      var options = cfg.results.querySelectorAll('button');
      for (var i = 0; i < options.length; i++) {
        options[i].setAttribute('aria-selected', 'false');
      }
      picked = place;
      option.setAttribute('aria-selected', 'true');
      cfg.input.value = place.name;
      // A pick closes the suggestions (owner, 2026-10-18: they must go away
      // once one is chosen) and gives focus back to the field.
      cfg.results.hidden = true;
      // The status line goes with the list: a Not found line left over from the
      // search that offered this pick no longer applies (plan 35).
      hideLine(cfg.status);
      cfg.input.focus();
      cfg.onPick(place);
    }

    // noMatch is the server's no_match flag: no row closely matches what was
    // typed, no typed row could be placed, and every lookup that was needed
    // answered. unfinished is the server's unfinished flag: nothing matches
    // closely but a lookup that was needed failed (plan 35). quiet says the
    // typing pause started the search. An unfinished search, or no rows with
    // no_match not true, never says Not found: Enter says Search unavailable
    // (rows stay listed), a typed search says nothing.
    function render(results, noMatch, unfinished, quiet) {
      cfg.results.textContent = '';
      var shown = 0;
      var typedShown = false;
      for (var i = 0; i < results.length && shown < MAX_RESULTS; i++) {
        var res = results[i];
        var name = res ? placeShortName(res) : '';
        if (name === '' || typeof res.lat !== 'number' || typeof res.lon !== 'number' ||
            !isFinite(res.lat) || !isFinite(res.lon) ||
            Math.abs(res.lat) > 90 || Math.abs(res.lon) > 180) {
          continue;
        }
        shown += 1;
        var item = document.createElement('li');
        item.setAttribute('role', 'presentation');
        var option = document.createElement('button');
        option.setAttribute('type', 'button');
        option.setAttribute('role', 'option');
        option.setAttribute('aria-selected', 'false');
        option.className = 'app-window__option';
        // typed marks the one row that is the person's own text, placed in the
        // town they typed (kind=area only; plan 32). It reads Use "<name>" with
        // the town on the second line, and picks like any other row: the name
        // is the cleaned text to save, the point is a place in that town.
        var typed = res.typed === true;
        typedShown = typedShown || typed;
        var label = document.createElement('span');
        setText(label, typed ? 'Use "' + name + '"' : name);
        option.appendChild(label);
        var second = typed && typeof res.town === 'string' && res.town.trim() !== '' ? res.town.trim() : res.display_name;
        if (typeof second === 'string' && second !== '' && second !== name) {
          var sub = document.createElement('span');
          sub.className = 'app-window__option-sub';
          setText(sub, second);
          option.appendChild(sub);
        }
        // area is the server's name for the area the place lies in, or ''.
        var area = typeof res.area === 'string' ? res.area.trim() : '';
        option.addEventListener('click', pick.bind(null, option, { name: name, lat: res.lat, lon: res.lon, area: area }));
        item.appendChild(option);
        cfg.results.appendChild(item);
      }
      if (shown === 0) {
        cfg.results.hidden = true;
        if (noMatch) {
          showLine(cfg.status, MESSAGE_NOT_FOUND);
        } else if (quiet) {
          // An empty list the server did not call not found means a lookup did
          // not finish (plan 35): a typed search says nothing about it.
          hideLine(cfg.status);
        } else {
          showLine(cfg.status, MESSAGE_UNAVAILABLE);
        }
        return;
      }
      // A typed row is a match, so the line never goes with one.
      if (noMatch && !typedShown) {
        showLine(cfg.status, MESSAGE_NOT_FOUND);
      } else if (unfinished && !quiet && !typedShown) {
        showLine(cfg.status, MESSAGE_UNAVAILABLE);
      } else {
        hideLine(cfg.status);
      }
      cfg.results.hidden = false;
    }

    // search runs one search for the text in the field. It sends the text only.
    // typed is true when the typing pause started it (see the quiet rules above).
    function search(query, typed) {
      var quiet = typed === true;
      var q = String(query === undefined ? cfg.input.value : query).trim();
      // Enter, or the typed search itself, ends the pending pause.
      cancelTyping();
      if (runeCount(q) < MIN_QUERY_RUNES) {
        cancel();
        if (!quiet) {
          cfg.results.hidden = true;
          showLine(cfg.status, MESSAGE_TOO_SHORT);
        }
        return Promise.resolve();
      }
      if (inFlight !== null && q === inFlightText) {
        if (!quiet && inFlightQuiet) {
          inFlightQuiet = false;
          clearPick();
          cfg.results.hidden = true;
          showLine(cfg.status, MESSAGE_SEARCHING);
        }
        return inFlight.promise.then(function () {}, function () {});
      }
      cancel();
      if (!quiet) {
        clearPick();
        cfg.results.hidden = true;
        showLine(cfg.status, MESSAGE_SEARCHING);
      }
      seq += 1;
      var mine = seq;
      inFlightText = q;
      inFlightQuiet = quiet;
      lastSentText = q;
      if (cfg.typing) {
        startGap();
      }
      var call = request(cfg.url + encodeURIComponent(q), { credentials: 'same-origin' }, SEARCH_TIMEOUT_MS);
      inFlight = call;
      function finish() {
        if (mine === seq) {
          inFlight = null;
          inFlightText = '';
        }
      }
      return call.promise.then(function (answer) {
        if (mine !== seq) {
          return;
        }
        finish();
        if (!answer.ok) {
          // The same text may be sent again after a failure.
          lastSentText = '';
          var busy = answer.status === 429 || answer.status === 503;
          if (inFlightQuiet && busy) {
            // A refusal for text no longer in the box, or one that lands while
            // newer text waits for its pause, is dropped quietly.
            if (typingTimer !== null || waitingText !== null || q !== String(cfg.input.value).trim()) {
              return;
            }
            if (quietFailures < QUIET_RETRIES) {
              quietFailures += 1;
              retryLater(q);
              return;
            }
          }
          if (!inFlightQuiet || busy) {
            if (inFlightQuiet) {
              cfg.results.hidden = true;
            }
            var sent = serverSentence(answer.body);
            showLine(cfg.status, sent !== '' ? sent : (answer.status === 429 ? MESSAGE_RATE_LIMITED : MESSAGE_UNAVAILABLE));
          }
          return;
        }
        render(answer.body && Array.isArray(answer.body.results) ? answer.body.results : [],
          !!answer.body && answer.body.no_match === true,
          !!answer.body && answer.body.unfinished === true, inFlightQuiet);
      }, function () {
        if (mine !== seq) {
          return;
        }
        finish();
        lastSentText = '';
        if (!inFlightQuiet) {
          showLine(cfg.status, MESSAGE_UNAVAILABLE);
        }
      });
    }

    // startGap starts the wait before the next typed search may go out. A typed
    // search that comes due meanwhile is held in waitingText and sent when the
    // gap ends.
    function startGap() {
      if (coolTimer !== null) {
        window.clearTimeout(coolTimer);
      }
      coolTimer = window.setTimeout(function () {
        coolTimer = null;
        if (waitingText !== null) {
          var text = waitingText;
          waitingText = null;
          typedSearch(text);
        }
      }, TYPING_GAP_MS);
    }

    function typedSearch(q) {
      if (coolTimer !== null) {
        waitingText = q;
        return;
      }
      search(q, true);
    }

    // retryLater asks for q again, quietly. It waits on the typing timer, so
    // anything that ends a typing pause also ends it.
    function retryLater(q) {
      cancelTyping();
      typingTimer = window.setTimeout(function () {
        typingTimer = null;
        search(q, true);
      }, RETRY_WAIT_MS);
    }

    // onTyping is the typing path: every keystroke restarts the pause, and the
    // search runs once the person stops for TYPING_PAUSE_MS with at least 3
    // letters in the field and text that was not the last one sent. Under 3
    // letters it drops any search and its results quietly.
    function onTyping() {
      cancelTyping();
      quietFailures = 0;
      var q = String(cfg.input.value).trim();
      if (runeCount(q) < MIN_QUERY_RUNES) {
        cancel();
        lastSentText = '';
        cfg.results.hidden = true;
        hideLine(cfg.status);
        return;
      }
      if (q === lastSentText) {
        return;
      }
      typingTimer = window.setTimeout(function () {
        typingTimer = null;
        typedSearch(q);
      }, TYPING_PAUSE_MS);
    }

    cfg.form.addEventListener('submit', function (e) {
      e.preventDefault();
      search();
    });
    // Typing after a pick forgets the pick, so what is sent is always a place
    // that was picked and not text that was changed afterwards.
    cfg.input.addEventListener('input', function () {
      if (picked !== null && cfg.input.value !== picked.name) {
        clearPick();
      }
      if (cfg.typing) {
        onTyping();
      }
    });

    return { search: search, getPick: function () { return picked; } };
  }

  // The home area search of step 4, the only place search of this page. Typing
  // after a pick forgets the pick (makeSearch calls onClear), so text that was
  // changed is never sent as a place.
  var homeSearch = null;
  if (el('welcome-home-form')) {
    homeSearch = makeSearch({
      input: homeInput,
      form: el('welcome-home-form'),
      status: el('welcome-home-status'),
      results: el('welcome-home-results'),
      url: STEP4_SEARCH_URL,
      typing: true,
      onPick: function (place) { homePick = place; },
      onClear: function () { homePick = null; }
    });
  }

  // ---- Step 4: finishing ----------------------------------------------------

  function setFinishing(on) {
    finishing = on;
    continueButton.disabled = on;
    skipButton.disabled = on;
    setAlertsBusy(on);
  }

  function leave() {
    try {
      window.sessionStorage.setItem(FOCUS_SEARCH_KEY, '1');
    } catch (err) {
      // The flow still ends; the search just is not focused.
    }
    window.location.replace(LAND_ON);
  }

  function postDone() {
    return request(DONE_URL, { method: 'POST', credentials: 'same-origin' }, SAVE_TIMEOUT_MS).promise.then(function (answer) {
      if (answer.status === 204) {
        leave();
        return;
      }
      setFinishing(false);
      if (answer.status === 409) {
        // The server has no agreement for the current terms: go back to it.
        goTo(2);
        showLine(consentError, MESSAGE_AGREE_FIRST);
        return;
      }
      showLine(doneErrorLine(), answer.status === 401 ? MESSAGE_SESSION_ENDED : MESSAGE_SAVE_FAILED);
    }, function () {
      setFinishing(false);
      showLine(doneErrorLine(), MESSAGE_SAVE_FAILED);
    });
  }

  // doneErrorLine is the line a failed first-run-done answers on: step 5's own,
  // since the flow is recorded as done only from there.
  function doneErrorLine() {
    return current === ALERTS_STEP ? alertsError : profileError;
  }

  // profileBody builds the PATCH body from what the person filled: a changed
  // name, and a home area only when it is a picked place.
  function profileBody() {
    var body = {};
    var name = nameInput.value.trim();
    if (name !== '' && name !== storedName) {
      body.display_name = name;
    }
    if (homePick !== null) {
      body.home_area = { name: homePick.name, lat: homePick.lat, lon: homePick.lon };
    }
    return body;
  }

  // toAlerts leaves step 4 for step 5. The step 4 buttons are freed first, so
  // they work again when the person comes back with Back.
  function toAlerts() {
    setFinishing(false);
    goTo(5);
  }

  // finish is step 4's way on. With save true it first sends the profile fields
  // the person filled (no request when there are none); with save false (Skip)
  // it saves nothing. Then it shows step 5. The flow is not recorded as done
  // here: only step 5 does that (decision 15).
  function finish(save) {
    if (finishing) {
      return Promise.resolve();
    }
    // A person can look at step 4 before agreeing, but cannot finish without
    // it: show the agreement and say why. Nothing is sent. The server refuses
    // first-run-done without the agreement as well (409, handled in postDone).
    if (!agreed) {
      goTo(CONSENT_STEP);
      showLine(consentError, MESSAGE_AGREE_FIRST);
      return Promise.resolve();
    }
    setFinishing(true);
    hideLine(profileError);
    var body = save ? profileBody() : {};
    if (!body.display_name && !body.home_area) {
      toAlerts();
      return Promise.resolve();
    }
    return request(PROFILE_URL, {
      method: 'PATCH',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    }, SAVE_TIMEOUT_MS).promise.then(function (answer) {
      if (!answer.ok) {
        setFinishing(false);
        showLine(profileError, saveMessage(answer));
        return;
      }
      // Saved: a retry of the last call must not send these again.
      if (body.display_name) {
        storedName = body.display_name;
      }
      if (body.home_area) {
        homePick = null;
        hasHomeArea = true;
      }
      toAlerts();
    }, function () {
      setFinishing(false);
      showLine(profileError, MESSAGE_SAVE_FAILED);
    });
  }

  // ---- Step 5: Alerts ---------------------------------------------------------

  // setAlertsBusy disables the three buttons of step 5 and marks the step busy
  // while a browser prompt or a save runs. No spinner is drawn: the browser's
  // own prompt is the progress.
  function setAlertsBusy(on) {
    alertsBusy = on;
    alertsAllow.disabled = on;
    alertsLater.disabled = on;
    alertsContinue.disabled = on;
    if (on) {
      alertsStep.setAttribute('aria-busy', 'true');
    } else {
      alertsStep.removeAttribute('aria-busy');
    }
  }

  // deviceVariant names the part of step 5 a device state shows. A device that
  // is on, ready or still unknown shows the ask; a state this script does not
  // know shows the plainest answer, that nothing can be shown.
  function deviceVariant(state) {
    if (state === 'on' || state === 'ready' || state === 'maybe') {
      return 'ready';
    }
    if (state === 'needs-install' || state === 'blocked' || state === 'unsupported') {
      return state;
    }
    return 'unsupported';
  }

  // drawAlerts shows exactly one part of step 5 for a device state, with the
  // title and the buttons that go with it. Only the ready part has Allow alerts
  // and Not now; every other part has the one Continue.
  function drawAlerts(state) {
    var variant = deviceVariant(state);
    alertsStep.setAttribute('data-device-state', variant);
    alertsReady.hidden = variant !== 'ready';
    alertsInstall.hidden = variant !== 'needs-install';
    alertsBlocked.hidden = variant !== 'blocked';
    alertsUnsupported.hidden = variant !== 'unsupported';
    setText(alertsTitle, variant === 'needs-install' ? TITLE_LATER : alertsTitleReady);
    alertsNoHome.hidden = hasHomeArea;
    alertsAllow.hidden = variant !== 'ready';
    alertsLater.hidden = variant !== 'ready';
    alertsContinue.hidden = variant === 'ready';
  }

  // enterAlerts runs on every arrival at step 5. It reads the base state at once
  // (no request, no prompt) and draws it, then asks for the device state with the
  // server's word and redraws only if that answer is still wanted.
  function enterAlerts() {
    alertsSeq += 1;
    var mine = alertsSeq;
    alertsRefused = false;
    setText(alertsStatus, '');
    hideLine(alertsError);
    var notify = window.PinalertNotify;
    if (!notify || typeof notify.baseState !== 'function' || pushKey === '') {
      drawAlerts('unsupported');
      return;
    }
    var base = notify.baseState();
    drawAlerts(base === 'maybe' ? 'ready' : base);
    if (base !== 'maybe' || typeof notify.deviceState !== 'function') {
      return;
    }
    notify.deviceState(pushKey).then(function (state) {
      if (mine !== alertsSeq || current !== ALERTS_STEP || alertsBusy || alertsRefused) {
        return;
      }
      drawAlerts(state);
    }, function () {
      // The base state stays on screen.
    });
  }

  // needAgreement sends a person who has not agreed back to the agreement, as
  // finish does. It returns true when it did, and then nothing else runs.
  function needAgreement() {
    if (agreed) {
      return false;
    }
    goTo(CONSENT_STEP);
    showLine(consentError, MESSAGE_AGREE_FIRST);
    return true;
  }

  // alertsDone records the flow as done and leaves. It is the only caller of
  // postDone.
  function alertsDone() {
    if (alertsBusy || needAgreement()) {
      return Promise.resolve();
    }
    setAlertsBusy(true);
    hideLine(alertsError);
    return postDone();
  }

  // onAllowAlerts is the only place the browser's notification prompt can run.
  // enable is called inside the click, before anything is awaited, so the
  // browser counts the tap as the person's own.
  function onAllowAlerts() {
    if (alertsBusy || needAgreement()) {
      return;
    }
    if (!window.PinalertNotify || typeof window.PinalertNotify.enable !== 'function') {
      showLine(alertsError, MESSAGE_ALERTS_FAILED);
      return;
    }
    hideLine(alertsError);
    setText(alertsStatus, '');
    var pending = window.PinalertNotify.enable(pushKey);
    setAlertsBusy(true);
    Promise.resolve(pending).then(function (result) {
      setAlertsBusy(false);
      onAlertsResult(result || {});
    }, function () {
      setAlertsBusy(false);
      showLine(alertsError, MESSAGE_ALERTS_FAILED);
    });
  }

  // onAlertsResult shows what enable answered. A finished or refused ask ends
  // the flow (a refusal after the person reads it); a failure keeps both buttons.
  function onAlertsResult(result) {
    if (result.ok === true) {
      alertsDone();
      return;
    }
    if (result.reason === 'refused') {
      alertsRefused = true;
      alertsAllow.hidden = true;
      alertsLater.hidden = true;
      alertsContinue.hidden = false;
      // The status line is an always-present live region (never hidden), so the
      // screen reader announces the text written into it (review T-04).
      setText(alertsStatus, MESSAGE_ALERTS_OFF);
      alertsContinue.focus();
      return;
    }
    if (result.reason === 'unsupported') {
      // The device turned out not to be able to show alerts: draw that part.
      alertsRefused = true;
      drawAlerts(result.state);
      alertsContinue.focus();
      return;
    }
    if (result.reason === 'offline') {
      showLine(alertsError, MESSAGE_ALERTS_OFFLINE);
      return;
    }
    var sentence = typeof result.message === 'string' && result.message !== '' && result.message.length <= 200 ? result.message : '';
    showLine(alertsError, sentence !== '' ? sentence : MESSAGE_ALERTS_FAILED);
  }

  // onNotNow (step 5) ends the flow and asks the browser nothing, so the
  // browser's one permission prompt is kept for Settings.
  function onNotNow() {
    alertsDone();
  }

  function onAlertsContinue() {
    alertsDone();
  }

  // ---- Keyboard: Tab stays inside, Escape does nothing ----------------------

  function onKeydown(e) {
    if (e.key !== 'Tab') {
      return;
    }
    var items = focusables();
    if (items.length === 0) {
      return;
    }
    var first = items[0];
    var last = items[items.length - 1];
    var active = document.activeElement;
    var title = el('welcome-step-' + current + '-title');
    // Shift+Tab from the title goes back to whatever comes before it (the step
    // list, the Back control). It wraps only when nothing does.
    var backButton = el('welcome-back-' + current);
    var nothingBefore = !sideShown() && !(backButton && isShown(backButton));
    if (e.shiftKey && (active === first || (active === title && nothingBefore) || !root.contains(active))) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && (active === last || !root.contains(active))) {
      e.preventDefault();
      first.focus();
    }
  }

  // ---- Wiring ---------------------------------------------------------------

  el('welcome-continue-1').addEventListener('click', function () { goTo(2); });
  // Back on steps 2 to 5 shows the previous step; the step list buttons show
  // the step they name. Looking at a step records nothing.
  function wireBack(n) {
    var back = el('welcome-back-' + n);
    if (back) {
      back.addEventListener('click', function () { goTo(n - 1); });
    }
  }
  for (var b = 2; b <= stepCount; b++) {
    wireBack(b);
  }
  function wireListButton(button) {
    button.addEventListener('click', function () { goTo(button.getAttribute('data-step')); });
  }
  var listButtons = root.querySelectorAll('.welcome__steps-button');
  for (var lb = 0; lb < listButtons.length; lb++) {
    wireListButton(listButtons[lb]);
  }
  // The browser's own back and forward (and the phone's swipe back) move
  // between the steps the person has seen. Nothing is added to the history.
  if (canPush && typeof window.addEventListener === 'function') {
    window.addEventListener('popstate', function (e) {
      var n = e && e.state && e.state.pinalertStep;
      if (n) {
        goTo(n, 'none');
      }
    });
  }
  el('welcome-disagree').addEventListener('click', disagree);
  el('welcome-reread').addEventListener('click', readAgain);
  agreeButton.addEventListener('click', function () { agree(); });
  allowButton.addEventListener('click', allowLocation);
  notNowButton.addEventListener('click', notNow);
  continueButton.addEventListener('click', function () { finish(true); });
  skipButton.addEventListener('click', function () { finish(false); });
  alertsAllow.addEventListener('click', onAllowAlerts);
  alertsLater.addEventListener('click', onNotNow);
  alertsContinue.addEventListener('click', onAlertsContinue);
  root.addEventListener('keydown', onKeydown);

  var startStep = parseInt(root.getAttribute('data-start-step'), 10);
  if (!isFinite(startStep)) {
    startStep = 1;
  }
  agreed = startStep > CONSENT_STEP;
  watchForEnd();
  goTo(firstStep(startStep), 'replace');

  window.PinalertWelcome = {
    goTo: goTo,
    agree: agree,
    disagree: disagree,
    allowLocation: allowLocation,
    finish: finish
  };
}());
