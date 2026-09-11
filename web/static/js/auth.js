// auth.js — the login gate's request-link flow (plan 01.1-01).
//
// This page ships no app-shell and does not load app.js, so it owns a
// small self-contained setText helper rather than reaching for
// window.Pinalert.
//
// SECURITY (T-01-03): every string that reaches the DOM here — server
// validation messages included — is inserted as text content, never
// assembled into markup for the browser to parse.
(function () {
  'use strict';

  var loginFormPanel = document.getElementById('login-form-panel');
  var emailInput = document.getElementById('login-email');
  var requestButton = document.getElementById('request-link');
  var authError = document.getElementById('auth-error');
  var checkInbox = document.getElementById('check-inbox');
  var resendButton = document.getElementById('resend-link');
  var refreshButton = document.getElementById('refresh-inbox');
  var linkExpiryNotice = document.getElementById('link-expiry-notice');

  if (!requestButton || !emailInput) {
    return;
  }

  function setText(el, value) {
    if (!el) {
      return;
    }
    el.textContent = value === null || value === undefined ? '' : String(value);
  }

  // Two independent timers, per UI-SPEC item 9: the resend cooldown and the
  // link's own 5-minute expiry are separate clocks and neither is ever
  // derived from the other. Both durations are read off the check-inbox
  // panel below — server-rendered values, not hard-coded here, since the
  // server is the single source of truth for both
  // (internal/api/handlers.loginGateViewModel).
  var cooldownSeconds = checkInbox ? parseInt(checkInbox.getAttribute('data-cooldown-seconds'), 10) : 0;
  var linkTTLSeconds = checkInbox ? parseInt(checkInbox.getAttribute('data-link-ttl-seconds'), 10) : 0;
  if (!(cooldownSeconds > 0)) {
    cooldownSeconds = 0;
  }
  if (!(linkTTLSeconds > 0)) {
    linkTTLSeconds = 0;
  }

  var resendCountdownTimer = null;
  var linkExpiryTimer = null;

  // startResendCountdown disables #resend-link and decrements a per-second
  // label until zero, at which point the button is re-enabled and restored
  // to its resting label. A fresh call always clears any prior countdown
  // first, so a resend never leaves two intervals ticking against the same
  // button.
  function setResendCountdownLabel(remaining) {
    setText(resendButton, 'Resend in ' + remaining + 's');
  }

  function startResendCountdown(seconds) {
    if (!resendButton || seconds <= 0) {
      return;
    }
    if (resendCountdownTimer !== null) {
      clearInterval(resendCountdownTimer);
    }
    var remaining = seconds;
    resendButton.disabled = true;
    setResendCountdownLabel(remaining);
    resendCountdownTimer = setInterval(function () {
      remaining -= 1;
      if (remaining <= 0) {
        clearInterval(resendCountdownTimer);
        resendCountdownTimer = null;
        resendButton.disabled = false;
        setText(resendButton, 'Resend link');
        return;
      }
      setResendCountdownLabel(remaining);
    }, 1000);
  }

  // startLinkExpiryTimer runs entirely independently of the resend
  // countdown above — a separate variable, a separate interval — and after
  // `seconds` elapses reveals the inline expiry notice. Never inferred from
  // the resend cooldown reaching zero; conflating the two is exactly how a
  // 45-second cooldown would start silently claiming a 5-minute link
  // expired (UI-SPEC item 9).
  function startLinkExpiryTimer(seconds) {
    if (!linkExpiryNotice || seconds <= 0) {
      return;
    }
    if (linkExpiryTimer !== null) {
      clearInterval(linkExpiryTimer);
    }
    linkExpiryNotice.hidden = true;
    var remaining = seconds;
    linkExpiryTimer = setInterval(function () {
      remaining -= 1;
      if (remaining <= 0) {
        clearInterval(linkExpiryTimer);
        linkExpiryTimer = null;
        setText(linkExpiryNotice, 'This link has expired. Request a new one above.');
        linkExpiryNotice.hidden = false;
      }
    }, 1000);
  }

  // requestLink POSTs the email to the request-link endpoint below. On
  // success it reveals the check-inbox panel in place — no navigation — and pushes
  // ?sent=<email> into the address bar via history.replaceState so a
  // manual reload ("Check again") re-renders the same waiting state
  // server-side without ever restarting the globe animation (UI-SPEC
  // item 4).
  function requestLink(email) {
    setText(authError, '');
    requestButton.disabled = true;
    setText(requestButton, 'Sending…');

    return fetch('/api/auth/request-link', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email })
    }).then(function (res) {
      if (res.status === 429) {
        throw new Error('Too many requests — try again in a minute.');
      }
      return res.json().catch(function () {
        return {};
      }).then(function (body) {
        if (!res.ok) {
          var message = (body && body.error && body.error.message) || 'Something went wrong. Try again.';
          throw new Error(message);
        }
        return body;
      });
    }).then(function (body) {
      requestButton.disabled = false;
      setText(requestButton, 'Send verification link');

      var confirmedEmail = (body && body.email) || email;
      if (loginFormPanel) {
        loginFormPanel.hidden = true;
      }
      if (checkInbox) {
        checkInbox.hidden = false;
      }
      var url = new URL(window.location.href);
      url.searchParams.set('sent', confirmedEmail);
      window.history.replaceState(null, '', url.toString());

      // A link was just sent successfully — both clocks start now.
      startResendCountdown(cooldownSeconds);
      startLinkExpiryTimer(linkTTLSeconds);
    }).catch(function (err) {
      requestButton.disabled = false;
      setText(requestButton, 'Send verification link');
      setText(authError, (err && err.message) || 'Something went wrong. Try again.');
      // A refused (429) or otherwise failed request sent nothing, so
      // neither timer starts or resets here — its clock must not move
      // (UI-SPEC item 6).
    });
  }

  requestButton.addEventListener('click', function () {
    requestLink(emailInput.value.trim());
  });

  if (resendButton) {
    resendButton.addEventListener('click', function () {
      requestLink(emailInput.value.trim());
    });
  }

  if (refreshButton) {
    refreshButton.addEventListener('click', function () {
      window.location.reload();
    });
  }
}());
