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

  // The retracted visibility slug, held here as its own constant rather
  // than imported: this module loads before visibility.js (see the header
  // comment on window.PinalertVisibility's own file) and so cannot read
  // its VISIBILITY_STATES array at evaluation time. Kept in step with that
  // array by TestResolutionOutcomeSlugMatchesVisibilityStates.
  var RETRACTED_VISIBILITY_SLUG = 'retracted';

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

  // createVoteBlock builds the DOM for one report's controls: two buttons
  // (Confirm first, Dispute second, the Copywriting Contract's reading
  // order) inside a .vote-controls container, plus a hidden .vote-error
  // paragraph. It applies no state of its own — updateVoteBlock is always
  // called immediately after it. Built entirely with document.createElement,
  // never a markup string (T-01-03).
  function createVoteBlock(reportId) {
    var controls = document.createElement('div');
    controls.className = 'vote-controls';
    controls.dataset.reportId = String(reportId);

    var confirmBtn = document.createElement('button');
    confirmBtn.type = 'button';
    confirmBtn.className = 'vote-btn vote-btn--confirm';
    confirmBtn.dataset.action = 'confirm';
    confirmBtn.setAttribute('aria-pressed', 'false');
    Pinalert.setText(confirmBtn, 'Confirm');

    var disputeBtn = document.createElement('button');
    disputeBtn.type = 'button';
    disputeBtn.className = 'vote-btn vote-btn--dispute';
    disputeBtn.dataset.action = 'dispute';
    disputeBtn.setAttribute('aria-pressed', 'false');
    Pinalert.setText(disputeBtn, 'Dispute');

    controls.appendChild(confirmBtn);
    controls.appendChild(disputeBtn);

    // Mark resolved (Task 1, 02-07, D-13/D-15): the third button, appended
    // after Dispute per the Copywriting Contract's reading order. It
    // renders for the reporter and the non-reporter alike — the server's
    // response (via resolutionOutcomeMessage) is what distinguishes D-13's
    // instant path from D-14's threshold-gated one, never the markup.
    var resolveBtn = document.createElement('button');
    resolveBtn.type = 'button';
    resolveBtn.className = 'vote-btn vote-btn--resolve';
    resolveBtn.dataset.action = 'resolve';
    Pinalert.setText(resolveBtn, RESOLVE_BUTTON_LABEL);
    controls.appendChild(resolveBtn);

    // The inline destructive confirmation (D-15). Mounted INSIDE
    // .vote-controls — one level deeper than 02-UI-SPEC.md's sketch, which
    // drew it as a sibling with no data-action attributes — so its two
    // buttons are reached by the one delegated listener below rather than
    // needing a second one. Same markup, classes and copy either way; only
    // the nesting differs. Recorded in the SUMMARY as a deliberate
    // divergence.
    var confirmBlock = document.createElement('div');
    confirmBlock.className = 'resolve-confirm';
    confirmBlock.hidden = true;

    var confirmHeading = document.createElement('p');
    Pinalert.setText(confirmHeading, RESOLVE_CONFIRM_HEADING);
    confirmBlock.appendChild(confirmHeading);

    var affirmBtn = document.createElement('button');
    affirmBtn.type = 'button';
    affirmBtn.className = 'vote-btn vote-btn--confirm-resolve';
    affirmBtn.dataset.action = 'confirm-resolve';
    Pinalert.setText(affirmBtn, RESOLVE_CONFIRM_AFFIRM_LABEL);
    confirmBlock.appendChild(affirmBtn);

    var cancelBtn = document.createElement('button');
    cancelBtn.type = 'button';
    cancelBtn.className = 'vote-btn vote-btn--cancel-resolve';
    cancelBtn.dataset.action = 'cancel-resolve';
    Pinalert.setText(cancelBtn, RESOLVE_CONFIRM_CANCEL_LABEL);
    confirmBlock.appendChild(cancelBtn);

    controls.appendChild(confirmBlock);

    var error = document.createElement('p');
    error.className = 'vote-error';
    error.setAttribute('role', 'alert');
    error.hidden = true;

    var block = { controls: controls, error: error };

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

  // applyOwnReportRule removes the reporter's own Confirm/Dispute buttons
  // from the DOM rather than leaving them present but non-interactive
  // (D-03) — a UI courtesy layered on the server's enforced refusal
  // (02-03b), never a replacement for it. Runs on every call, including
  // every 30-second poll, so it must be idempotent: it queries the buttons
  // fresh each time and tolerates their already being gone.
  function applyOwnReportRule(block, report) {
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
    applyOwnReportRule(block, report);

    var confirmBtn = block.controls.querySelector('.vote-btn--confirm');
    var disputeBtn = block.controls.querySelector('.vote-btn--dispute');
    if (confirmBtn) {
      confirmBtn.setAttribute('aria-pressed', report.your_vote === 'confirm' ? 'true' : 'false');
    }
    if (disputeBtn) {
      disputeBtn.setAttribute('aria-pressed', report.your_vote === 'dispute' ? 'true' : 'false');
    }
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
  }

  // openResolveConfirm implements D-15's inline destructive confirmation:
  // hides the three primary buttons via the hidden attribute, reveals
  // .resolve-confirm, and moves focus to the affirm button. Never touches
  // `disabled` — setBlockBusy remains its sole owner (see its own doc
  // comment), which is what keeps a background poll landing mid-flight
  // from re-enabling a button whose vote is in the air.
  function openResolveConfirm(block) {
    var primaryBtns = block.controls.querySelectorAll(
      '.vote-btn--confirm, .vote-btn--dispute, .vote-btn--resolve'
    );
    for (var i = 0; i < primaryBtns.length; i++) {
      primaryBtns[i].hidden = true;
    }
    var confirmBlock = block.controls.querySelector('.resolve-confirm');
    confirmBlock.hidden = false;
    var affirmBtn = block.controls.querySelector('.vote-btn--confirm-resolve');
    if (affirmBtn) {
      affirmBtn.focus();
    }
  }

  // closeResolveConfirm reverses openResolveConfirm. Same `disabled`
  // discipline as above — this function never touches it either.
  function closeResolveConfirm(block) {
    var primaryBtns = block.controls.querySelectorAll(
      '.vote-btn--confirm, .vote-btn--dispute, .vote-btn--resolve'
    );
    for (var i = 0; i < primaryBtns.length; i++) {
      primaryBtns[i].hidden = false;
    }
    var confirmBlock = block.controls.querySelector('.resolve-confirm');
    confirmBlock.hidden = true;
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

  // onControlsClick is the only place D-04 is implemented: nothing about
  // the block's rendering changes until the server has answered. The
  // resolve/cancel-resolve branches (Task 1, 02-07) short-circuit before
  // any of that: opening or closing the inline confirmation posts nothing
  // and reads no location.
  function onControlsClick(block, button) {
    var action = button.dataset.action;

    if (action === 'resolve') {
      openResolveConfirm(block);
      return;
    }
    if (action === 'cancel-resolve') {
      closeResolveConfirm(block);
      return;
    }

    clearVoteError(block);
    setBlockBusy(block, true);

    var reportId = block.controls.dataset.reportId;
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

    castVote(reportId, transportAction).then(function (result) {
      if (isResolveAffirm) {
        closeResolveConfirm(block);
        Pinalert.showToast(resolutionOutcomeMessage('resolve', result));
      }
      // Keep the block busy across the refetch: releasing it before the
      // re-render would show buttons that still carry the pre-vote state
      // for the duration of the round trip, exactly the optimistic-
      // looking window D-04 forbids. The re-render is what applies the
      // server's your_vote — this module never assigns it itself. D-12's
      // removal of a newly-retracted report from the feed/map happens
      // here too, via this same refetch, never a local DOM removal.
      return Pinalert.fetchReports();
    }).then(function () {
      if (affirmBtn) {
        Pinalert.setText(affirmBtn, RESOLVE_CONFIRM_AFFIRM_LABEL);
      }
      setBlockBusy(block, false);
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
    });

    // One accepted limitation: map.js rebuilds its popup content on every
    // render, so a background poll landing during an in-flight vote
    // replaces this block and leaves this handler holding a detached
    // node — after which the freshly built block's buttons are already
    // interactive again. The consequence is bounded: a second tap in
    // that window sends a duplicate vote, which the append-only vote log
    // absorbs and the current-vote read collapses to one row (re-sending
    // the same vote is documented as harmless in 02-UI-SPEC.md's
    // interaction contract). Engineering around it would mean holding a
    // vote-scoped identity across a popup rebuild, which is more
    // machinery than the outcome justifies.
  }

  window.PinalertVotes = {
    getVoterLocation: getVoterLocation,
    castVote: castVote,
    createVoteBlock: createVoteBlock,
    updateVoteBlock: updateVoteBlock,
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
    RETRACTED_VISIBILITY_SLUG: RETRACTED_VISIBILITY_SLUG
  };
}());
