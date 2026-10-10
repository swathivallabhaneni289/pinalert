// activity.js — the Activity page's whole client half (02-07). Exports no
// public API, matching account-menu.js's own convention: this module only
// reads the profile page's own server-rendered rows and upgrades them in
// place.
//
// This module computes nothing and fetches nothing (T-02-04): the
// Activity page renders every trust signal the server already computed
// into each row's data attributes, and this file's whole job is to turn
// those attributes into the same chip feed.js/map.js render, through the
// same visibility.js builders — never a second implementation, never a
// second label map, never a fetch of its own.
//
// The page also renders the shared trust block on each row, from the trust
// object the server wrote into the row, through the same trust.js builder the
// feed and the map popup use. This module still computes and fetches nothing.
//
// SECURITY (T-01-03): every string reaching the DOM goes through
// visibility.js's own builders (which themselves go through
// Pinalert.setText), never a markup-parsing sink.
(function () {
  'use strict';

  var list = document.querySelector('.profile-report-list');
  if (!list) {
    // Guard the whole module behind the list's presence so this file is
    // inert if it is ever loaded on another page.
    return;
  }

  // reopenBlocks maps a report id to the { controls, button, error } object
  // the shared builder in votes.js returns, built as rows are rendered below. One delegated click listener on `list`
  // (Task 3) looks entries up here rather than each row carrying its own
  // listener.
  var reopenBlocks = {};

  // trustBlocks maps a report id to the trust block instance built for that
  // row, so the Reopen success path can refresh the right one.
  var trustBlocks = {};

  var rows = list.querySelectorAll('.report-row');
  for (var i = 0; i < rows.length; i++) {
    var row = rows[i];

    // Shaped like one element of GET /api/reports' response — the exact
    // field names 02-04 published, confirmed against visibility.js's own
    // visibilityState/visibilityReason readers before this module was
    // wired in. The Activity page performs no fetch of its own, so this
    // object is built from the row's own server-rendered attributes and
    // nothing else.
    //
    // The server decided expiry and wrote it as the row's expired attribute,
    // so this is an attribute read and not a date comparison (A44).
    var report = {
      visibility: row.dataset.visibility,
      visibility_reason: row.dataset.visibilityReason,
      expired: row.dataset.expired === 'true'
    };

    var tag = PinalertVisibility.createVisibilityTag();
    var body = row.querySelector('.report-row__body');
    PinalertVisibility.updateVisibilityTag(tag, report);
    PinalertVisibility.applyVisibilityClass(row, report);

    // The state label and the reporter words share the state line above the
    // trust block (A75). The block takes no options, so this page shows the
    // two counters and the panel but no score numbers (D-09, A50).
    if (body) {
      var trustBlock = PinalertTrust.createTrustBlock();
      body.appendChild(PinalertTrust.createStateLine('report-row__stateline', tag, trustBlock));
      body.appendChild(trustBlock.el);
      PinalertTrust.updateTrustBlock(trustBlock, readRowTrust(row));
      trustBlocks[row.dataset.reportId] = trustBlock;
    }

    // Reopen (Task 3, D-16 amended). The button's render condition is the
    // server-rendered retracted-and-unexpired hook and NOTHING else —
    // this module reads no ownership field, compares no account and
    // computes no threshold anywhere (T-02-03). The reporter's instant
    // reopen is real, but it is CastVote's and Resolve's to grant from the
    // session's own account; the absence of any identity check here is
    // deliberate, not an oversight.
    if (row.dataset.canReopen === 'true' && body) {
      // Reopen comes from the one shared builder in votes.js, so the page
      // gets the same control, busy handling and error line the feed has.
      // There is no confirmation step: 02-UI-SPEC.md's Destructive
      // paragraph assigns that to Mark Resolved alone.
      var reopenBlock = PinalertVotes.createReopenControl(row.dataset.reportId);
      body.appendChild(reopenBlock.controls);
      body.appendChild(reopenBlock.error);
      reopenBlocks[row.dataset.reportId] = reopenBlock;
    }
  }

  // The one delegated click listener this module attaches (Task 3's own
  // acceptance criterion). Ignores every click that did not land on a
  // .vote-btn--reopen — there is exactly one action this list ever
  // dispatches.
  list.addEventListener('click', function (evt) {
    var btn = evt.target.closest('.vote-btn--reopen');
    if (!btn) {
      return;
    }
    var row = btn.closest('.report-row');
    var block = row && reopenBlocks[row.dataset.reportId];
    if (!row || !block) {
      return;
    }
    onReopenClick(row, block);
  });

  // onReopenClick runs the identical GPS-then-POST-then-wait flow (D-04)
  // as any other vote, through PinalertVotes.castVote. Every row here is the
  // viewer's own report, and the server exempts the reporter's reopen from the
  // 1 km rule, so the call passes ownReport and never asks for a location
  // (WR-04). No geolocation code lives in this file.
  function onReopenClick(row, block) {
    PinalertVotes.setBlockBusy(block, true);
    var reopenBtn = block.button;
    if (reopenBtn) {
      PinalertVotes.setReopenLabel(reopenBtn, PinalertVotes.REOPEN_BUTTON_LABEL_IN_FLIGHT);
    }

    PinalertVotes.castVote(row.dataset.reportId, 'reopen', { ownReport: true }).then(function (result) {
      // The fixed toast is this module's one deliberate exception to
      // "render the server's answer, never a guess" — licensed by two
      // structural, checkable facts: (a) this page lists ONLY the
      // viewing account's own submitted reports, so this button can only
      // ever be rendered on a row the viewer reported — there is no
      // non-reporter path through it at all; and (b) 02-03a's amended
      // CastVote makes a reporter's reopen instant and threshold-free
      // however the retraction arose (D-16 amended 2026-09-15; see
      // 02-CONTEXT.md's amendment history and 02-UI-SPEC.md's amended
      // Reopen section). Together they mean a 200 from this button
      // always means the report is reopened, so the pending-outcome copy
      // is unreachable here and a branch to select it would be dead code
      // pretending to be caution.
      Pinalert.showToast(PinalertVotes.REOPEN_SUCCEEDED_TOAST);

      // The exception stops at the toast: the chip, the state class and
      // the conditional removal of the control block all still come from
      // the RESPONSE's visibility, exactly like every other outcome in
      // this phase.
      var updated = {
        visibility: result && result.visibility,
        visibility_reason: result && result.reason
      };
      var tag = row.querySelector('.visibility-tag');
      PinalertVisibility.updateVisibilityTag(tag, updated);
      PinalertVisibility.applyVisibilityClass(row, updated);

      // R-08: the trust block refreshes from the trust object in the same
      // response. The updater never touches the open state, so an open panel
      // stays open.
      var rowTrust = trustBlocks[row.dataset.reportId];
      if (rowTrust) {
        PinalertTrust.updateTrustBlock(rowTrust, result && result.trust);
      }

      PinalertVotes.setBlockBusy(block, false);
      if (updated.visibility !== PinalertVotes.RETRACTED_VISIBILITY_SLUG) {
        block.controls.remove();
        block.error.remove();
        // The control that held focus is gone, so focus moves to this row's
        // trust toggle rather than falling back to the page body (A64).
        if (rowTrust) {
          PinalertTrust.focusToggle(rowTrust);
        }
      } else if (reopenBtn) {
        PinalertVotes.setReopenLabel(reopenBtn, PinalertVotes.REOPEN_BUTTON_LABEL);
      }
    }).catch(function (err) {
      PinalertVotes.setBlockBusy(block, false);
      if (reopenBtn) {
        PinalertVotes.setReopenLabel(reopenBtn, PinalertVotes.REOPEN_BUTTON_LABEL);
      }
      PinalertVotes.showVoteError(block, (err && err.fieldMessage) || PinalertVotes.GENERIC_FAILURE_MESSAGE);
    });
  }

  // readRowTrust parses the row's trust attribute, which is the server's
  // trust object escaped by the template. This module validates nothing
  // itself because trust.js does; a missing or malformed attribute yields
  // null, which hides the block while the rest of the row still renders.
  function readRowTrust(row) {
    try {
      return JSON.parse(row.dataset.trust);
    } catch (err) {
      return null;
    }
  }
}());
