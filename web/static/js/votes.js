// votes.js — window.PinalertVotes: the client half of the confirm/dispute/
// resolve/reopen mechanic. One block builder and one updater serve both the
// feed row (feed.js) and the map pin popup (map.js) from this single module
// (D-01) — not two implementations of the same controls.
//
// A DIVERGENCE FROM THE REPORT-SUBMISSION PATH, STATED IN THE STRONGEST
// TERMS: the report-submission flow deliberately falls back to a configured
// default position when the browser's location prompt is denied, because
// submission must never dead-end during an emergency. Voting requires the
// exact opposite (D-18): a vote cast from a position the voter is not
// standing in silently destroys TRUST-03's distinct-geohash-cell
// independence predicate. Nothing in this file may recentre a view or read
// a fallback coordinate on denial — it must reject outright. The two
// existing call sites with the submission behaviour are correct where they
// are and must not be "made consistent" with this file.
//
// SECURITY (T-01-03): every string reaching the DOM — including the
// server's own error messages — goes through Pinalert.setText, never a
// markup-parsing sink. The rule is about the sink, not about who authored
// the string.
//
// D-04: this module never predicts a vote's outcome. It renders whatever
// the server most recently answered and refetches after every write rather
// than guessing.
//
// Deliberate deviation from 02-RESEARCH.md's own code sketch, which attaches
// castVote onto window.Pinalert: this file exports window.PinalertVotes
// instead. window.Pinalert is the shared store (app.js's own returned
// object) — neither map.js nor feed.js mutates another module's public API
// from outside, and this module owns its own surface for the same reason
// window.PinalertMap does.
(function () {
  'use strict';

  // The four path segments 02-03b mounted under /api/reports/{id}/{action}.
  // All four are listed even though this plan renders controls for only
  // the first two: the allowlist describes the API's shape, not this
  // plan's UI, so a future resolve/reopen control can call castVote with
  // 'resolve' or 'reopen' without editing a security control. Same purpose
  // as app.js's iconClass CATEGORIES allowlist (T-01-17): a caller-supplied
  // string is validated against a fixed list before it is interpolated
  // into a URL path.
  var VOTE_ACTIONS = ['confirm', 'dispute', 'resolve', 'reopen'];

  var VOTER_LOCATION_CACHE_KEY = 'pinalert_voter_location';

  // The Copywriting Contract's GPS-denial sentence, verbatim. Its tone
  // deliberately differs from the submission path's location notice: that
  // copy offers a workaround because one exists; this copy must not imply
  // one, because D-18 means there is none.
  var GPS_DENIED_MESSAGE = 'Voting needs your location, so nearby confirmations can be verified as independent. Turn on location access for this site and try again.';

  // The Copywriting Contract's generic vote-failure sentence, verbatim.
  // Used only when no server message is available (a transport failure
  // with no response body); whenever the server answered, its own message
  // wins — 02-03b already ships the Copywriting Contract's exact strings
  // for 400/401/403/404/409, and duplicating them here would create two
  // copies to keep in sync.
  var GENERIC_FAILURE_MESSAGE = "Couldn't record your vote. Check your connection and try again.";

  // getVoterLocation resolves a Promise of {latitude, longitude}. It
  // consults the per-session cache before ever raising the browser's own
  // geolocation prompt (D-17), and REJECTS — never resolves a substitute
  // position — when the prompt is denied, unsupported, or times out
  // (D-18). Every rejection carries fieldMessage set to GPS_DENIED_MESSAGE,
  // matching app.js's submitReport convention so one catch clause upstream
  // handles a denied prompt and a server refusal identically.
  function getVoterLocation() {
    return new Promise(function (resolve, reject) {
      try {
        var cached = sessionStorage.getItem(VOTER_LOCATION_CACHE_KEY);
        if (cached) {
          var parsed = JSON.parse(cached);
          if (parsed && isFinite(parsed.latitude) && isFinite(parsed.longitude)) {
            resolve({ latitude: parsed.latitude, longitude: parsed.longitude });
            return;
          }
        }
      } catch (e) {
        // A browser in a privacy mode can throw on storage access, and a
        // corrupted cached value must not throw past this caller. A cache
        // that cannot be read is treated as absent: a fresh prompt
        // follows below, degrading D-17's convenience while D-18 still
        // holds, which is the property that actually matters.
      }

      if (!navigator.geolocation) {
        var unsupportedErr = new Error(GPS_DENIED_MESSAGE);
        unsupportedErr.fieldMessage = GPS_DENIED_MESSAGE;
        reject(unsupportedErr);
        return;
      }

      navigator.geolocation.getCurrentPosition(
        function (pos) {
          var coords = {
            latitude: pos.coords.latitude,
            longitude: pos.coords.longitude
          };
          try {
            sessionStorage.setItem(VOTER_LOCATION_CACHE_KEY, JSON.stringify(coords));
          } catch (e) {
            // Same privacy-mode/quota concern as the read above — a cache
            // write that fails costs only the next vote's convenience,
            // never this vote's correctness.
          }
          resolve(coords);
        },
        function () {
          // No recentring, no notice, no default position, no second
          // attempt. D-18 is a hard stop.
          var deniedErr = new Error(GPS_DENIED_MESSAGE);
          deniedErr.fieldMessage = GPS_DENIED_MESSAGE;
          reject(deniedErr);
        },
        { timeout: 8000 }
      );
    });
  }

  // castVote POSTs a vote and resolves with the parsed 200 body
  // ({visibility, reason}). Transport only — no DOM, no refetch, no
  // decision about what to show. Keeping this function DOM-free is what
  // lets a future resolve/reopen control reuse it unchanged.
  function castVote(reportId, action) {
    if (VOTE_ACTIONS.indexOf(action) === -1) {
      var actionErr = new Error(GENERIC_FAILURE_MESSAGE);
      actionErr.fieldMessage = GENERIC_FAILURE_MESSAGE;
      return Promise.reject(actionErr);
    }

    return getVoterLocation().then(function (coords) {
      return fetch('/api/reports/' + encodeURIComponent(String(reportId)) + '/' + action, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        // Exactly the two fields the server's decoder declares. That
        // decoder runs with an unknown-field guard, so any extra key here
        // would be a loud 400 rather than a silently dropped field. The
        // voter's cell is computed server-side and must never be sent
        // from here (T-02-01).
        body: JSON.stringify(coords)
      }).then(function (res) {
        return res.json().catch(function () {
          return {};
        }).then(function (body) {
          if (!res.ok) {
            var fieldError = (body && body.error) || {
              field: 'body',
              message: GENERIC_FAILURE_MESSAGE
            };
            var err = new Error(fieldError.message);
            err.field = fieldError.field;
            err.fieldMessage = fieldError.message;
            throw err;
          }
          return body;
        });
      });
    });
  }

  window.PinalertVotes = {
    getVoterLocation: getVoterLocation,
    castVote: castVote,
    GPS_DENIED_MESSAGE: GPS_DENIED_MESSAGE,
    GENERIC_FAILURE_MESSAGE: GENERIC_FAILURE_MESSAGE
  };
}());
