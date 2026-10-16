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
//     Disagree records nothing and shows one message.
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
  // place search of Settings. Each has its own budget on the server.
  var STEP3_SEARCH_URL = '/api/geocode?q=';
  var STEP4_SEARCH_URL = '/api/geocode/place?q=';
  var FOCUS_SEARCH_KEY = 'pinalert.focusSearch';
  var LAND_ON = '/';
  var SAVE_TIMEOUT_MS = 10000;
  var SEARCH_TIMEOUT_MS = 10000;
  var LOCATION_TIMEOUT_MS = 8000;
  var MIN_QUERY_RUNES = 3;
  var MAX_RESULTS = 5;

  var DISAGREE_MESSAGE = 'You need to agree to use Pinalert.';
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

  var disagreeMessage = el('welcome-disagree-message');
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

  var current = 0;
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

  function focusables() {
    var all = root.querySelectorAll(FOCUSABLE_SELECTOR);
    var out = [];
    for (var i = 0; i < all.length; i++) {
      if (!all[i].disabled && isShown(all[i])) {
        out.push(all[i]);
      }
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
      if (n === step) {
        item.setAttribute('aria-current', 'step');
      } else {
        item.removeAttribute('aria-current');
      }
    }
  }

  // enterStep4 draws the home area field for the first time. The field is
  // prefilled only from a place searched in step 3; otherwise it keeps what the
  // page was drawn with (empty, or the stored home area of a returning account)
  // and no prefilled line shows.
  function enterStep4() {
    if (step3Pick !== null) {
      homePick = step3Pick;
      homeInput.value = step3Pick.name;
      homePrefilled.hidden = false;
    } else {
      homePick = null;
      homePrefilled.hidden = true;
    }
  }

  // goTo shows one step. A step that does not exist is refused. Changing step
  // clears the Disagree message, and focus moves to the new step's title.
  function goTo(step) {
    var n = parseInt(step, 10);
    if (!isFinite(n) || n < 1 || n > stepCount) {
      return;
    }
    var previous = current;
    current = n;
    for (var i = 1; i <= stepCount; i++) {
      el('welcome-step-' + i).hidden = i !== n;
    }
    root.setAttribute('data-step', String(n));
    root.setAttribute('aria-labelledby', 'welcome-step-' + n + '-title');
    renderStepList(n);
    hideLine(disagreeMessage);
    if (n === 4 && previous !== 4) {
      enterStep4();
    }
    el('welcome-step-' + n + '-title').focus();
  }

  // ---- Step 2: Agree and Disagree -------------------------------------------

  function disagree() {
    showLine(disagreeMessage, DISAGREE_MESSAGE);
  }

  function agree() {
    if (agreeing) {
      return Promise.resolve();
    }
    agreeing = true;
    agreeButton.disabled = true;
    hideLine(consentError);
    return request(CONSENT_URL, { method: 'POST', credentials: 'same-origin' }, SAVE_TIMEOUT_MS).promise.then(function (answer) {
      agreeing = false;
      agreeButton.disabled = false;
      if (answer.status === 204) {
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
  // cfg: {input, form, status, results, url, onPick(place), onClear()}. A pick
  // is only ever a result of the search, so a place is never anything the
  // person typed or the device reported. A failure shows one plain line and
  // never blocks the step.
  function makeSearch(cfg) {
    var picked = null;
    var seq = 0;
    var inFlight = null;
    var inFlightText = '';

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
    }

    function pick(option, place) {
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
        option.addEventListener('click', pick.bind(null, option, { name: name, lat: res.lat, lon: res.lon }));
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
    function search() {
      var q = String(cfg.input.value).trim();
      if (runeCount(q) < MIN_QUERY_RUNES) {
        cancel();
        cfg.results.hidden = true;
        showLine(cfg.status, MESSAGE_TOO_SHORT);
        return Promise.resolve();
      }
      if (inFlight !== null && q === inFlightText) {
        return inFlight.promise.then(function () {}, function () {});
      }
      cancel();
      clearPick();
      cfg.results.hidden = true;
      showLine(cfg.status, MESSAGE_SEARCHING);
      seq += 1;
      var mine = seq;
      inFlightText = q;
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
          var sent = serverSentence(answer.body);
          showLine(cfg.status, sent !== '' ? sent : (answer.status === 429 ? MESSAGE_RATE_LIMITED : MESSAGE_UNAVAILABLE));
          return;
        }
        render(answer.body && Array.isArray(answer.body.results) ? answer.body.results : []);
      }, function () {
        if (mine !== seq) {
          return;
        }
        finish();
        showLine(cfg.status, MESSAGE_UNAVAILABLE);
      });
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
      onPick: function (place) { step3Pick = place; },
      onClear: function () { step3Pick = null; }
    });
    homeSearch = makeSearch({
      input: homeInput,
      form: el('welcome-home-form'),
      status: el('welcome-home-status'),
      results: el('welcome-home-results'),
      url: STEP4_SEARCH_URL,
      onPick: function (place) {
        homePick = place;
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
    if (e.shiftKey && (active === first || active === title || !root.contains(active))) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && (active === last || !root.contains(active))) {
      e.preventDefault();
      first.focus();
    }
  }

  // ---- Wiring ---------------------------------------------------------------

  el('welcome-continue-1').addEventListener('click', function () { goTo(2); });
  el('welcome-disagree').addEventListener('click', disagree);
  agreeButton.addEventListener('click', function () { agree(); });
  allowButton.addEventListener('click', allowLocation);
  searchOpenButton.addEventListener('click', openSearch);
  el('welcome-continue-3').addEventListener('click', function () { goTo(4); });
  continueButton.addEventListener('click', function () { finish(true); });
  skipButton.addEventListener('click', function () { finish(false); });
  root.addEventListener('keydown', onKeydown);

  var startStep = parseInt(root.getAttribute('data-start-step'), 10);
  goTo(isFinite(startStep) ? startStep : 1);

  window.PinalertWelcome = {
    goTo: goTo,
    agree: agree,
    disagree: disagree,
    allowLocation: allowLocation,
    finish: finish
  };
}());
