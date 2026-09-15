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

  var rows = list.querySelectorAll('.report-row');
  for (var i = 0; i < rows.length; i++) {
    var row = rows[i];

    // Shaped like one element of GET /api/reports' response — the exact
    // field names 02-04 published, confirmed against visibility.js's own
    // visibilityState/visibilityReason readers before this module was
    // wired in. The Activity page performs no fetch of its own, so this
    // object is built from the row's own server-rendered attributes and
    // nothing else.
    var report = {
      visibility: row.dataset.visibility,
      visibility_reason: row.dataset.visibilityReason
    };

    var tag = PinalertVisibility.createVisibilityTag();
    var body = row.querySelector('.report-row__body');
    if (body) {
      body.appendChild(tag);
    }
    PinalertVisibility.updateVisibilityTag(tag, report);
    PinalertVisibility.applyVisibilityClass(row, report);
  }
}());
