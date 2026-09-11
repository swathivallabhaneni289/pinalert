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

  if (!requestButton || !emailInput) {
    return;
  }

  function setText(el, value) {
    if (!el) {
      return;
    }
    el.textContent = value === null || value === undefined ? '' : String(value);
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
    }).catch(function (err) {
      requestButton.disabled = false;
      setText(requestButton, 'Send verification link');
      setText(authError, (err && err.message) || 'Something went wrong. Try again.');
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
