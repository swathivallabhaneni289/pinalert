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

  // Each part holds its own list. The module works on the rows of all of them.
  //
  // Amended 2026-11-17 (quick 261117-vuf): Posted left My activity (your posts
  // are on your profile), and with it the poster's Reopen control and the "used
  // both undos" sentence, which only its cards carried. Every card here is a
  // voted card now, so this file mounts no control and listens for no click.
  var rows = document.querySelectorAll('.profile-report-list .report-row');
  if (!rows.length) {
    // Guard the whole module behind the rows' presence so this file is
    // inert if it is ever loaded on another page.
    return;
  }

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
    // The What happened cell is where the state word and the trust block go
    // (D-21).
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
      // The reporter's profile path, when the server wrote one on the card: the
      // last line of the explanation panel (plan 09-09, D-04).
      PinalertTrust.setProfileLink(trustBlock, row.dataset.reporterProfilePath);
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
