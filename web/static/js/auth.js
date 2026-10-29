// auth.js - the sign in card's panel state machine (Phase 7.1, plan 12).
//
// One page, one card, four panels that swap in place with the hidden attribute:
// Create account, Sign in, the code screen and Forgot password. Nothing navigates
// until a sign in succeeds, and then only to a path from NEXT_PATHS. The email lives
// in a variable and in the page text of the code screen; it is never written into
// the page address, a link, the address bar or anything the browser keeps.
//
// This page ships no app shell and does not load app.js, so it owns a small
// self-contained setText helper rather than reaching for window.Pinalert.
//
// SECURITY (T-01-03, T-07.1-20): every string that reaches the DOM here, server
// sentences included, is set as text content and never assembled into markup.
// The numbers (resend wait, code lifetime, seconds left, code length) are read from
// the page's data attributes and the code field; none is typed in this file.
(function () {
  'use strict';

  var RATE_LIMIT_MESSAGE = 'Too many requests. Try again in a minute.';
  var WRONG_CREDENTIALS_MESSAGE = 'Email or password is wrong.';
  var OLD_ACCOUNT_LINE = 'Had an account before passwords? Choose Create account with the same email. Your reports stay with you.';
  var OFFLINE_MESSAGE = "You're offline. Try again when you have a signal.";
  var SERVER_MESSAGE = 'Something went wrong. Try again.';
  var CODE_WRONG_MESSAGE = 'That code is wrong or has expired.';
  var CODE_EXPIRED_MESSAGE = 'The code has expired. Get a new one.';
  var CODE_TRIES_MESSAGE = 'Too many wrong tries. Get a new code.';
  var CODE_SENT_MESSAGE = 'A new code is on its way.';
  var PASSWORD_SET_MESSAGE = 'Password set. Sign in to continue.';
  var SENDING_LABEL = 'Sending\u2026';
  var SIGNING_IN_LABEL = 'Signing in\u2026';
  var CHECKING_LABEL = 'Checking\u2026';
  var MAX_CLIENT_TRIES = 5;
  // How long one request may take, the body read included (review fix WR-B01).
  var REQUEST_TIMEOUT_MS = 10000;
  var NEXT_PATHS = ['/', '/low'];

  function byId(id) {
    return document.getElementById(id);
  }

  var card = byId('login-form-panel');
  var createForm = byId('auth-form-create');
  if (!card || !createForm) {
    return;
  }

  var title = byId('login-title');
  var doneLine = byId('login-done');
  var choice = byId('login-choice');
  var tabCreate = byId('login-tab-create');
  var tabSignin = byId('login-tab-signin');

  var panels = {
    create: byId('auth-panel-create'),
    signin: byId('auth-panel-signin'),
    code: byId('auth-panel-code'),
    forgot: byId('auth-panel-forgot')
  };
  var forms = {
    create: createForm,
    signin: byId('auth-form-signin'),
    code: byId('auth-form-code'),
    forgot: byId('auth-form-forgot')
  };

  var createEmail = byId('create-email');
  var createPassword = byId('create-password');
  var createError = byId('create-error');
  var createSubmit = byId('create-submit');

  var signinEmail = byId('signin-email');
  var signinPassword = byId('signin-password');
  var signinError = byId('signin-error');
  var signinStatus = byId('signin-status');
  var signinSubmit = byId('signin-submit');

  var codeInput = byId('code-input');
  var codeNewWrap = byId('code-new-wrap');
  var codeNewPassword = byId('code-new-password');
  var codeEmail = byId('code-email');
  var codeStatus = byId('code-status');
  var codeError = byId('code-error');
  var codeSubmit = byId('code-submit');
  var codeResend = byId('code-resend');
  var codeReadyLive = byId('code-ready-live');

  var forgotEmail = byId('forgot-email');
  var forgotError = byId('forgot-error');
  var forgotSubmit = byId('forgot-submit');

  // What each panel owns: the error line its refusals go to, and the inputs a
  // refusal may mark invalid.
  var parts = {
    create: { error: createError, inputs: [createEmail, createPassword] },
    signin: { error: signinError, inputs: [signinEmail, signinPassword], status: signinStatus },
    code: { error: codeError, inputs: [codeInput, codeNewPassword] },
    forgot: { error: forgotError, inputs: [forgotEmail] }
  };

  // The password fields and their Show buttons. Each button names its field in
  // data-show-for.
  var showButtons = [byId('create-show'), byId('signin-show'), byId('code-new-show')];

  var state = {
    panel: 'create',
    mode: 'create',
    from: 'create',
    email: '',
    tries: 0,
    resendLeft: 0,
    resendAt: 0,
    expiresAt: 0,
    resendBusy: false,
    resendTimer: null,
    expiryTimer: null,
    // Bumped by showPanel, so every screen has its own number (IN-B03).
    generation: 0
  };

  function setText(el, value) {
    if (!el) {
      return;
    }
    el.textContent = value === null || value === undefined ? '' : String(value);
  }

  function numberAttr(name) {
    var n = parseInt(card.getAttribute(name), 10);
    return n > 0 ? n : 0;
  }

  // The full Resend wait, started after every send. data-resend-seconds is the
  // wait left when the page was drawn (review fix WR-B02), which on an entry
  // panel is the whole wait. A page that also draws data-resend-cooldown-seconds
  // names the whole wait everywhere, the code screen after a reload included.
  var fullResendSeconds = numberAttr('data-resend-cooldown-seconds') || numberAttr('data-resend-seconds');

  // ---- Errors and the field marks ------------------------------------------------

  // The password fields already point at their rule line; the error is added to
  // that description and taken away again, never replacing it.
  var originalDescribedBy = {};
  parts.create.inputs.concat(parts.signin.inputs, parts.code.inputs, parts.forgot.inputs).forEach(function (input) {
    originalDescribedBy[input.id] = input.getAttribute('aria-describedby');
  });

  function markInvalid(input, line, on) {
    if (!input) {
      return;
    }
    var original = originalDescribedBy[input.id];
    if (on) {
      input.setAttribute('aria-invalid', 'true');
      input.setAttribute('aria-describedby', (original ? original + ' ' : '') + line.id);
    } else {
      input.removeAttribute('aria-invalid');
      if (original) {
        input.setAttribute('aria-describedby', original);
      } else {
        input.removeAttribute('aria-describedby');
      }
    }
  }

  // showError writes one sentence into a panel's own error line and, when the
  // refusal names a field of that panel, marks that field.
  function showError(line, field, message) {
    setText(line, message);
    line.hidden = false;
    if (field) {
      markInvalid(field, line, true);
    }
  }

  // The wrong sign in sentence and the old account line sit in one block, one
  // under the other, inside a wrapper the card's stylesheet lays out.
  function showSignInError(first, more) {
    setText(signinError, '');
    var wrap = document.createElement('span');
    wrap.className = 'inline-error__text';
    var top = document.createElement('span');
    setText(top, first);
    wrap.appendChild(top);
    if (more) {
      var under = document.createElement('span');
      setText(under, more);
      wrap.appendChild(under);
    }
    signinError.appendChild(wrap);
    signinError.hidden = false;
  }

  function clearErrors(name) {
    var part = parts[name];
    setText(part.error, '');
    part.error.hidden = true;
    part.inputs.forEach(function (input) {
      markInvalid(input, part.error, false);
    });
    if (part.status) {
      setText(part.status, '');
      part.status.hidden = true;
    }
  }

  function clearAllErrors() {
    Object.keys(parts).forEach(clearErrors);
  }

  // ---- Busy buttons ---------------------------------------------------------------

  var restingLabel = {};

  function isBusy(button) {
    return button.getAttribute('aria-busy') === 'true';
  }

  function setBusy(button, label) {
    restingLabel[button.id] = button.textContent;
    setText(button, label);
    button.setAttribute('aria-busy', 'true');
  }

  function clearBusy(button) {
    if (Object.prototype.hasOwnProperty.call(restingLabel, button.id)) {
      setText(button, restingLabel[button.id]);
      delete restingLabel[button.id];
    }
    button.removeAttribute('aria-busy');
  }

  function isOff(button) {
    return button.getAttribute('aria-disabled') === 'true';
  }

  // ---- Requests -------------------------------------------------------------------

  // postJSON always resolves: status 0 stands for a request that never got an answer.
  // The time limit covers the body read too, so a request WebKit leaves unsettled
  // (an iPhone tab suspended on the way to Mail) still ends and frees its button,
  // as request() in welcome.js and settings.js does (review fix WR-B01).
  function postJSON(path, body) {
    var controller = typeof AbortController === 'function' ? new AbortController() : null;
    var init = {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    };
    if (controller !== null) {
      init.signal = controller.signal;
    }
    var noAnswer = { ok: false, status: 0, body: {} };
    return new Promise(function (resolve) {
      var settled = false;
      var timer = setTimeout(function () {
        timer = null;
        if (controller !== null) {
          controller.abort();
        }
        finish(noAnswer);
      }, REQUEST_TIMEOUT_MS);
      function finish(result) {
        if (settled) {
          return;
        }
        settled = true;
        if (timer !== null) {
          clearTimeout(timer);
          timer = null;
        }
        resolve(result);
      }
      var call;
      try {
        call = fetch(path, init);
      } catch (err) {
        call = Promise.reject(err);
      }
      Promise.resolve(call).then(function (res) {
        return res.json().catch(function () {
          return {};
        }).then(function (data) {
          finish({ ok: res.ok, status: res.status, body: data || {} });
        });
      }).then(null, function () {
        finish(noAnswer);
      });
    });
  }

  function sentenceOf(result) {
    var detail = result.body && result.body.error;
    return detail && typeof detail.message === 'string' ? detail.message : '';
  }

  function fieldOf(result) {
    var detail = result.body && result.body.error;
    return detail && typeof detail.field === 'string' ? detail.field : '';
  }

  // messageFor picks the sentence for a refused or failed request: the server's own
  // when it sent one, and a fixed one when it did not.
  function messageFor(result) {
    if (result.status === 0) {
      return OFFLINE_MESSAGE;
    }
    var said = sentenceOf(result);
    if (result.status === 429) {
      return said || RATE_LIMIT_MESSAGE;
    }
    return said || SERVER_MESSAGE;
  }

  // fieldFor names the input a refusal belongs to, only for a 400 that names a field
  // of this panel. A limit or a mail problem also names "email" but blames nothing.
  function fieldFor(result, byField) {
    if (result.status !== 400) {
      return null;
    }
    return byField[fieldOf(result)] || null;
  }

  function goNext(next) {
    var target = NEXT_PATHS.indexOf(next) >= 0 ? next : NEXT_PATHS[0];
    window.location.assign(target);
  }

  // ---- Show and Hide --------------------------------------------------------------

  function toggleShow(input, button) {
    var showing = input.type === 'text';
    input.type = showing ? 'password' : 'text';
    setText(button, showing ? 'Show' : 'Hide');
    button.setAttribute('aria-label', showing ? 'Show password' : 'Hide password');
  }

  function resetShow() {
    showButtons.forEach(function (button) {
      var input = byId(button.getAttribute('data-show-for'));
      if (input) {
        input.type = 'password';
      }
      setText(button, 'Show');
      button.setAttribute('aria-label', 'Show password');
    });
  }

  // ---- Panels and the two choices -------------------------------------------------

  function setTabs(name) {
    var create = name === 'create';
    tabCreate.setAttribute('aria-selected', create ? 'true' : 'false');
    tabSignin.setAttribute('aria-selected', create ? 'false' : 'true');
    tabCreate.setAttribute('tabindex', create ? '0' : '-1');
    tabSignin.setAttribute('tabindex', create ? '-1' : '0');
  }

  function hideDoneLine() {
    if (doneLine) {
      doneLine.hidden = true;
    }
  }

  // showPanel swaps the card to one panel. The entry panels keep the title Pinalert
  // and the two choices; the code and Forgot panels take their own title and focus.
  function showPanel(name) {
    resetShow();
    clearAllErrors();
    Object.keys(panels).forEach(function (key) {
      panels[key].hidden = key !== name;
    });
    var entry = name === 'create' || name === 'signin';
    choice.hidden = !entry;
    if (entry) {
      setTabs(name);
      setText(title, 'Pinalert');
    } else {
      hideDoneLine();
      setText(title, name === 'code' && state.mode !== 'reset' ? 'Check your email' : 'Reset your password');
      title.focus();
    }
    state.panel = name;
    // Every change of panel starts a new screen; an answer to a request sent from
    // an older one only frees its button (review fix IN-B03).
    state.generation += 1;
  }

  // selectTab is what a press, an arrow, Home, End, Enter or Space on a choice does.
  // Focus stays on the choice just pressed (Safari does not focus a tapped button).
  function selectTab(name, focus) {
    if (state.panel !== name) {
      showPanel(name);
      hideDoneLine();
    }
    if (focus) {
      (name === 'create' ? tabCreate : tabSignin).focus();
    }
  }

  function onTabKey(event) {
    var key = event.key;
    var target;
    if (key === 'ArrowLeft' || key === 'ArrowRight') {
      target = state.panel === 'create' ? 'signin' : 'create';
    } else if (key === 'Home') {
      target = 'create';
    } else if (key === 'End') {
      target = 'signin';
    } else if (key === 'Enter' || key === ' ') {
      target = event.target === tabSignin ? 'signin' : 'create';
    } else {
      return;
    }
    event.preventDefault();
    selectTab(target, true);
  }

  // ---- The code screen: timers ----------------------------------------------------

  function renderResend() {
    if (state.resendBusy) {
      setText(codeResend, SENDING_LABEL);
      codeResend.setAttribute('aria-busy', 'true');
      codeResend.setAttribute('aria-disabled', 'true');
      return;
    }
    codeResend.removeAttribute('aria-busy');
    if (state.resendLeft > 0) {
      setText(codeResend, 'Resend in ' + state.resendLeft + 's');
      codeResend.setAttribute('aria-disabled', 'true');
    } else {
      setText(codeResend, 'Resend code');
      codeResend.removeAttribute('aria-disabled');
    }
  }

  function stopTimers() {
    if (state.resendTimer !== null) {
      clearInterval(state.resendTimer);
      state.resendTimer = null;
    }
    if (state.expiryTimer !== null) {
      clearInterval(state.expiryTimer);
      state.expiryTimer = null;
    }
  }

  // Both clocks below are wall clock deadlines (review fix WR-B02). A tick only
  // redraws from Date.now, so a tab that iOS suspended or a desktop browser
  // throttled shows the right wait the moment it runs again, and coming back to
  // the page (visibilitychange, pageshow) catches up at once.
  function secondsUntil(at) {
    var left = Math.ceil((at - Date.now()) / 1000);
    return left > 0 ? left : 0;
  }

  function tickResend() {
    state.resendLeft = secondsUntil(state.resendAt);
    renderResend();
    if (state.resendLeft <= 0 && state.resendTimer !== null) {
      clearInterval(state.resendTimer);
      state.resendTimer = null;
      setText(codeReadyLive, 'Resend code');
    }
  }

  // startResendWait starts the Resend wait: the seconds left at this moment. It is
  // its own clock, separate from the code's lifetime below. The countdown is not a
  // live region; it announces once, when the wait is over.
  function startResendWait(seconds) {
    if (state.resendTimer !== null) {
      clearInterval(state.resendTimer);
      state.resendTimer = null;
    }
    state.resendBusy = false;
    state.resendAt = Date.now() + seconds * 1000;
    state.resendLeft = seconds;
    setText(codeReadyLive, '');
    renderResend();
    if (seconds <= 0) {
      return;
    }
    state.resendTimer = setInterval(tickResend, 1000);
  }

  function expireNow() {
    if (state.expiryTimer !== null) {
      clearInterval(state.expiryTimer);
      state.expiryTimer = null;
    }
    setText(codeStatus, CODE_EXPIRED_MESSAGE);
    codeStatus.hidden = false;
  }

  function tickExpiry() {
    if (state.expiryTimer !== null && Date.now() >= state.expiresAt) {
      expireNow();
    }
  }

  // startExpiry shows the expiry notice once the code's lifetime (the seconds left
  // at this moment) has run out. The button stays pressable; a press still asks
  // the server.
  function startExpiry(seconds) {
    if (state.expiryTimer !== null) {
      clearInterval(state.expiryTimer);
      state.expiryTimer = null;
    }
    state.expiresAt = Date.now() + seconds * 1000;
    state.expiryTimer = setInterval(tickExpiry, 1000);
  }

  // catchUp redraws both clocks from the wall clock when the person comes back.
  function catchUp() {
    if (state.resendTimer !== null) {
      tickResend();
    }
    tickExpiry();
  }

  // ---- The code screen: field, modes, tries ---------------------------------------

  // cleanCode keeps digits only and stops at the length the field allows.
  function cleanCode(value) {
    var digits = String(value).replace(/\D/g, '');
    var max = parseInt(codeInput.getAttribute('maxlength'), 10);
    return max > 0 ? digits.slice(0, max) : digits;
  }

  // A paste is cleaned before the browser cuts it to the field's length, otherwise
  // "482 913" would arrive as "48291" (sketch 010 finding 1).
  function onCodePaste(event) {
    var data = event.clipboardData;
    var pasted = data && typeof data.getData === 'function' ? data.getData('text') : '';
    if (!pasted) {
      return;
    }
    event.preventDefault();
    codeInput.value = cleanCode(pasted);
  }

  function lockCodeButton(on) {
    if (on) {
      codeSubmit.setAttribute('aria-disabled', 'true');
    } else {
      codeSubmit.removeAttribute('aria-disabled');
    }
  }

  function setCodeMode(mode) {
    var reset = mode === 'reset';
    state.mode = mode;
    codeNewWrap.hidden = !reset;
    if (reset) {
      codeNewPassword.setAttribute('required', '');
    } else {
      codeNewPassword.removeAttribute('required');
    }
    setText(codeSubmit, reset ? 'Set new password' : 'Create account');
  }

  // openCode shows the code screen after a request that sent a code. Every address
  // gets this same screen.
  function openCode(mode, email, from) {
    state.from = from;
    state.email = email;
    state.tries = 0;
    setText(codeEmail, email);
    codeInput.value = '';
    codeNewPassword.value = '';
    setText(codeStatus, '');
    codeStatus.hidden = true;
    setCodeMode(mode);
    lockCodeButton(false);
    showPanel('code');
    startResendWait(fullResendSeconds);
    startExpiry(numberAttr('data-code-ttl-seconds'));
  }

  // refuseCode counts one refused code. The fifth switches the button off until a
  // new code is asked for.
  function refuseCode() {
    state.tries += 1;
    if (state.tries >= MAX_CLIENT_TRIES) {
      showError(codeError, codeInput, CODE_TRIES_MESSAGE);
      lockCodeButton(true);
      codeInput.value = '';
      if (isOff(codeResend)) {
        title.focus();
      } else {
        codeResend.focus();
      }
      return;
    }
    showError(codeError, codeInput, CODE_WRONG_MESSAGE);
    codeInput.focus();
    codeInput.select();
  }

  // ---- Submits --------------------------------------------------------------------

  function submitCreate(event) {
    event.preventDefault();
    if (isBusy(createSubmit)) {
      return;
    }
    clearErrors('create');
    resetShow();
    var email = createEmail.value.trim();
    var mine = state.generation;
    setBusy(createSubmit, SENDING_LABEL);
    postJSON('/api/auth/sign-up', { email: email, password: createPassword.value }).then(function (result) {
      clearBusy(createSubmit);
      if (mine !== state.generation) {
        return;
      }
      if (result.ok) {
        openCode('create', email, 'create');
        return;
      }
      var field = fieldFor(result, { email: createEmail, password: createPassword });
      showError(createError, field, messageFor(result));
      if (field) {
        field.focus();
      }
    });
  }

  function submitSignIn(event) {
    event.preventDefault();
    if (isBusy(signinSubmit)) {
      return;
    }
    clearErrors('signin');
    resetShow();
    var mine = state.generation;
    setBusy(signinSubmit, SIGNING_IN_LABEL);
    postJSON('/api/auth/sign-in', { email: signinEmail.value.trim(), password: signinPassword.value }).then(function (result) {
      clearBusy(signinSubmit);
      // A sign in that succeeded goes on whatever the card shows now: the
      // browser is signed in.
      if (result.ok) {
        goNext(result.body.next);
        return;
      }
      if (mine !== state.generation) {
        return;
      }
      if (result.status === 400 && fieldOf(result) === 'credentials') {
        showSignInError(WRONG_CREDENTIALS_MESSAGE, OLD_ACCOUNT_LINE);
        markInvalid(signinEmail, signinError, true);
        markInvalid(signinPassword, signinError, true);
        signinPassword.focus();
        return;
      }
      var field = fieldFor(result, { email: signinEmail, password: signinPassword });
      showSignInError(messageFor(result), '');
      markInvalid(field, signinError, true);
    });
  }

  function submitForgot(event) {
    event.preventDefault();
    if (isBusy(forgotSubmit)) {
      return;
    }
    clearErrors('forgot');
    var email = forgotEmail.value.trim();
    var mine = state.generation;
    setBusy(forgotSubmit, SENDING_LABEL);
    postJSON('/api/auth/reset', { email: email }).then(function (result) {
      clearBusy(forgotSubmit);
      if (mine !== state.generation) {
        return;
      }
      if (result.ok) {
        openCode('reset', email, 'forgot');
        return;
      }
      var field = fieldFor(result, { email: forgotEmail });
      showError(forgotError, field, messageFor(result));
      if (field) {
        field.focus();
      }
    });
  }

  function submitCode(event) {
    event.preventDefault();
    // Enter reaches this handler even when the button is switched off.
    if (isBusy(codeSubmit) || isOff(codeSubmit)) {
      return;
    }
    clearErrors('code');
    // The code sent line has done its job; it must not sit beside a refusal
    // (review fix IN-B07). The expiry notice stays: it is still true.
    if (codeStatus.textContent === CODE_SENT_MESSAGE) {
      setText(codeStatus, '');
      codeStatus.hidden = true;
    }
    resetShow();
    var reset = state.mode === 'reset';
    var path = reset ? '/api/auth/reset/code' : '/api/auth/sign-up/code';
    var payload = reset ? { code: codeInput.value, password: codeNewPassword.value } : { code: codeInput.value };
    var mine = state.generation;
    setBusy(codeSubmit, CHECKING_LABEL);
    postJSON(path, payload).then(function (result) {
      clearBusy(codeSubmit);
      // A confirmed account is signed in: it goes on whatever the card shows now.
      if (result.ok && !reset) {
        goNext(result.body.next);
        return;
      }
      if (mine !== state.generation) {
        return;
      }
      if (result.ok) {
        finishReset();
        return;
      }
      if (result.status === 400 && fieldOf(result) === 'code') {
        refuseCode();
        return;
      }
      var field = fieldFor(result, { password: codeNewPassword });
      showError(codeError, field, messageFor(result));
      if (field) {
        field.focus();
      }
    });
  }

  // A reset signs nobody in: the card goes back to Sign in with the email kept.
  function finishReset() {
    stopTimers();
    codeInput.value = '';
    codeNewPassword.value = '';
    signinEmail.value = state.email;
    signinPassword.value = '';
    showPanel('signin');
    setText(signinStatus, PASSWORD_SET_MESSAGE);
    signinStatus.hidden = false;
    signinPassword.focus();
  }

  function resend() {
    if (state.resendBusy || isOff(codeResend)) {
      return;
    }
    clearErrors('code');
    state.resendBusy = true;
    renderResend();
    var mine = state.generation;
    postJSON('/api/auth/code/resend', {}).then(function (result) {
      // The card has moved on: the next code screen sets Resend up itself.
      if (mine !== state.generation) {
        return;
      }
      state.resendBusy = false;
      if (!result.ok) {
        renderResend();
        showError(codeError, null, messageFor(result));
        return;
      }
      state.tries = 0;
      lockCodeButton(false);
      startResendWait(fullResendSeconds);
      startExpiry(numberAttr('data-code-ttl-seconds'));
      setText(codeStatus, CODE_SENT_MESSAGE);
      codeStatus.hidden = false;
    });
  }

  // changeEmail goes back to the panel the person came from. Nothing is sent.
  function changeEmail() {
    stopTimers();
    if (state.from === 'forgot') {
      forgotEmail.value = state.email;
      showPanel('forgot');
      forgotEmail.focus();
    } else {
      createEmail.value = state.email;
      showPanel('create');
      createEmail.focus();
    }
  }

  function openForgot() {
    forgotEmail.value = signinEmail.value;
    showPanel('forgot');
    forgotEmail.focus();
  }

  function backToSignIn() {
    signinEmail.value = forgotEmail.value;
    showPanel('signin');
    signinEmail.focus();
  }

  // ---- Wiring ---------------------------------------------------------------------

  [tabCreate, tabSignin].forEach(function (tab) {
    tab.addEventListener('click', function () {
      selectTab(tab === tabSignin ? 'signin' : 'create', true);
    });
    tab.addEventListener('keydown', onTabKey);
  });

  forms.create.addEventListener('submit', submitCreate);
  forms.signin.addEventListener('submit', submitSignIn);
  forms.code.addEventListener('submit', submitCode);
  forms.forgot.addEventListener('submit', submitForgot);

  showButtons.forEach(function (button) {
    button.addEventListener('click', function () {
      var input = byId(button.getAttribute('data-show-for'));
      if (input) {
        toggleShow(input, button);
      }
    });
  });

  byId('signin-forgot').addEventListener('click', openForgot);
  byId('forgot-back').addEventListener('click', backToSignIn);
  byId('code-change-email').addEventListener('click', changeEmail);
  codeResend.addEventListener('click', resend);

  codeInput.addEventListener('input', function () {
    var cleaned = cleanCode(codeInput.value);
    if (cleaned !== codeInput.value) {
      codeInput.value = cleaned;
    }
  });
  codeInput.addEventListener('paste', onCodePaste);

  // Back from Mail, or from the back and forward cache: the clocks catch up.
  document.addEventListener('visibilitychange', function () {
    if (document.visibilityState !== 'hidden') {
      catchUp();
    }
  });
  window.addEventListener('pageshow', catchUp);

  // ---- Start ----------------------------------------------------------------------

  // The server has already drawn the right panel. A page that opens on the code
  // screen (a reload, or a person back from reading their mail) takes the address
  // from the page text and the clocks from the data attributes.
  var start = card.getAttribute('data-start-panel');
  if (start === 'code') {
    var mode = card.getAttribute('data-code-mode') === 'reset' ? 'reset' : 'create';
    state.panel = 'code';
    state.from = mode === 'reset' ? 'forgot' : 'create';
    state.email = codeEmail.textContent.trim();
    setCodeMode(mode);
    startResendWait(numberAttr('data-resend-seconds'));
    var left = numberAttr('data-code-seconds-left');
    if (left > 0) {
      startExpiry(left);
    } else {
      expireNow();
    }
  } else {
    state.panel = start === 'signin' ? 'signin' : 'create';
  }
}());
