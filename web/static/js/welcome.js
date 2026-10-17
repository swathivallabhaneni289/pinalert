// welcome.js: the first-time flow at /welcome (Phase 3.4, plan 10, UI-SPEC
// Surface 7, D-27, D-28, D-24b).
//
// Self contained, like settings.js: it needs no other script of the app, only
// the ids welcome.html.tmpl renders, and it leaves quietly on a page that has no
// flow. It shows one of four steps at a time, moves focus to the step title,
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
//     person filled, and then POST /api/me/first-run-done. Skip sends only the
//     second call. Nothing is saved before that.
//
// What it never does:
//   - It asks the browser where the device is in one place only, Allow location
//     on step 3. The fix is saved only through PinalertPosition, in this browser,
//     and is sent nowhere. The server stores no reader location.
//   - It never fills the home area from the device fix. The home area is a place
//     the person picked from a search: on step 3 (kept in memory for step 4 only)
//     or on step 4. There is no reverse lookup. Text that was typed but never
//     picked is not a home area and is not sent.
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
  // Step 3 uses the same forward search as the report form; step 4 uses the
  // place search of Settings, asking for areas only (kind=area: neighbourhoods,
  // localities, towns, cities, districts, never a street or a building, since
  // the stored home area is a cell about 5 km wide). Each has its own budget on
  // the server.
  var STEP3_SEARCH_URL = '/api/geocode?q=';
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
  var MESSAGE_NO_MATCH = 'No matches found. Try a nearby town or landmark.';
  var MESSAGE_UNAVAILABLE = 'Search unavailable. Try again in a moment.';
  var MESSAGE_RATE_LIMITED = 'Too many searches. Wait a few seconds and try again.';

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
  var consentError = el('welcome-error-2');
  var agreeButton = el('welcome-agree');
  var allowButton = el('welcome-allow-location');
  var searchOpenButton = el('welcome-search-open');
  var searchBlock = el('welcome-search-block');
  var nameInput = el('welcome-name-input');
  var homeInput = el('welcome-home-input');
  var homePrefilled = el('welcome-home-prefilled');
  var profileError = el('welcome-error-4');
  var continueButton = el('welcome-continue-4');
  var skipButton = el('welcome-skip');

  // The step a person must agree on, and the highest step an address may name
  // for an account that has not agreed yet. A person may look at any step in
  // any order, but finishing needs the agreement (the server holds the same rule).
  var CONSENT_STEP = 2;

  var current = 0;
  // Whether the current terms are agreed: true when the server started the flow
  // past the agreement step, or after Agree was saved on this page.
  var agreed = false;
  // Whether the person has typed or picked in the home area. Until they have,
  // the field follows the place picked on step 3.
  var homeTouched = false;
  // The text the home area was last prefilled with from step 3, and the text it
  // was drawn with, so a prefill nobody touched can be taken away again.
  var lastPrefill = null;
  var drawnHome = homeInput ? homeInput.value : '';
  var agreeing = false;
  var locating = false;
  var finishing = false;

  // The values the page was drawn with, so only what the person changed is sent.
  var storedName = nameInput ? nameInput.value.trim() : '';

  // The place picked on step 3, kept in memory for step 4 only. It is never
  // stored anywhere and never sent from step 3.
  var step3Pick = null;
  // The place picked for the home area, from step 3's prefill or step 4's own
  // search. Only a picked place is ever sent as a home area.
  var homePick = null;

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
  // the focus trap skips the controls of a hidden step and a hidden search.
  function isShown(node) {
    for (var n = node; n && n !== root; n = n.parentNode) {
      if (n.hidden) {
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

  // enterStep4 draws the home area field for the first time. The field is
  // prefilled only from a place searched in step 3, and only with the area that
  // place lies in (step3Pick.area: the place itself when it is an area, else its
  // locality, district or city), never with a street or a building name. When
  // step 3 gave no area, or nothing was searched, the field keeps what the page
  // was drawn with (empty, or the stored home area of a returning account) and
  // no prefilled line shows.
  //
  // Step 4 can be shown again after a trip to another step. Then a home area
  // the person typed or picked is left alone, and one that was only prefilled
  // from step 3 follows what step 3 holds now (and goes away with it).
  function enterStep4() {
    if (homeTouched) {
      return;
    }
    if (step3Pick !== null && step3Pick.area !== '') {
      // The point stays the picked place's own: the server keeps only a cell
      // about 5 km wide, so the point of a street and the point of the area it
      // lies in give the same cell or the next one, and only the name matters.
      homePick = { name: step3Pick.area, lat: step3Pick.lat, lon: step3Pick.lon, area: step3Pick.area };
      homeInput.value = step3Pick.area;
      lastPrefill = step3Pick.area;
      homePrefilled.hidden = false;
    } else {
      homePick = null;
      homePrefilled.hidden = true;
      if (lastPrefill !== null && homeInput.value === lastPrefill) {
        homeInput.value = drawnHome;
      }
      lastPrefill = null;
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
    if (n === 4 && previous !== 4) {
      enterStep4();
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

  function openSearch() {
    searchBlock.hidden = false;
    el('welcome-search-input').focus();
  }

  // ---- Place searches (step 3 and step 4) -----------------------------------

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
      cfg.onPick(place);
    }

    function render(results) {
      cfg.results.textContent = '';
      var shown = 0;
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
        var label = document.createElement('span');
        setText(label, name);
        option.appendChild(label);
        if (typeof res.display_name === 'string' && res.display_name !== '' && res.display_name !== name) {
          var sub = document.createElement('span');
          sub.className = 'app-window__option-sub';
          setText(sub, res.display_name);
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
        showLine(cfg.status, MESSAGE_NO_MATCH);
        return;
      }
      hideLine(cfg.status);
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
        render(answer.body && Array.isArray(answer.body.results) ? answer.body.results : []);
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

  var step3Search = null;
  var homeSearch = null;
  if (el('welcome-search-form') && el('welcome-home-form')) {
    step3Search = makeSearch({
      input: el('welcome-search-input'),
      form: el('welcome-search-form'),
      status: el('welcome-search-status'),
      results: el('welcome-search-results'),
      url: STEP3_SEARCH_URL,
      typing: true,
      onPick: function (place) { step3Pick = place; },
      onClear: function () { step3Pick = null; }
    });
    homeSearch = makeSearch({
      input: homeInput,
      form: el('welcome-home-form'),
      status: el('welcome-home-status'),
      results: el('welcome-home-results'),
      url: STEP4_SEARCH_URL,
      typing: true,
      onPick: function (place) {
        homePick = place;
        homeTouched = true;
        homePrefilled.hidden = true;
      },
      onClear: function () {
        homePick = null;
        homePrefilled.hidden = true;
      }
    });
  }

  // Typing in the home area after a step 3 prefill forgets that pick too (the
  // step 4 search only knows picks of its own).
  homeInput.addEventListener('input', function () {
    homeTouched = true;
    if (homePick !== null && homeInput.value !== homePick.name) {
      homePick = null;
      homePrefilled.hidden = true;
    }
  });

  // ---- Step 4: finishing ----------------------------------------------------

  function setFinishing(on) {
    finishing = on;
    continueButton.disabled = on;
    skipButton.disabled = on;
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
      showLine(profileError, answer.status === 401 ? MESSAGE_SESSION_ENDED : MESSAGE_SAVE_FAILED);
    }, function () {
      setFinishing(false);
      showLine(profileError, MESSAGE_SAVE_FAILED);
    });
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

  // finish ends the flow. With save true it first sends the profile fields the
  // person filled (no request when there are none); with save false (Skip) it
  // saves nothing. Then it records the flow as done and lands on Nearby.
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
      return postDone();
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
      }
      return postDone();
    }, function () {
      setFinishing(false);
      showLine(profileError, MESSAGE_SAVE_FAILED);
    });
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
  // Back on steps 2 to 4 shows the previous step; the step list buttons show
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
  searchOpenButton.addEventListener('click', openSearch);
  el('welcome-continue-3').addEventListener('click', function () { goTo(4); });
  continueButton.addEventListener('click', function () { finish(true); });
  skipButton.addEventListener('click', function () { finish(false); });
  root.addEventListener('keydown', onKeydown);

  var startStep = parseInt(root.getAttribute('data-start-step'), 10);
  if (!isFinite(startStep)) {
    startStep = 1;
  }
  agreed = startStep > CONSENT_STEP;
  goTo(firstStep(startStep), 'replace');

  window.PinalertWelcome = {
    goTo: goTo,
    agree: agree,
    disagree: disagree,
    allowLocation: allowLocation,
    finish: finish
  };
}());
