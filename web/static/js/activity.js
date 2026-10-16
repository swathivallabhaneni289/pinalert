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
// The page also mounts the saved reports strip through queue.js with one call
// and computes nothing about it.
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

  // The saved reports strip mounts first, before the list guard below, so it
  // shows for an author who has no report on the server yet. One call, and
  // queue.js does all the work.
  var queueStrip = document.getElementById('queue-strip');
  var queuePage = document.querySelector('.profile-page');
  if (window.PinalertQueue && queueStrip && queuePage) {
    PinalertQueue.mount({
      strip: queueStrip,
      accountId: queuePage.dataset.accountId,
      headingLevel: 3,
      resendHref: resendHref,
      focusFallback: queueFocusFallback
    });
  }

  // The five parts each hold their own list. The module works on the rows of
  // all of them, and one click listener on the page covers every part.
  var rows = document.querySelectorAll('.profile-report-list .report-row');
  if (!rows.length) {
    // Guard the whole module behind the rows' presence so this file is
    // inert if it is ever loaded on another page.
    return;
  }

  // The words that stand in place of Reopen once both undos are used (C52).
  var UNDOS_USED_SENTENCE = 'You have used both undos on this report.';

  // The visibility value that reads as Disputed on the Status chip. The two
  // chip words a reopened report can take are written by the template on the
  // chip itself (data-word-disputed and data-word-ongoing), so this file holds
  // no label of its own.
  var HIDDEN_VISIBILITY_SLUG = 'hidden';

  // reopenBlocks maps a report id to the { controls, button, error } object
  // the shared builder in votes.js returns, built as rows are rendered below.
  // One delegated click listener on the page (Task 3) looks entries up here
  // rather than each row carrying its own listener.
  var reopenBlocks = {};

  // trustBlocks maps a report id to the trust block instance built for that
  // row, so the Reopen success path can refresh the right one.
  var trustBlocks = {};

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
      expired: row.dataset.expired === 'true',
      // The server decided this from the resolver's answer and wrote it as the
      // row's moderation attribute, so this is an attribute read.
      moderation: row.dataset.moderation
    };

    var tag = PinalertVisibility.createVisibilityTag();
    // The What happened cell is where the state word, the trust block, Reopen
    // and the used both undos sentence go (D-21).
    var body = row.querySelector('.activity-card__what');
    PinalertVisibility.updateVisibilityTag(tag, report);
    PinalertVisibility.applyVisibilityClass(row, report);

    // The state label and the reporter words share the state line above the
    // trust block (A75). The block takes no options, so this page shows the
    // two counters and the panel but no score numbers (D-09, A50).
    //
    // A row the server marked as held or removed mounts no trust block and so
    // no reporter words: it has no trust attribute to read. Its state label
    // goes in front of the server sentence when the row has one, so the label
    // is read first.
    if (body && !report.moderation) {
      var trustBlock = PinalertTrust.createTrustBlock();
      body.appendChild(PinalertTrust.createStateLine('report-row__stateline', tag, trustBlock));
      body.appendChild(trustBlock.el);
      PinalertTrust.updateTrustBlock(trustBlock, readRowTrust(row));
      trustBlocks[row.dataset.reportId] = trustBlock;
    } else if (body) {
      // A held or removed row has no trust block. The state label goes in front
      // of the server sentence in the Report cell, so it is read first. The
      // stylesheet draws the label once, in the Status chip. With no sentence to
      // go before, the reference is null and the insert puts the label at the end
      // of the cell.
      var moderationNote = row.querySelector('.report-row__moderation-note');
      var moderationHost = moderationNote ? moderationNote.parentNode : body;
      moderationHost.insertBefore(tag, moderationNote);
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
    } else if (row.dataset.undosUsed === 'true' && body) {
      // Both undos of this report are used (D-22), so the server withheld the
      // Reopen control and wrote this hook instead. The sentence is a fixed
      // string written as text, and the control is never mounted here, so the
      // page does not offer a button the server would refuse.
      var usedNote = document.createElement('p');
      usedNote.className = 'vote-note';
      Pinalert.setText(usedNote, UNDOS_USED_SENTENCE);
      body.appendChild(usedNote);
    }
  }

  // The one delegated click listener this module attaches (Task 3's own
  // acceptance criterion). Ignores every click that did not land on a
  // .vote-btn--reopen — there is exactly one action this list ever
  // dispatches.
  document.querySelector('.profile-page').addEventListener('click', function (evt) {
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
  // as any other vote, through PinalertVotes.castVote. The control exists only
  // on a row the server marked as the account's own resolved, unexpired post,
  // and the server exempts the reporter's reopen from the 1 km rule, so the
  // call passes ownReport and never asks for a location (WR-04). No
  // geolocation code lives in this file.
  function onReopenClick(row, block) {
    PinalertVotes.setBlockBusy(block, true);
    var reopenBtn = block.button;
    if (reopenBtn) {
      PinalertVotes.setReopenLabel(reopenBtn, PinalertVotes.REOPEN_BUTTON_LABEL_IN_FLIGHT);
    }

    PinalertVotes.castVote(row.dataset.reportId, 'reopen', { ownReport: true }).then(function (result) {
      // The fixed toast is this module's one deliberate exception to
      // "render the server's answer, never a guess" — licensed by two
      // structural, checkable facts: (a) the server sets the Reopen hook
      // only on the account's own resolved, unexpired posts and never on a
      // voted card, so this button can only ever be rendered on a row the
      // viewer reported, so there is no non-reporter path through it at
      // all; and (b) 02-03a's amended
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

      // The Status chip follows the response too: Disputed when the report is
      // now hidden, Ongoing otherwise. A reopened post is unexpired by
      // construction, so Expired and Resolved cannot be the answer here.
      var chip = row.querySelector('.activity-chip');
      if (chip && updated.visibility !== PinalertVotes.RETRACTED_VISIBILITY_SLUG) {
        Pinalert.setText(chip, updated.visibility === HIDDEN_VISIBILITY_SLUG ? chip.dataset.wordDisputed : chip.dataset.wordOngoing);
      }

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

  // resendHref gives the saved reports strip the address of the page that has a
  // report form, with the saved report's request id. The page is the Low data
  // view when the server wrote that address as the page's resend page (the
  // device's remembered choice), else the main page. The attribute is only
  // compared and never copied, and the target is one of two constants.
  function resendHref(record) {
    var page = queuePage && queuePage.dataset.resendPage === '/low' ? '/low' : '/';
    return page + '?resend=' + encodeURIComponent(record.request_id);
  }

  // queueFocusFallback is where focus goes when the last saved report leaves
  // the strip with focus inside it: the first control in the report lists that
  // is on screen (a part the phone is not showing has no layout box), else the
  // first row of the list, else nothing. This page has no Report button, so it
  // names its own place.
  function queueFocusFallback() {
    var controls = document.querySelectorAll('.profile-report-list button, .profile-report-list a[href], .activity-hub__row');
    for (var c = 0; c < controls.length; c++) {
      if (!controls[c].closest('[hidden]') && controls[c].offsetParent !== null) {
        return controls[c];
      }
    }
    return null;
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
