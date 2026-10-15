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
  //
  // 'unresolve' is the fifth path segment (03.1 plan 08): the take back of a
  // mistaken Mark resolved. It joins the closed list like the others, so it
  // is validated before it is interpolated, and nothing else is.
  var VOTE_ACTIONS = ['confirm', 'dispute', 'resolve', 'reopen', 'unresolve'];

  var VOTER_LOCATION_CACHE_KEY = 'pinalert_voter_location';

  // The worst accuracy radius, in metres, at which a fix may be cached
  // (WR-07). 300 m is about two geohash 7 cells (a cell is roughly 150 m
  // across), so a fix worse than this can land in a neighbouring cell and
  // would turn one person into several places if it were reused. A fix worse
  // than 300 m may still be used for the single vote that asked for it, but is
  // never stored. A client side limit, not a trust rule.
  var VOTER_LOCATION_MAX_CACHE_ACCURACY_M = 300;

  // The code a GeolocationPositionError carries when the voter or the browser
  // refused the request. The other two codes, position unavailable (2) and
  // timeout (3), are not refusals (WR-07).
  var GEOLOCATION_PERMISSION_DENIED = 1;

  // The hard ceiling on reusing a cached fix as the voter's place when a new
  // fix could not be taken (10 minutes, WR-02). Beyond it the cached position
  // is no longer where the voter is, so the vote is refused with the location
  // sentence rather than sent from an old place. An entry with no capture
  // time never passes. A client side limit, not a trust rule: the server's
  // 1 km check stays the only authority on whether a vote counts.
  var VOTER_LOCATION_STALE_MAX_MS = 10 * 60 * 1000;

  // How long a cached fix is reused without asking again, in milliseconds
  // (outline UI-SPEC open item 6). This is a client side timer for location
  // freshness, not a trust rule: the server's 1 km check stays the only
  // authority on whether a vote counts (D-02).
  var VOTER_LOCATION_FRESH_MS = 60000;

  // How old a cached fix may be and still drive the too far hint, in
  // milliseconds (UI-SPEC open item 6: a stale fix never marks a button). A
  // client side timer, not a trust threshold: the server decides every vote.
  var VOTER_LOCATION_HINT_MS = 120000;

  // The three buttons the too far hint marks (RR-3). Yes, Cancel and Reopen
  // are never in this list.
  var HINT_BUTTON_SELECTORS = ['.vote-btn--confirm', '.vote-btn--dispute', '.vote-btn--resolve'];

  // The too far sentence is these two literals around the radius the page
  // supplies. The tail starts with a non breaking space so the unit never
  // wraps alone. A Go test checks the joined sentence against the server's.
  var TOO_FAR_LEAD = 'Too far to vote. You need to be within ';
  var TOO_FAR_TAIL = '\u00a0km of this report.';

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

  // Shown in the row when the server recorded the vote but the refetch that
  // follows it failed, so the counters on screen are the old ones (WR-08).
  var VOTE_SAVED_REFRESH_FAILED_MESSAGE = 'Your vote was saved, but the list could not refresh. The counts shown may be out of date.';

  // Copywriting Contract strings for Mark Resolved (Task 1, 02-07), verbatim.
  var RESOLVE_BUTTON_LABEL = 'Mark resolved';
  var RESOLVE_BUTTON_LABEL_IN_FLIGHT = 'Marking…';
  var RESOLVE_CONFIRM_HEADING = 'Mark this report resolved?';
  var RESOLVE_CONFIRM_AFFIRM_LABEL = 'Yes, mark resolved';
  var RESOLVE_CONFIRM_CANCEL_LABEL = 'Cancel';
  var RESOLVE_SUCCEEDED_TOAST = 'Report marked resolved.';
  var RESOLVE_PENDING_TOAST = "Your resolve vote was recorded. It needs agreement from other nearby confirmers before it's marked resolved.";

  // Copywriting Contract strings for Reopen, declared here and exported so
  // Task 3's activity.js consumes them rather than declaring a second copy
  // table. Only REOPEN_BUTTON_LABEL, REOPEN_BUTTON_LABEL_IN_FLIGHT and
  // REOPEN_SUCCEEDED_TOAST are ever rendered by this phase — the Activity
  // page's button has exactly one outcome (D-16 amended 2026-09-15) and
  // never reaches REOPEN_PENDING_TOAST. It stays declared for
  // API-consistency with castVote's generic outcome handling, per
  // 02-UI-SPEC.md's amended Reopen section, which states plainly it is
  // retained but unreachable through that button.
  var REOPEN_BUTTON_LABEL = 'Reopen · not actually resolved';
  var REOPEN_BUTTON_LABEL_IN_FLIGHT = 'Reopening…';
  var REOPEN_SUCCEEDED_TOAST = 'Report reopened.';
  var REOPEN_PENDING_TOAST = 'Your reopen vote was recorded. It needs agreement from other nearby confirmers before it reopens.';

  // The take back words (03.1 UI-SPEC C42 to C57), verbatim. The server owns
  // the limit and sends undos_left; these are only the sentences around it.
  var CLOSED_NOTE_RESOLVED = 'This report was marked resolved, so voting is off.';
  var CLOSED_NOTE_EXPIRED = 'This report has faded, so voting is off.';
  var TAKE_BACK_STATEMENT = 'You marked this resolved.';
  var TAKE_BACK_STATEMENT_WAITING = 'You marked this resolved. It closes when other people nearby agree.';
  var TAKE_BACK_STATEMENT_POSTER = 'People nearby marked this resolved.';
  var TAKE_BACK_BUTTON_LABEL = 'Undo';
  var TAKE_BACK_BUTTON_NAME = 'Undo my resolved mark';
  var TAKE_BACK_BUTTON_LABEL_IN_FLIGHT = 'Undoing…';
  var TAKE_BACK_USED_MESSAGE = 'You have used both undos on this report.';
  var TAKE_BACK_LIMIT_CONFIRM_LINE = "You have used both undos on this report, so this can't be undone.";
  var TAKE_BACK_ERROR_MESSAGE = "Couldn't undo that. Try again.";
  var TAKE_BACK_FADED_MESSAGE = "This report has faded, so it can't be changed.";
  var TAKE_BACK_SUCCEEDED_CLOSED_TOAST = 'Taken back. The report is open again.';
  var TAKE_BACK_SUCCEEDED_WAITING_TOAST = 'Taken back.';
  var TOAST_ACTION_LABEL = 'Undo';
  var TOAST_ACTION_NAME = 'Undo marking this report resolved';

  // The server's own sentence for a vote on a report that has run out its
  // time (handlers/votes.go). A take back that meets it shows the faded line
  // instead, because to the reader the report has simply faded.
  var SERVER_EXPIRED_REFUSAL = 'This report has expired.';

  // The retracted visibility slug, held here as its own constant rather
  // than imported: this module loads before visibility.js (see the header
  // comment on window.PinalertVisibility's own file) and so cannot read
  // its VISIBILITY_STATES array at evaluation time. Kept in step with that
  // array by TestResolutionOutcomeSlugMatchesVisibilityStates.
  var RETRACTED_VISIBILITY_SLUG = 'retracted';

  // Numbers the vote blocks this page builds, so the question id and the
  // too far sentence id are unique per block. They are never built from
  // report data.
  var voteBlockSeq = 0;

  // The waits the server has told this page about, one entry per report and
  // vote kind (D-12). It lives in memory only: it is never written to any
  // browser storage or queue, because a vote is never saved for later (D-18),
  // and a reload starts with none. A tap during a wait is answered here and
  // sends nothing.
  var voteWaits = {};

  // Which button takes focus back once a vote is released, keyed by the
  // action that was sent. Confirm and Dispute return to the pressed button
  // (UI-SPEC Busy state). Yes returns to Mark resolved (A52).
  var RELEASE_FOCUS_SELECTORS = {
    confirm: '.vote-btn--confirm',
    dispute: '.vote-btn--dispute',
    'confirm-resolve': '.vote-btn--resolve'
  };

  // Reads one cache entry, as stored text, into latitude, longitude and the
  // capture time in epoch milliseconds, or null. Unparsable text and a
  // non finite coordinate give null. A missing or non numeric capture time
  // gives a null time, and an entry with no capture time is unusable for any
  // vote, fresh or stale (WR-02).
  function parseVoterLocation(raw) {
    if (!raw) {
      return null;
    }
    var parsed;
    try {
      parsed = JSON.parse(raw);
    } catch (e) {
      return null;
    }
    if (!parsed || !isFinite(parsed.latitude) || !isFinite(parsed.longitude)) {
      return null;
    }
    return {
      latitude: parsed.latitude,
      longitude: parsed.longitude,
      at: (typeof parsed.at === 'number' && isFinite(parsed.at)) ? parsed.at : null
    };
  }

  // True only for a parsed entry with a capture time that is not in the
  // future and is younger than maxAgeMs. A capture time ahead of the clock
  // counts as stale. Plan 03-18's hint reuses this with its own bound.
  function isFreshVoterLocation(entry, maxAgeMs) {
    if (!entry || entry.at === null) {
      return false;
    }
    var age = Date.now() - entry.at;
    return age >= 0 && age < maxAgeMs;
  }

  // Resolves to 'granted', 'denied', 'prompt' or 'unknown' and never
  // rejects. 'unknown' covers a browser without the Permissions API, a query
  // that throws or rejects, and any state outside the closed list. Where the
  // API is missing the silent refresh of D-02 simply does not run and the
  // cached fix is reused (RESEARCH A7).
  function geolocationPermissionState() {
    return new Promise(function (resolve) {
      try {
        if (!navigator.permissions || typeof navigator.permissions.query !== 'function') {
          resolve('unknown');
          return;
        }
        navigator.permissions.query({ name: 'geolocation' }).then(function (status) {
          var state = status && status.state;
          if (state === 'granted' || state === 'denied' || state === 'prompt') {
            resolve(state);
          } else {
            resolve('unknown');
          }
        }, function () {
          resolve('unknown');
        });
      } catch (e) {
        resolve('unknown');
      }
    });
  }

  // Resolves a Promise of latitude and longitude only. The capture time is
  // never resolved. The rules, in order (D-02, builds on D-17 and D-18):
  //  1. A cached fix younger than VOTER_LOCATION_FRESH_MS is reused without
  //     asking, unless the caller passed fresh true.
  //  2. Otherwise the permission state decides. Denied rejects outright.
  //     Granted takes a silent fix with maximumAge 0 and, if that fails,
  //     reuses the cached fix unless fresh was asked for or the cached fix
  //     is older than VOTER_LOCATION_STALE_MAX_MS.
  //  3. Prompt or unknown (no Permissions API) reuses a cached fix that is
  //     younger than VOTER_LOCATION_STALE_MAX_MS and does not ask the browser
  //     again. A session with no usable cached fix raises the browser's own
  //     prompt, exactly as the first vote of a session always has. A fresh
  //     request in this state rejects: a retry never prompts.
  // Every rejection carries fieldMessage set to GPS_DENIED_MESSAGE, matching
  // the submitReport convention in app.js so one catch clause upstream
  // handles a denied prompt and a server refusal identically. No rejection
  // is ever turned into a substitute position (D-18).
  function getVoterLocation(opts) {
    var fresh = !!(opts && opts.fresh);
    return new Promise(function (resolve, reject) {
      var cached = null;
      try {
        cached = parseVoterLocation(sessionStorage.getItem(VOTER_LOCATION_CACHE_KEY));
      } catch (e) {
        // A browser in a privacy mode can throw on storage access. A cache
        // that cannot be read is treated as absent: a fresh prompt
        // follows below, degrading D-17's convenience while D-18 still
        // holds, which is the property that actually matters.
      }

      function useCached() {
        resolve({ latitude: cached.latitude, longitude: cached.longitude });
      }

      function deny() {
        var deniedErr = new Error(GPS_DENIED_MESSAGE);
        deniedErr.fieldMessage = GPS_DENIED_MESSAGE;
        reject(deniedErr);
      }

      // The one and only place the module asks the browser for a position.
      // onFailure runs when the browser has no geolocation support or the
      // request fails. There is no recentring, no notice and no default
      // position. D-18 is a hard stop.
      //
      // The first attempt asks for a high accuracy fix. A device that cannot
      // give one within the timeout (a cold GPS start, a GPS only provider
      // indoors) reports a timeout or "position unavailable", which is not a
      // refusal, so it gets exactly one more attempt at the default accuracy
      // before the vote is turned away (WR-07). Only a refusal by the voter or
      // the browser ends the request at once, and only that is the permission
      // case the denial sentence is about. The second attempt is a real fix
      // taken now, never a substitute position.
      function requestPosition(onFailure) {
        if (!navigator.geolocation) {
          onFailure();
          return;
        }
        function attempt(highAccuracy) {
          navigator.geolocation.getCurrentPosition(
            function (pos) {
              var coords = {
                latitude: pos.coords.latitude,
                longitude: pos.coords.longitude
              };
              // A fix with no accuracy figure, or one worse than the bound, is
              // used for this vote only and never cached.
              var accuracy = pos.coords.accuracy;
              if (typeof accuracy === 'number' && accuracy <= VOTER_LOCATION_MAX_CACHE_ACCURACY_M) {
                try {
                  sessionStorage.setItem(VOTER_LOCATION_CACHE_KEY, JSON.stringify({
                    latitude: coords.latitude,
                    longitude: coords.longitude,
                    at: Date.now()
                  }));
                } catch (e) {
                  // Same privacy-mode/quota concern as the read above. A cache
                  // write that fails costs only the next vote's convenience,
                  // never this vote's correctness.
                }
              }
              resolve(coords);
            },
            function (err) {
              if (highAccuracy && !(err && err.code === GEOLOCATION_PERMISSION_DENIED)) {
                attempt(false);
                return;
              }
              onFailure();
            },
            highAccuracy
              ? { enableHighAccuracy: true, timeout: 8000, maximumAge: 0 }
              : { enableHighAccuracy: false, timeout: 8000, maximumAge: 0 }
          );
        }
        attempt(true);
      }

      if (!fresh && isFreshVoterLocation(cached, VOTER_LOCATION_FRESH_MS)) {
        useCached();
        return;
      }

      function decide(state) {
        if (state === 'denied') {
          deny();
          return;
        }
        if (state === 'granted') {
          requestPosition(function () {
            if (!fresh && isFreshVoterLocation(cached, VOTER_LOCATION_STALE_MAX_MS)) {
              useCached();
            } else {
              deny();
            }
          });
          return;
        }
        // Prompt or unknown. A retry never prompts.
        if (fresh) {
          deny();
          return;
        }
        if (isFreshVoterLocation(cached, VOTER_LOCATION_STALE_MAX_MS)) {
          useCached();
          return;
        }
        requestPosition(deny);
      }

      geolocationPermissionState().then(decide, function () {
        decide('unknown');
      });
    });
  }

  // The position a reporter's own resolve or reopen carries (WR-04). The
  // server exempts the reporter from the 1 km rule for these two actions, so
  // the browser must not ask for a location it does not need and must not
  // block the action when location is denied. It still sends a cached fix
  // that is fresh, because that is a real position. With none, it sends
  // 0,0 as a placeholder the server never judges for the reporter. It never
  // prompts and never rejects.
  function ownReportLocation() {
    var fix = peekVoterLocation();
    if (isFreshVoterLocation(fix, VOTER_LOCATION_FRESH_MS)) {
      return Promise.resolve({ latitude: fix.latitude, longitude: fix.longitude });
    }
    return Promise.resolve({ latitude: 0, longitude: 0 });
  }

  // voteWaitKind names the cooldown a vote action belongs to, in the server's
  // own key words: Mark resolved, Reopen and the take back share the
  // resolution cooldown, every other action is content (D-12).
  function voteWaitKind(action) {
    if (action === 'resolve' || action === 'reopen' || action === 'unresolve') {
      return 'resolution';
    }
    return 'content';
  }

  // rememberVoteWait keeps the server's wait answer for one report and vote
  // kind, and returns the whole seconds it kept (at least 1). The message is
  // the server's own sentence, shown as it came.
  function rememberVoteWait(reportId, action, seconds, message, field) {
    var wait = Math.ceil(Number(seconds));
    if (!isFinite(wait) || wait < 1) {
      wait = 1;
    }
    voteWaits[String(reportId) + ':' + voteWaitKind(action)] = {
      until: Date.now() + wait * 1000,
      seconds: wait,
      message: String(message || GENERIC_FAILURE_MESSAGE),
      field: field || ''
    };
    return wait;
  }

  // knownVoteWait returns null when no wait runs for this report and vote
  // kind. While one runs it returns an Error shaped like the one a 429 answer
  // gives, with the stored sentence reworded for the time left. When the
  // original wait phrase is not found in the sentence, the sentence is kept
  // exactly as the server sent it.
  function knownVoteWait(reportId, action) {
    var key = String(reportId) + ':' + voteWaitKind(action);
    var entry = voteWaits[key];
    if (!entry) {
      return null;
    }
    if (Date.now() >= entry.until) {
      delete voteWaits[key];
      return null;
    }
    var left = Math.max(1, Math.ceil((entry.until - Date.now()) / 1000));
    var original = Pinalert.formatWait(entry.seconds);
    var at = entry.message.indexOf(original);
    var message = at === -1
      ? entry.message
      : entry.message.slice(0, at) + Pinalert.formatWait(left) + entry.message.slice(at + original.length);
    var err = new Error(message);
    err.field = entry.field;
    err.fieldMessage = message;
    err.status = 429;
    err.retryAfterSeconds = left;
    return err;
  }

  // castVote POSTs a vote and resolves with the parsed 200 body
  // ({visibility, reason}). It retries once, and only when the server
  // refused the vote with error field location (too far from the report),
  // using a fix taken silently now under an already granted permission. If
  // no such fix can be taken, the server's own sentence is what the caller
  // sees. Transport only: no DOM, no refetch, no decision about what to
  // show. Keeping this function DOM-free is what lets a future
  // resolve/reopen control reuse it unchanged (D-02). opts.ownReport is true
  // only for the viewer's own report and only matters for resolve and reopen:
  // those two skip the location request entirely (WR-04). The server decides
  // from the database identity, never from this flag, so a wrong flag costs
  // at most a 403 that the retry below answers with a real fix.
  //
  // opts.location is a fix that getVoterLocation has just resolved for this
  // very tap (refreshHintThenRetry, WR-03): it is sent as it is, so a coarse
  // fix that was not cached still serves the tap that took it, and the tap
  // does not take a second fix. Only a finite latitude and longitude pair is
  // accepted. Anything else is ignored and the normal location request runs.
  function castVote(reportId, action, opts) {
    if (VOTE_ACTIONS.indexOf(action) === -1) {
      var actionErr = new Error(GENERIC_FAILURE_MESSAGE);
      actionErr.fieldMessage = GENERIC_FAILURE_MESSAGE;
      return Promise.reject(actionErr);
    }

    // While the server's wait for this report and vote kind still runs, the
    // tap is answered here with the same sentence and nothing is read or
    // sent. This also covers the take back below.
    var pendingWait = knownVoteWait(reportId, action);
    if (pendingWait) {
      return Promise.reject(pendingWait);
    }

    // A take back adds no claim, so it asks for no location at all and sends
    // an empty object (03.1 D-22): the server checks the account and reads no
    // position for it. This is neither of the two sanctioned location
    // exceptions below; it is a request with no coordinates in it, so it
    // cannot weaken the independence rule that the location is there to keep.
    if (action === 'unresolve') {
      return postVote(null);
    }

    var skipLocation = !!(opts && opts.ownReport) && (action === 'resolve' || action === 'reopen');
    var given = opts && opts.location;
    var haveGiven = !!given && typeof given.latitude === 'number' && isFinite(given.latitude) &&
      typeof given.longitude === 'number' && isFinite(given.longitude);
    var firstFix;
    if (skipLocation) {
      firstFix = ownReportLocation();
    } else if (haveGiven) {
      firstFix = Promise.resolve({ latitude: given.latitude, longitude: given.longitude });
    } else {
      firstFix = getVoterLocation({ fresh: false });
    }
    return firstFix.then(postVote).catch(function (err) {
      // A location denial carries no field, a transport failure carries
      // none either and a 400 carries another field, so none of those is
      // retried. The retry never prompts: a fresh fix is taken only under
      // an already granted permission.
      if (!err || err.field !== 'location') {
        throw err;
      }
      return getVoterLocation({ fresh: true }).then(postVote, function () {
        throw err;
      });
    });

    function postVote(coords) {
      return fetch('/api/reports/' + encodeURIComponent(String(reportId)) + '/' + action, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        // Exactly the two fields the server's decoder declares. That
        // decoder runs with an unknown-field guard, so any extra key here
        // would be a loud 400 rather than a silently dropped field. The
        // voter's cell is computed server-side and must never be sent
        // from here (T-02-01). The cached capture time stays out of it.
        // A take back (coords is null) sends the empty object.
        body: coords === null ? '{}' : JSON.stringify({ latitude: coords.latitude, longitude: coords.longitude })
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
            err.status = res.status;
            if (res.status === 429) {
              // The server's own sentence and wait are remembered, so a tap
              // during the wait is answered without a request. A 429 is
              // never retried.
              var waitSeconds = Number(body && body.retry_after_seconds);
              if (!isFinite(waitSeconds) || !(waitSeconds > 0)) {
                waitSeconds = Number(res.headers.get('Retry-After'));
              }
              err.retryAfterSeconds = rememberVoteWait(reportId, action, waitSeconds, fieldError.message, fieldError.field);
            }
            throw err;
          }
          return body;
        });
      });
    }
  }

  // createVoteBlock builds the DOM for one report's controls: two buttons
  // (Confirm first, Dispute second, the Copywriting Contract's reading
  // order) inside a .vote-controls container, plus a hidden .vote-error
  // paragraph. It applies no state of its own — updateVoteBlock is always
  // called immediately after it. Built entirely with document.createElement,
  // never a markup string (T-01-03). It returns the controls, the note and
  // the error, and the consumers append them in that order. The resolve
  // option set to false builds the block without Mark resolved and its
  // confirmation (the Low data view); any other value builds them.
  function createVoteBlock(reportId, options) {
    var seq = ++voteBlockSeq;
    var withResolve = !(options && options.resolve === false);
    var controls = document.createElement('div');
    controls.className = 'vote-controls';
    controls.dataset.reportId = String(reportId);

    var confirmBtn = createVoteButton('vote-btn vote-btn--confirm', 'confirm', 'vote-btn__icon--check', 'Confirm');
    confirmBtn.setAttribute('aria-pressed', 'false');

    var disputeBtn = createVoteButton('vote-btn vote-btn--dispute', 'dispute', 'vote-btn__icon--ban', 'Dispute');
    disputeBtn.setAttribute('aria-pressed', 'false');

    controls.appendChild(confirmBtn);
    controls.appendChild(disputeBtn);

    if (withResolve) {
      // Mark resolved (Task 1, 02-07, D-13/D-15): the third button, appended
      // after Dispute per the Copywriting Contract's reading order. It
      // renders for the reporter and the non-reporter alike; the server's
      // response (via resolutionOutcomeMessage) is what distinguishes D-13's
      // instant path from D-14's threshold-gated one, never the markup.
      var resolveBtn = createVoteButton('vote-btn vote-btn--resolve', 'resolve', 'vote-btn__icon--shield-check', RESOLVE_BUTTON_LABEL);
      controls.appendChild(resolveBtn);

      // The inline destructive confirmation (D-15). Mounted INSIDE
      // .vote-controls, one level deeper than 02-UI-SPEC.md's sketch, which
      // drew it as a sibling with no data-action attributes, so its two
      // buttons are reached by the one delegated listener below rather than
      // needing a second one. Same markup, classes and copy either way; only
      // the nesting differs. Recorded in the SUMMARY as a deliberate
      // divergence.
      //
      // The confirmation is a group named by its question, so a screen reader
      // reads the question before either answer (A16, A63). Cancel comes
      // first in the DOM and takes focus when the group opens, so the safe
      // choice is the first one reached. The question id comes from the block
      // counter, never from the report id.
      var confirmBlock = document.createElement('div');
      confirmBlock.className = 'resolve-confirm';
      confirmBlock.hidden = true;
      var questionId = 'pinalert-resolve-q-' + seq;
      confirmBlock.setAttribute('role', 'group');
      confirmBlock.setAttribute('aria-labelledby', questionId);

      var confirmHeading = document.createElement('p');
      confirmHeading.id = questionId;
      Pinalert.setText(confirmHeading, RESOLVE_CONFIRM_HEADING);
      confirmBlock.appendChild(confirmHeading);

      // The line under the question that tells a reader who has no undo left
      // that this mark cannot be taken back (C53). Hidden until updateVoteBlock
      // finds the server's undos_left at zero. A plain paragraph in the group.
      var confirmLimit = document.createElement('p');
      confirmLimit.className = 'resolve-confirm__limit';
      confirmLimit.hidden = true;
      Pinalert.setText(confirmLimit, TAKE_BACK_LIMIT_CONFIRM_LINE);
      confirmBlock.appendChild(confirmLimit);

      var cancelBtn = document.createElement('button');
      cancelBtn.type = 'button';
      cancelBtn.className = 'vote-btn vote-btn--cancel-resolve';
      cancelBtn.dataset.action = 'cancel-resolve';
      Pinalert.setText(cancelBtn, RESOLVE_CONFIRM_CANCEL_LABEL);
      confirmBlock.appendChild(cancelBtn);

      var affirmBtn = document.createElement('button');
      affirmBtn.type = 'button';
      affirmBtn.className = 'vote-btn vote-btn--confirm-resolve';
      affirmBtn.dataset.action = 'confirm-resolve';
      Pinalert.setText(affirmBtn, RESOLVE_CONFIRM_AFFIRM_LABEL);
      confirmBlock.appendChild(affirmBtn);

      controls.appendChild(confirmBlock);
    }

    // The closed note and the take back block (03.1 D-22). Both live inside
    // the controls container, so a surface that already mounts the controls
    // shows them with no change of its own. They ship hidden and empty;
    // updateVoteBlock fills and reveals them from the report.
    var closedNote = document.createElement('p');
    closedNote.className = 'vote-note vote-closed-note';
    closedNote.hidden = true;
    controls.appendChild(closedNote);

    var takeBack = createTakeBackBlock(reportId);
    controls.appendChild(takeBack.el);

    var error = document.createElement('p');
    error.className = 'vote-error';
    error.setAttribute('role', 'alert');
    error.hidden = true;

    // The slot for the too far sentence. It ships hidden and empty, and is
    // a sibling of the controls, never a child of them.
    var note = document.createElement('p');
    note.className = 'vote-note';
    note.id = 'pinalert-vote-note-' + seq;
    note.hidden = true;

    var block = { controls: controls, note: note, error: error };
    block.confirmLimit = confirmLimit;
    block.closedNote = closedNote;
    block.takeBack = takeBack;

    // One listener on the container, not one per button. Delegation at
    // this level is what lets a future third button be appended with no
    // new wiring.
    controls.addEventListener('click', function (evt) {
      evt.stopPropagation();
      var btn = evt.target.closest('.vote-btn');
      if (!btn) {
        return;
      }
      onControlsClick(block, btn);
    });

    // This listener exists solely to stop the feed row's own keydown
    // handler from also firing on Enter/Space — the browser already
    // synthesises a click from a keyboard activation of a <button>, so
    // casting a vote here too would double-submit.
    controls.addEventListener('keydown', function (evt) {
      if (evt.target.closest('.vote-btn') &&
          (evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar')) {
        evt.stopPropagation();
      }
    });

    return block;
  }

  // createTakeBackBlock builds the take back part of the opened report (03.1
  // D-22): a statement, the Undo button, the poster's Reopen button and the
  // undos left line. It is built once, hidden, and updateVoteBlock decides
  // which parts show. The poster's Reopen is the button of the one Reopen
  // builder (createReopenControl), mounted here without that builder's own
  // controls wrapper and error line, so there is no second .vote-controls
  // nested in the first and the one delegated listener of the block reaches
  // it. Every string goes through Pinalert.setText.
  function createTakeBackBlock(reportId) {
    var el = document.createElement('div');
    el.className = 'vote-takeback';
    el.hidden = true;

    var statement = document.createElement('p');
    statement.className = 'vote-note vote-takeback__statement';

    var undoBtn = createVoteButton('vote-btn vote-btn--takeback', 'unresolve', 'vote-btn__icon--rotate-ccw', TAKE_BACK_BUTTON_LABEL);
    undoBtn.setAttribute('aria-label', TAKE_BACK_BUTTON_NAME);
    undoBtn.hidden = true;

    var reopenBtn = createReopenControl(reportId).button;
    reopenBtn.hidden = true;

    var count = document.createElement('p');
    count.className = 'vote-note vote-takeback__count';

    el.appendChild(statement);
    el.appendChild(undoBtn);
    el.appendChild(reopenBtn);
    el.appendChild(count);
    return { el: el, statement: statement, undoBtn: undoBtn, reopenBtn: reopenBtn, count: count };
  }

  // isReportFaded says whether a report has run out its time, on the server's
  // clock (the same comparison formatFade makes). A report in the past view is
  // faded by definition; a report with no readable expires_at is never called
  // faded here, so a missing field cannot hide a control by itself.
  function isReportFaded(report) {
    if (!report) {
      return false;
    }
    if (report.view === 'past') {
      return true;
    }
    var expires = new Date(report.expires_at).getTime();
    if (!isFinite(expires)) {
      return false;
    }
    return expires <= Pinalert.serverNow();
  }

  // takeBackState reads only the report's own fields and decides which take
  // back control, if any, the viewer sees. It never computes a limit: the
  // server sends undos_left and refuses a third undo regardless (D-22, D-28).
  //   undo          : the viewer's mark closed the report, undos left
  //   undo-waiting  : the viewer's mark is waiting for agreement, undos left
  //   used          : the viewer's mark stands and no undo is left
  //   reopen        : the poster, report closed by other people, undos left
  //   reopen-used   : the same with none left
  //   none          : nothing, and always nothing for a faded report
  function takeBackState(report) {
    if (!report || isReportFaded(report) || typeof report.undos_left !== 'number') {
      return 'none';
    }
    var left = report.undos_left;
    var finished = report.view === 'resolved' || report.view === 'past';
    if (report.your_resolution === 'resolve') {
      if (left > 0) {
        return finished ? 'undo' : 'undo-waiting';
      }
      return 'used';
    }
    if (report.is_own_report === true && report.view === 'resolved') {
      return left > 0 ? 'reopen' : 'reopen-used';
    }
    return 'none';
  }

  // undosLeftLine is C51 for a count, "1 undo left" in the singular.
  function undosLeftLine(left) {
    if (left === 1) {
      return '1 undo left on this report.';
    }
    return String(left) + ' undos left on this report.';
  }

  // applyTakeBackBlock shows the finished report rule and the take back block
  // for one report. It reads the report only, never the busy state, and it
  // never assigns the native disabled property (setBlockBusy owns it). Nothing
  // is disabled instead of hidden: a control that does not apply is hidden, as
  // the own report rule removes its buttons.
  function applyTakeBackBlock(block, report) {
    var state = takeBackState(report);
    var finished = report.view === 'resolved' || report.view === 'past';
    var faded = isReportFaded(report);
    var confirmBtn = block.controls.querySelector('.vote-btn--confirm');
    var disputeBtn = block.controls.querySelector('.vote-btn--dispute');
    var resolveBtn = block.controls.querySelector('.vote-btn--resolve');
    var confirmBlock = block.controls.querySelector('.resolve-confirm');
    var hideResolve = finished || state === 'undo-waiting' || state === 'used';
    var confirmOpen = !!confirmBlock && !confirmBlock.hidden;
    var focusWasInConfirm = false;

    // A confirmation left open by a report that has just closed or whose
    // mark has just been recorded closes by itself. Focus that was inside it
    // moves to the take back button below when that can take it, so it never
    // falls to the page body.
    if (confirmOpen && hideResolve) {
      focusWasInConfirm = confirmBlock.contains(document.activeElement);
      confirmBlock.hidden = true;
      confirmOpen = false;
    }

    // While the confirmation is open the three primary buttons are hidden by
    // it, and a poll must not unhide them (a render never touches the open
    // confirmation's own state).
    if (!confirmOpen) {
      if (confirmBtn) {
        confirmBtn.hidden = finished;
      }
      if (disputeBtn) {
        disputeBtn.hidden = finished;
      }
      if (resolveBtn) {
        resolveBtn.hidden = hideResolve;
      }
    }

    // The line under the confirmation question: shown when the viewer has no
    // undo left on this report, from the server's own count.
    if (block.confirmLimit) {
      block.confirmLimit.hidden = !(typeof report.undos_left === 'number' && report.undos_left === 0);
    }

    // The closed note replaces the vote buttons of a finished report.
    if (finished) {
      Pinalert.setText(block.closedNote, faded ? CLOSED_NOTE_EXPIRED : CLOSED_NOTE_RESOLVED);
      block.closedNote.hidden = false;
    } else {
      Pinalert.setText(block.closedNote, '');
      block.closedNote.hidden = true;
    }

    var tb = block.takeBack;
    if (state === 'none') {
      tb.el.hidden = true;
      tb.undoBtn.hidden = true;
      tb.reopenBtn.hidden = true;
      Pinalert.setText(tb.statement, '');
      Pinalert.setText(tb.count, '');
      return;
    }

    var statement = TAKE_BACK_STATEMENT;
    if (state === 'undo-waiting' || (state === 'used' && !finished)) {
      statement = TAKE_BACK_STATEMENT_WAITING;
    } else if (state === 'reopen' || state === 'reopen-used') {
      statement = TAKE_BACK_STATEMENT_POSTER;
    }
    Pinalert.setText(tb.statement, statement);
    tb.undoBtn.hidden = !(state === 'undo' || state === 'undo-waiting');
    tb.reopenBtn.hidden = state !== 'reopen';
    if (state === 'used' || state === 'reopen-used') {
      Pinalert.setText(tb.count, TAKE_BACK_USED_MESSAGE);
    } else {
      Pinalert.setText(tb.count, undosLeftLine(report.undos_left));
    }
    tb.el.hidden = false;
    if (focusWasInConfirm && canTakeFocus(tb.undoBtn)) {
      tb.undoBtn.focus();
    }
  }

  // applyOwnReportRule removes the reporter's own Confirm/Dispute buttons
  // from the DOM rather than leaving them present but non-interactive
  // (D-03) — a UI courtesy layered on the server's enforced refusal
  // (02-03b), never a replacement for it. Runs on every call, including
  // every 30-second poll, so it must be idempotent: it queries the buttons
  // fresh each time and tolerates their already being gone.
  function applyOwnReportRule(block, report) {
    block.isOwnReport = !!report.is_own_report;
    block.lastReport = report;
    if (report.is_own_report) {
      var confirmBtn = block.controls.querySelector('.vote-btn--confirm');
      var disputeBtn = block.controls.querySelector('.vote-btn--dispute');
      if (confirmBtn) {
        confirmBtn.remove();
      }
      if (disputeBtn) {
        disputeBtn.remove();
      }
    }

    var hasAnyButton = block.controls.querySelector('.vote-btn') !== null;
    block.controls.hidden = !hasAnyButton;
  }

  // updateVoteBlock writes aria-pressed on both buttons from
  // report.your_vote — rewriting both on every call, since writing only
  // the newly active one leaves a stale pressed state on the other after
  // a vote change (D-02) — and applies the own-report rule above. It never
  // reads or writes the busy state; see the next function's own doc
  // comment for why that separation matters.
  function updateVoteBlock(block, report) {
    // The saved-but-not-refreshed sentence ends with the first render that
    // follows a good fetch. A render of stale rows, such as the map redrawing
    // during a failed poll, leaves it in place.
    if (block.refreshNotice && Pinalert.state.status === 'ok') {
      clearVoteError(block);
    }
    applyOwnReportRule(block, report);

    var confirmBtn = block.controls.querySelector('.vote-btn--confirm');
    var disputeBtn = block.controls.querySelector('.vote-btn--dispute');
    if (confirmBtn) {
      confirmBtn.setAttribute('aria-pressed', report.your_vote === 'confirm' ? 'true' : 'false');
    }
    if (disputeBtn) {
      disputeBtn.setAttribute('aria-pressed', report.your_vote === 'dispute' ? 'true' : 'false');
    }
    applyTakeBackBlock(block, report);
    var hintWasShowing = !!block.note && !block.note.hidden;
    // A finished report offers no vote, so it can be neither too far to vote
    // on nor marked so: the too far sentence is cleared under "voting is off".
    if (report.view === 'resolved' || report.view === 'past') {
      clearDistanceHint(block);
    } else {
      applyDistanceHint(block, report);
    }
    clearRefusalWithHint(block, hintWasShowing);
  }

  // setBlockBusy is the sole owner of the busy state on a vote block: it
  // toggles the interactive attribute that takes every button in the
  // block out of the tab order and pointer-event path while a vote is in
  // flight, plus aria-busy on the container itself. No label change: the
  // Copywriting Contract supplies no in-flight string for Confirm/Dispute
  // and states outright that their labels never change, so the busy
  // treatment is carried entirely by this attribute and aria-busy.
  function setBlockBusy(block, busy) {
    var btns = block.controls.querySelectorAll('.vote-btn');
    for (var i = 0; i < btns.length; i++) {
      btns[i].disabled = busy;
    }
    if (busy) {
      block.controls.setAttribute('aria-busy', 'true');
    } else {
      block.controls.removeAttribute('aria-busy');
    }
  }

  // Reads the cached voter fix and nothing else, for the too far hint. It
  // never asks the browser for a position, never queries the permission and
  // never writes the cache: the silent refresh of the vote path keeps the
  // cache current. A browser that throws on storage access gives no fix.
  function peekVoterLocation() {
    try {
      return parseVoterLocation(sessionStorage.getItem(VOTER_LOCATION_CACHE_KEY));
    } catch (e) {
      return null;
    }
  }

  // The vote radius in kilometres, read straight from the page shell's
  // attribute. A missing, empty, unreadable or zero value gives null, which
  // means no hint. There is no default number here on purpose (D-22).
  function readVoteRadiusKm() {
    var shell = document.getElementById('app-shell');
    if (!shell) {
      return null;
    }
    var raw = shell.getAttribute('data-vote-radius-km');
    if (raw === null || raw === '') {
      return null;
    }
    var radius = Number(raw);
    if (isFinite(radius) && radius > 0) {
      return radius;
    }
    return null;
  }

  // One great circle distance in kilometres, the same earth radius as the
  // server's. It is the only geometry in the browser and it grants nothing.
  function greatCircleKm(lat1, lon1, lat2, lon2) {
    var toRad = Math.PI / 180;
    var dLat = (lat2 - lat1) * toRad;
    var dLon = (lon2 - lon1) * toRad;
    var a = Math.sin(dLat / 2) * Math.sin(dLat / 2) +
      Math.cos(lat1 * toRad) * Math.cos(lat2 * toRad) *
      Math.sin(dLon / 2) * Math.sin(dLon / 2);
    a = Math.min(1, Math.max(0, a));
    return 2 * 6371 * Math.asin(Math.sqrt(a));
  }

  // Removes the too far marks from the three hint buttons and empties and
  // hides the note. A node that an aria reference points at is still read
  // out, so the note is emptied as well as hidden.
  function clearDistanceHint(block) {
    HINT_BUTTON_SELECTORS.forEach(function (selector) {
      var btn = block.controls.querySelector(selector);
      if (btn) {
        btn.removeAttribute('aria-disabled');
        btn.removeAttribute('data-too-far');
        btn.removeAttribute('aria-describedby');
      }
    });
    if (block.note) {
      Pinalert.setText(block.note, '');
      block.note.hidden = true;
    }
  }

  // Marks Confirm, Dispute and Mark resolved too far, with the reason as
  // visible text in the note, only when the radius is known, the cached fix
  // is younger than the hint bound and the report lies strictly farther than
  // the radius. Every other case clears the marks, so the hint fails open and
  // the first tap asks for location as before. The reporter's own report is
  // never marked. Marked buttons stay focusable. The server refuses far votes
  // on its own, so this grants and decides nothing (D-01, D-22). The optional
  // third argument is a fix the caller has just taken (latitude, longitude and
  // capture time): the mark is then computed from it and not from the session
  // cache, because a coarse fix is never cached (WR-07) and the tap that took
  // it must still be judged by it (WR-03).
  function applyDistanceHint(block, report, justTaken) {
    if (report.is_own_report || !block.note) {
      clearDistanceHint(block);
      return;
    }
    var radiusKm = readVoteRadiusKm();
    var fix = justTaken || peekVoterLocation();
    if (radiusKm === null || fix === null || !isFreshVoterLocation(fix, VOTER_LOCATION_HINT_MS)) {
      clearDistanceHint(block);
      return;
    }
    var distanceKm = greatCircleKm(fix.latitude, fix.longitude, report.latitude, report.longitude);
    if (!isFinite(distanceKm)) {
      clearDistanceHint(block);
      return;
    }
    if (distanceKm > radiusKm) {
      HINT_BUTTON_SELECTORS.forEach(function (selector) {
        var btn = block.controls.querySelector(selector);
        if (btn) {
          btn.setAttribute('aria-disabled', 'true');
          btn.setAttribute('data-too-far', 'true');
          btn.setAttribute('aria-describedby', block.note.id);
        }
      });
      Pinalert.setText(block.note, TOO_FAR_LEAD + String(radiusKm) + TOO_FAR_TAIL);
      block.note.hidden = false;
    } else {
      clearDistanceHint(block);
    }
  }

  // farFromCachedFix says whether a list row shows the Too far to vote fact
  // (03.1 D-14). It keeps every guard of applyDistanceHint so the row and the
  // opened report agree: the reporter's own report, an unknown radius, no
  // cached fix, a fix older than the hint bound and an unreadable distance all
  // answer false. It reads the cached fix only. It never asks the browser for
  // a position, never queries the permission and never writes the cache, so a
  // row cannot cause a location prompt. The server still refuses far votes.
  function farFromCachedFix(report) {
    if (!report || report.is_own_report) {
      return false;
    }
    var radiusKm = readVoteRadiusKm();
    var fix = peekVoterLocation();
    if (radiusKm === null || fix === null || !isFreshVoterLocation(fix, VOTER_LOCATION_HINT_MS)) {
      return false;
    }
    var distanceKm = greatCircleKm(fix.latitude, fix.longitude, report.latitude, report.longitude);
    return isFinite(distanceKm) && distanceKm > radiusKm;
  }

  // showVoteError writes message into the block's error paragraph and
  // reveals it. Goes through Pinalert.setText even though the message may
  // be server-authored (T-01-03: the discipline is about the sink, not
  // the author).
  function showVoteError(block, message) {
    Pinalert.setText(block.error, message);
    block.error.hidden = false;
  }

  function clearVoteError(block) {
    block.error.hidden = true;
    Pinalert.setText(block.error, '');
    block.refreshNotice = false;
    block.refusedTooFar = false;
  }

  // clearRefusalWithHint removes the earlier too far refusal line together
  // with the hint (A8, D-22). When the hint was showing before a render, is
  // hidden after it, and the error line holds the server's too far refusal,
  // the line goes. Every other message sharing the line is left alone.
  function clearRefusalWithHint(block, hintWasShowing) {
    if (hintWasShowing && block.note && block.note.hidden && block.refusedTooFar) {
      clearVoteError(block);
    }
  }

  // showSavedButNotRefreshed is what a row shows when the server answered 200
  // to a vote and the refetch after it then failed. The 200 is the server's own
  // word that the vote was recorded, so for Confirm and Dispute the row takes
  // the pressed state from the action that was just recorded, in the store as
  // well so that a later render of the stale data does not undo it. Counters are
  // not touched: they stay as the server last sent them, and the plain sentence
  // beside the buttons says they may be out of date. The sentence goes away on
  // the next render that follows a good fetch (updateVoteBlock).
  function showSavedButNotRefreshed(block, action) {
    if (action === 'confirm' || action === 'dispute') {
      var reports = Pinalert.state.reports || [];
      for (var i = 0; i < reports.length; i++) {
        if (String(reports[i].id) === String(block.controls.dataset.reportId)) {
          reports[i].your_vote = action;
          updateVoteBlock(block, reports[i]);
          break;
        }
      }
    }
    showVoteError(block, VOTE_SAVED_REFRESH_FAILED_MESSAGE);
    block.refreshNotice = true;
  }

  // openResolveConfirm implements D-15's inline destructive confirmation:
  // hides the three primary buttons via the hidden attribute, reveals
  // .resolve-confirm, and moves focus to Cancel, the safe answer (A16).
  // Never touches `disabled` — setBlockBusy remains its sole owner (see its
  // own doc comment), which is what keeps a background poll landing
  // mid-flight from re-enabling a button whose vote is in the air.
  function openResolveConfirm(block) {
    var confirmBlock = block.controls.querySelector('.resolve-confirm');
    if (!confirmBlock) {
      return;
    }
    var primaryBtns = block.controls.querySelectorAll(
      '.vote-btn--confirm, .vote-btn--dispute, .vote-btn--resolve'
    );
    for (var i = 0; i < primaryBtns.length; i++) {
      primaryBtns[i].hidden = true;
    }
    confirmBlock.hidden = false;
    var cancelBtn = block.controls.querySelector('.vote-btn--cancel-resolve');
    if (cancelBtn) {
      cancelBtn.focus();
    }
  }

  // closeResolveConfirm reverses openResolveConfirm. Same `disabled`
  // discipline as above — this function never touches it either. It also
  // never moves focus: each caller decides where focus goes next (A52).
  function closeResolveConfirm(block) {
    var primaryBtns = block.controls.querySelectorAll(
      '.vote-btn--confirm, .vote-btn--dispute, .vote-btn--resolve'
    );
    for (var i = 0; i < primaryBtns.length; i++) {
      primaryBtns[i].hidden = false;
    }
    var confirmBlock = block.controls.querySelector('.resolve-confirm');
    if (!confirmBlock) {
      return;
    }
    confirmBlock.hidden = true;
  }

  // setVoteButtonLabel writes the visible word of a vote button into its
  // label span. It never writes on the button itself, because the text
  // setter replaces every child and would delete the icon span.
  function setVoteButtonLabel(button, text) {
    if (!button) {
      return;
    }
    var label = button.querySelector('.vote-btn__label');
    if (label) {
      Pinalert.setText(label, text);
    }
  }

  // createVoteButton builds one icon plus word vote button: an aria-hidden
  // icon span, then a label span holding the word, so a screen reader hears
  // the word alone. The icon modifier class is a literal passed by the
  // builders (check, ban, shield-check or rotate-ccw), never report data.
  // The icon span carries no shared glyph base class: the vote button icon
  // rule in the stylesheet repeats its own mask base.
  function createVoteButton(className, action, iconClass, labelText) {
    var button = document.createElement('button');
    button.type = 'button';
    button.className = className;
    button.dataset.action = action;

    var icon = document.createElement('span');
    icon.className = 'vote-btn__icon ' + iconClass;
    icon.setAttribute('aria-hidden', 'true');

    var label = document.createElement('span');
    label.className = 'vote-btn__label';

    button.appendChild(icon);
    button.appendChild(label);
    setVoteButtonLabel(button, labelText);
    return button;
  }

  // createReopenControl is the one Reopen builder the Activity page uses
  // (D-20, A64). It returns the controls, the button and the error line, with
  // no note, because Reopen never shows the too far state (A66), and with no
  // listener, because the Activity page attaches its own single delegated
  // one. setBlockBusy and showVoteError work on the returned object
  // unchanged.
  function createReopenControl(reportId) {
    var controls = document.createElement('div');
    controls.className = 'vote-controls';
    controls.dataset.reportId = String(reportId);

    var reopenBtn = createVoteButton('vote-btn vote-btn--reopen', 'reopen', 'vote-btn__icon--rotate-ccw', REOPEN_BUTTON_LABEL);
    controls.appendChild(reopenBtn);

    var error = document.createElement('p');
    error.className = 'vote-error';
    error.setAttribute('role', 'alert');
    error.hidden = true;

    return { controls: controls, button: reopenBtn, error: error };
  }

  // isDisplayed is true only for an element that is attached to the page and
  // has at least one layout box, so it is false for anything inside a pane
  // that is not shown.
  function isDisplayed(el) {
    return !!(el && el.isConnected && el.getClientRects().length > 0);
  }

  // canTakeFocus is true when the element is shown, not hidden and not
  // natively disabled. It only reads those properties. Focus is never
  // relied on for an element that cannot take it.
  function canTakeFocus(el) {
    return isDisplayed(el) && !el.hidden && !el.disabled;
  }

  // focusIsLost is true when focus sits on nothing useful: no active
  // element, the page body, or anywhere inside scope (the controls or the
  // row that is about to go away or was just closed).
  function focusIsLost(scope) {
    var active = document.activeElement;
    return !active || active === document.body || (!!scope && scope.contains(active));
  }

  // focusResolveButton puts focus on this block's Mark resolved when it can
  // take it. Used by the Cancel path and by the cancel hook for Escape.
  function focusResolveButton(block) {
    var resolveBtn = block.controls.querySelector('.vote-btn--resolve');
    if (canTakeFocus(resolveBtn)) {
      resolveBtn.focus();
    }
  }

  // captureVoteOrigin records where a vote started: the report id, whether
  // the controls sit in the map popup or in a feed row, and the controls
  // node itself. A map redraw can replace that node while the vote is in
  // flight, so later steps look the live node up again from this record.
  function captureVoteOrigin(block) {
    var inPopup = block.controls.closest('.leaflet-popup') !== null;
    return {
      reportId: block.controls.dataset.reportId,
      surface: inPopup ? 'popup' : 'row',
      controls: block.controls
    };
  }

  // isInOpenPopup says whether the controls sit in a map popup that is still
  // open. map.js sets data-popup-open on the popup element when it opens and
  // clears it synchronously when it closes, which the render does for a
  // retracted report before the refetch resolves. Leaflet then keeps the
  // closing node attached for a 200 ms fade, so isConnected alone would call
  // that dying popup live. The mark is the truthful signal at that moment, and
  // reading it needs no timer, animation event or style.
  function isInOpenPopup(controls) {
    var popup = controls.closest('.leaflet-popup');
    return popup !== null && popup.getAttribute('data-popup-open') === 'true';
  }

  // findLiveControls returns the controls node to focus into: the original
  // one while it is still attached, else the first attached vote controls
  // with the same report id on the same surface, else null. A popup that
  // map.js has marked closed is not live even while its node stays attached
  // for the fade. Ids are compared as strings and are never built into a
  // selector.
  function findLiveControls(origin) {
    var wantPopup = origin.surface === 'popup';
    if (origin.controls.isConnected && (!wantPopup || isInOpenPopup(origin.controls))) {
      return origin.controls;
    }
    var all = document.querySelectorAll('.vote-controls');
    for (var i = 0; i < all.length; i++) {
      var inPopup = all[i].closest('.leaflet-popup') !== null;
      if (all[i].dataset.reportId === origin.reportId && inPopup === wantPopup &&
          (!wantPopup || isInOpenPopup(all[i]))) {
        return all[i];
      }
    }
    return null;
  }

  // findFeedRow returns the feed row for a report id, or null. String
  // comparison only, no selector built from the id.
  function findFeedRow(reportId) {
    var rows = document.querySelectorAll('.report-row');
    for (var i = 0; i < rows.length; i++) {
      if (rows[i].dataset.reportId === reportId) {
        return rows[i];
      }
    }
    return null;
  }

  // captureRemovalFocus records the neighbours of the report's list row and
  // the map container before a resolve can remove the report. For a vote
  // from the popup, the list row is the row of the same report, found by id.
  function captureRemovalFocus(origin) {
    var row = null;
    if (origin.surface === 'row' && origin.controls.isConnected) {
      row = origin.controls.closest('.report-row');
    } else {
      row = findFeedRow(origin.reportId);
    }
    var map = origin.controls.closest('.leaflet-container') || document.querySelector('.leaflet-container');
    return {
      row: row,
      next: row ? row.nextElementSibling : null,
      prev: row ? row.previousElementSibling : null,
      map: map
    };
  }

  // focusAfterRetract moves focus when a resolve took the report out of the
  // feed and focus was lost with it (A52). From the popup it goes to the map
  // container. From the list it goes to the row that took the place, else
  // the previous row, else the view toggle, else the empty state, which is
  // made focusable for the purpose. Nothing is announced.
  function focusAfterRetract(origin, removal) {
    if (!focusIsLost(origin.controls) && !focusIsLost(removal.row)) {
      return;
    }
    if (origin.surface === 'popup' && canTakeFocus(removal.map)) {
      removal.map.focus();
      return;
    }
    var neighbours = [removal.next, removal.prev];
    for (var i = 0; i < neighbours.length; i++) {
      var candidate = neighbours[i];
      if (candidate && candidate.classList.contains('report-row') && canTakeFocus(candidate)) {
        candidate.focus();
        return;
      }
    }
    // The strip link of the tab that is showing: the strip always exists on
    // a report page, so focus lands on a real control and never on the body
    // (03.1 UI-SPEC, ending on the tab strip). It is found by its selector.
    var tabLink = currentTabLink();
    if (canTakeFocus(tabLink)) {
      tabLink.focus();
      return;
    }
    var toggle = document.getElementById('view-toggle');
    if (canTakeFocus(toggle)) {
      toggle.focus();
      return;
    }
    var emptyStates = [
      document.getElementById('feed-empty'),
      document.getElementById('feed-empty-history'),
      document.getElementById('disputed-empty'),
      document.getElementById('past-empty'),
      document.getElementById('filter-empty')
    ];
    for (var j = 0; j < emptyStates.length; j++) {
      if (isDisplayed(emptyStates[j])) {
        emptyStates[j].setAttribute('tabindex', '-1');
        emptyStates[j].focus();
        return;
      }
    }
  }

  // focusAfterVote runs once a vote is released, after the refetch has
  // redrawn both surfaces. If the controls are still on the page, the button
  // that matches the action takes focus back, but only when focus fell to
  // the body or is still on a hidden button. If the controls are gone, the
  // report left the feed, which is read here from the live page and not
  // from the response, and the removal steps apply when the vote was a
  // resolve.
  function focusAfterVote(origin, action, removal) {
    var selector = RELEASE_FOCUS_SELECTORS[action];
    if (!selector) {
      return;
    }
    var live = findLiveControls(origin);
    if (live) {
      var target = live.querySelector(selector);
      // A resolve that is still waiting for agreement replaces Mark resolved
      // with the take back block, so the Undo button takes the focus instead.
      if (!canTakeFocus(target) && action === 'confirm-resolve') {
        target = live.querySelector('.vote-btn--takeback');
      }
      if (canTakeFocus(target) && focusIsLost(origin.controls)) {
        target.focus();
      }
      return;
    }
    if (removal) {
      focusAfterRetract(origin, removal);
    }
  }

  // cancelResolveConfirm closes an open confirmation inside container by the
  // Cancel path, sends nothing, puts focus on that block's Mark resolved and
  // returns true. It returns false when no confirmation is open, and when
  // the block is busy: Cancel is natively disabled while a resolve is in
  // flight, so that path does not exist then. Plan 03-13's one key listener
  // calls this hook.
  function cancelResolveConfirm(container) {
    if (!container || typeof container.querySelector !== 'function') {
      return false;
    }
    var open = container.querySelector('.resolve-confirm:not([hidden])');
    if (!open) {
      return false;
    }
    var controls = open.closest('.vote-controls');
    if (!controls || controls.getAttribute('aria-busy') === 'true') {
      return false;
    }
    var block = { controls: controls };
    closeResolveConfirm(block);
    focusResolveButton(block);
    return true;
  }

  // resolutionOutcomeMessage (Task 1, 02-07) is the only place in the
  // client that DECIDES whether a resolution vote took effect (D-13,
  // D-16 amended): it reads result's visibility field and returns one of
  // four fixed Copywriting Contract strings, branching on nothing else —
  // no identity check, no threshold arithmetic, no is_own_report read.
  // The resolve path below — which genuinely has two possible outcomes —
  // is this function's caller. The reopen branches are retained for
  // API-consistency with castVote's generic outcome handling, per
  // 02-UI-SPEC.md's amended Reopen section; 02-07 Task 3's Activity page
  // does NOT call this function, because there a reopen has exactly one
  // outcome (D-16 amended 2026-09-15).
  function resolutionOutcomeMessage(action, result) {
    var retracted = !!(result && result.visibility === RETRACTED_VISIBILITY_SLUG);
    if (action === 'resolve') {
      return retracted ? RESOLVE_SUCCEEDED_TOAST : RESOLVE_PENDING_TOAST;
    }
    return retracted ? REOPEN_PENDING_TOAST : REOPEN_SUCCEEDED_TOAST;
  }

  // refreshHintThenRetry is what a tap on a button marked too far does
  // (WR-03). The mark comes from a cached fix that may be minutes old, and
  // nothing else rewrites that cache while the mark blocks every tap, so a
  // voter who has walked closer would be locked out. The tap therefore asks
  // for one fresh fix (silent: it never prompts), recomputes the mark from the
  // coordinates that fix resolved with and goes on with the original tap only
  // if the button is no longer marked. The tap sends those same coordinates and
  // takes no second fix. The fix is used even when it was too coarse to be
  // cached (WR-07), so a voter indoors is not locked out by an older, far fix
  // that is still in the cache. If no fix can be taken, or the voter is still
  // too far, the mark stays and nothing is sent. The server remains the only
  // judge of every vote.
  function refreshHintThenRetry(block, button) {
    if (block.hintRefreshing) {
      return;
    }
    block.hintRefreshing = true;
    getVoterLocation({ fresh: true }).then(function (coords) {
      block.hintRefreshing = false;
      if (!block.lastReport) {
        return;
      }
      var taken = { latitude: coords.latitude, longitude: coords.longitude, at: Date.now() };
      var hintWasShowing = !!block.note && !block.note.hidden;
      applyDistanceHint(block, block.lastReport, taken);
      clearRefusalWithHint(block, hintWasShowing);
      if (button.getAttribute('aria-disabled') !== 'true') {
        onControlsClick(block, button, taken);
      }
    }, function () {
      block.hintRefreshing = false;
    });
  }

  // onControlsClick is the only place D-04 is implemented: nothing about
  // the block's rendering changes until the server has answered. The
  // resolve/cancel-resolve branches (Task 1, 02-07) short-circuit before
  // any of that: opening or closing the inline confirmation posts nothing
  // and reads no location. The optional third argument is the fix that
  // refreshHintThenRetry just took for this tap, handed on to castVote.
  function onControlsClick(block, button, taken) {
    // A button marked too far sends no vote. The tap refreshes the fix and
    // the mark first, and the vote follows only if the mark is gone (WR-03).
    if (button.getAttribute('aria-disabled') === 'true') {
      refreshHintThenRetry(block, button);
      return;
    }
    var action = button.dataset.action;

    if (action === 'resolve') {
      openResolveConfirm(block);
      return;
    }
    if (action === 'cancel-resolve') {
      closeResolveConfirm(block);
      focusResolveButton(block);
      return;
    }
    // The take back and the poster's Reopen are the two buttons of the take
    // back block (03.1 D-22). They need no location and have their own busy
    // label, error line and focus steps, so they leave this flow here.
    if (action === 'unresolve' || action === 'reopen') {
      onTakeBackClick(block, button);
      return;
    }

    clearVoteError(block);
    setBlockBusy(block, true);

    var reportId = block.controls.dataset.reportId;
    var origin = captureVoteOrigin(block);
    var removal = null;
    // confirm-resolve is a UI-only dispatch key (never a member of
    // VOTE_ACTIONS) — its transport action is the ordinary 'resolve'
    // castVote already accepts, unchanged.
    var isResolveAffirm = action === 'confirm-resolve';
    var transportAction = isResolveAffirm ? 'resolve' : action;
    var affirmBtn = isResolveAffirm
      ? block.controls.querySelector('.vote-btn--confirm-resolve')
      : null;
    // The in-flight label swap targets the affirm button AFTER
    // setBlockBusy(block, true) above, never before, so the label and the
    // disabled attribute are never applied out of order.
    if (affirmBtn) {
      Pinalert.setText(affirmBtn, RESOLVE_BUTTON_LABEL_IN_FLIGHT);
    }

    castVote(reportId, transportAction, { ownReport: block.isOwnReport === true, location: taken }).then(function (result) {
      if (isResolveAffirm) {
        removal = captureRemovalFocus(origin);
        closeResolveConfirm(block);
        Pinalert.showToast(resolutionOutcomeMessage('resolve', result), resolveToastAction(reportId, result));
      }
      // Keep the block busy across the refetch: releasing it before the
      // re-render would show buttons that still carry the pre-vote state
      // for the duration of the round trip, exactly the optimistic-
      // looking window D-04 forbids. The re-render is what applies the
      // server's your_vote — this module never assigns it itself. D-12's
      // removal of a newly-retracted report from the feed/map happens
      // here too, via this same refetch, never a local DOM removal.
      return Pinalert.fetchReports({ afterWrite: true });
    }).then(function () {
      if (affirmBtn) {
        Pinalert.setText(affirmBtn, RESOLVE_CONFIRM_AFFIRM_LABEL);
      }
      setBlockBusy(block, false);
      // fetchReports never rejects. A failed refetch shows up as the store's
      // error status, and the row must then say the vote was saved (WR-08).
      if (Pinalert.state.status === 'error') {
        showSavedButNotRefreshed(block, action);
      }
      focusAfterVote(origin, action, removal);
    }).catch(function (err) {
      // Order is load-bearing here: clear busy BEFORE closing the
      // confirmation. Closing sets hidden on the two confirmation
      // buttons; clearing disabled afterwards would mean the restored
      // three-button row was re-enabled while the still-disabled
      // confirmation buttons were the ones being cleared, leaving a row
      // that looks normal and cannot be tapped.
      setBlockBusy(block, false);
      if (affirmBtn) {
        Pinalert.setText(affirmBtn, RESOLVE_CONFIRM_AFFIRM_LABEL);
      }
      if (isResolveAffirm) {
        closeResolveConfirm(block);
      }
      showVoteError(block, (err && err.fieldMessage) || GENERIC_FAILURE_MESSAGE);
      block.refusedTooFar = !!(err && err.field === 'location');
      focusAfterVote(origin, action, null);
    });
  }

  // currentTabLink is the strip link of the tab that is showing, found by its
  // selector, or null. The strip is feed.js's; this module only reads it.
  function currentTabLink() {
    return document.querySelector('.view-tabs .view-tab[aria-current="page"]');
  }

  // takeBackSuccessMessage is C56 when the take back reopened a report that
  // had closed, and C57 in every other case: a mark that was only waiting, or
  // a report that stays closed because other people also marked it. It reads
  // the server's answer and predicts nothing (D-04).
  function takeBackSuccessMessage(wasClosed, result) {
    var stillClosed = !!(result && result.visibility === RETRACTED_VISIBILITY_SLUG);
    return wasClosed && !stillClosed ? TAKE_BACK_SUCCEEDED_CLOSED_TOAST : TAKE_BACK_SUCCEEDED_WAITING_TOAST;
  }

  // takeBackErrorMessage picks the line under the block: C54 when the server
  // sent nothing usable, C55 for the expired refusal, and any other server
  // sentence exactly as sent.
  function takeBackErrorMessage(err) {
    var message = err && err.fieldMessage;
    if (!message || message === GENERIC_FAILURE_MESSAGE) {
      return TAKE_BACK_ERROR_MESSAGE;
    }
    if (message === SERVER_EXPIRED_REFUSAL) {
      return TAKE_BACK_FADED_MESSAGE;
    }
    return message;
  }

  // answeredByServer is true for an error the server sent (a 409 refusal, the
  // expired refusal, any other refused take back): postVote gives those a
  // field. A transport failure has none, and there is no newer state to read.
  function answeredByServer(err) {
    return !!err && typeof err.field === 'string' && err.field !== '';
  }

  // focusAfterTakeBack runs after a take back or a poster Reopen succeeded and
  // the refetch has redrawn. When focus sat on a control that is now hidden or
  // gone (the Undo button, the toast), it goes to the report's own row if the
  // list still holds it, else to the current tab link, so it never falls to
  // the page body (A52, A67). Focus the reader has already moved is left.
  function focusAfterTakeBack(origin) {
    if (!focusIsLost(origin.controls)) {
      return;
    }
    var row = findFeedRow(origin.reportId);
    if (row && canTakeFocus(row)) {
      row.focus();
      return;
    }
    var tabLink = currentTabLink();
    if (canTakeFocus(tabLink)) {
      tabLink.focus();
    }
  }

  // onTakeBackClick handles the Undo button and the poster's Reopen in the
  // opened report. Neither needs a location. The block is busy for the
  // request and the label names the action ("Undoing…" or "Reopening…"). The
  // block shows what the server answered after the refetch, never a guess.
  function onTakeBackClick(block, button) {
    var isReopen = button.dataset.action === 'reopen';
    var action = isReopen ? 'reopen' : 'unresolve';
    var restingLabel = isReopen ? REOPEN_BUTTON_LABEL : TAKE_BACK_BUTTON_LABEL;
    var origin = captureVoteOrigin(block);
    var wasClosed = takeBackState(block.lastReport) === 'undo';

    clearVoteError(block);
    setBlockBusy(block, true);
    setVoteButtonLabel(button, isReopen ? REOPEN_BUTTON_LABEL_IN_FLIGHT : TAKE_BACK_BUTTON_LABEL_IN_FLIGHT);

    castVote(origin.reportId, action, { ownReport: isReopen && block.isOwnReport === true }).then(function (result) {
      var message = isReopen ? REOPEN_SUCCEEDED_TOAST : takeBackSuccessMessage(wasClosed, result);
      return Pinalert.fetchReports({ afterWrite: true }).then(function () {
        setVoteButtonLabel(button, restingLabel);
        setBlockBusy(block, false);
        if (Pinalert.state.status === 'error') {
          showVoteError(block, VOTE_SAVED_REFRESH_FAILED_MESSAGE);
          block.refreshNotice = true;
        }
        Pinalert.showToast(message);
        focusAfterTakeBack(origin);
      });
    }, function (err) {
      // A refusal means this block is out of date: the toast Undo, another
      // device or another tab already took the mark back, or both undos are
      // spent. The list is read again before the block is released, as on
      // success, so the render replaces the stale Undo; the Past tab is never
      // polled, so nothing else would (WR-05). The refusal line and the
      // release come first, so the reader is not kept waiting for a list
      // round trip; the render keeps the line. Focus stays on the button
      // while it is shown; when the render hides it, focus follows the take
      // back's own route (row, then tab link).
      setVoteButtonLabel(button, restingLabel);
      setBlockBusy(block, false);
      showVoteError(block, takeBackErrorMessage(err));
      if (canTakeFocus(button)) {
        button.focus();
      }
      var refresh = answeredByServer(err) ? Pinalert.fetchReports({ afterWrite: true }) : Promise.resolve();
      return refresh.then(function () {
        if (!canTakeFocus(button)) {
          focusAfterTakeBack(origin);
        }
      });
    });
  }

  // takeBackFromToast is the short Undo's own flow. The opened report may be
  // closed by then (a resolved report leaves the list), so it works from the
  // report id alone and resolves with the sentence the toast shows next. It
  // never rejects: a refusal comes back as the sentence to show. link is the
  // set key resolveToastAction kept when this report was the link, else null.
  function takeBackFromToast(reportId, wasClosed, link) {
    var origin = { reportId: String(reportId), controls: document.getElementById('toast') };
    return castVote(reportId, 'unresolve').then(function (result) {
      relinkAfterToastUndo(reportId, wasClosed, link, result);
      return Pinalert.fetchReports({ afterWrite: true }).then(function () {
        focusAfterTakeBack(origin);
        return takeBackSuccessMessage(wasClosed, result);
      });
    }, function (err) {
      // A refused toast Undo reads the list again too (WR-05), so the opened
      // report's own Undo never outlives the mark the server no longer has.
      // The toast shows the sentence at once; the list read is not waited for.
      var sentence = takeBackErrorMessage(err);
      if (answeredByServer(err)) {
        Pinalert.fetchReports({ afterWrite: true });
      }
      return sentence;
    });
  }

  // resolveToastAction is the optional second argument of the message that
  // follows Yes, mark resolved (C58). It exists only when the server says an
  // undo is left on this report; the browser computes no limit of its own.
  // The toast is built before the refetch that follows the mark, so the link
  // is still held here when this report is the one a /?report=ID link opened.
  // That refetch usually ends the link (a closed report leaves Active and its
  // pin goes), so the toast keeps the link and the set it was seen in.
  function resolveToastAction(reportId, result) {
    if (!result || typeof result.undos_left !== 'number' || result.undos_left <= 0) {
      return undefined;
    }
    var wasClosed = result.visibility === RETRACTED_VISIBILITY_SLUG;
    var link = Pinalert.state.focusId === String(reportId) ? linkSetKey() : null;
    return {
      label: TOAST_ACTION_LABEL,
      ariaLabel: TOAST_ACTION_NAME,
      onClick: function () {
        return takeBackFromToast(reportId, wasClosed, link);
      }
    };
  }

  // linkSetKey names the tab, the chip and the centre the reader is on, so a
  // toast Undo can tell whether the reader is still on the set the link was
  // seen in.
  function linkSetKey() {
    var s = Pinalert.state;
    return s.view + '|' + s.category + '|' + s.center.lat + '|' + s.center.lon;
  }

  // relinkAfterToastUndo puts a report link back for the toast Undo's own
  // refetch, so a linked report that the mark moved off the map comes back
  // with its pin and popup from the server's answer. It acts only when the
  // take back reopened the report, the link has not been put back already,
  // and the reader is still on the same set; a reader who moved on keeps the
  // link ended, and a report that stays closed would have no pin to hold it.
  function relinkAfterToastUndo(reportId, wasClosed, link, result) {
    var stillClosed = !!(result && result.visibility === RETRACTED_VISIBILITY_SLUG);
    if (link === null || !wasClosed || stillClosed || Pinalert.state.focusId !== null || link !== linkSetKey()) {
      return;
    }
    Pinalert.setFocusId(reportId);
  }

  window.PinalertVotes = {
    getVoterLocation: getVoterLocation,
    castVote: castVote,
    createVoteBlock: createVoteBlock,
    cancelResolveConfirm: cancelResolveConfirm,
    createReopenControl: createReopenControl,
    setReopenLabel: setVoteButtonLabel,
    updateVoteBlock: updateVoteBlock,
    farFromCachedFix: farFromCachedFix,
    setBlockBusy: setBlockBusy,
    showVoteError: showVoteError,
    resolutionOutcomeMessage: resolutionOutcomeMessage,
    GPS_DENIED_MESSAGE: GPS_DENIED_MESSAGE,
    GENERIC_FAILURE_MESSAGE: GENERIC_FAILURE_MESSAGE,
    RESOLVE_BUTTON_LABEL: RESOLVE_BUTTON_LABEL,
    RESOLVE_BUTTON_LABEL_IN_FLIGHT: RESOLVE_BUTTON_LABEL_IN_FLIGHT,
    REOPEN_BUTTON_LABEL: REOPEN_BUTTON_LABEL,
    REOPEN_BUTTON_LABEL_IN_FLIGHT: REOPEN_BUTTON_LABEL_IN_FLIGHT,
    REOPEN_SUCCEEDED_TOAST: REOPEN_SUCCEEDED_TOAST,
    REOPEN_PENDING_TOAST: REOPEN_PENDING_TOAST,
    RETRACTED_VISIBILITY_SLUG: RETRACTED_VISIBILITY_SLUG,
    takeBackState: takeBackState,
    TAKE_BACK_BUTTON_LABEL: TAKE_BACK_BUTTON_LABEL,
    TAKE_BACK_BUTTON_NAME: TAKE_BACK_BUTTON_NAME,
    TAKE_BACK_BUTTON_LABEL_IN_FLIGHT: TAKE_BACK_BUTTON_LABEL_IN_FLIGHT,
    TAKE_BACK_USED_MESSAGE: TAKE_BACK_USED_MESSAGE,
    TAKE_BACK_ERROR_MESSAGE: TAKE_BACK_ERROR_MESSAGE,
    TAKE_BACK_FADED_MESSAGE: TAKE_BACK_FADED_MESSAGE,
    TAKE_BACK_SUCCEEDED_CLOSED_TOAST: TAKE_BACK_SUCCEEDED_CLOSED_TOAST,
    TAKE_BACK_SUCCEEDED_WAITING_TOAST: TAKE_BACK_SUCCEEDED_WAITING_TOAST
  };
}());
