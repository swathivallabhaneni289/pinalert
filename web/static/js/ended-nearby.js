// ended-nearby.js - window.PinalertEndedNearby: the browser half of My activity,
// part Ended nearby (Phase 3.4, plan 14, D-18 to D-21).
//
// Ended nearby lists everyone's reports near the reader that were resolved or
// faded out in the last 7 days. That is Phase 3.1's Past answer under a clearer
// name, with every feature kept. The page is drawn by the server without the
// reader's position, so this file reads the answer itself, after load:
//
//   - the position is the one Nearby (or the first time flow) last kept in this
//     browser, read through PinalertPosition. It is sent only as the query of
//     the one read below and is never stored, written or posted anywhere: the
//     server keeps no reader location (T-34-60);
//   - with no kept position nothing is requested, and the part says it needs the
//     location;
//   - the answer is drawn in two groups, Resolved then Faded out, each in the
//     server's order (newest first) and with the server's own cut and partial
//     flags; the rows are built by ended-cards.js;
//   - the same position fills the hub count and every Distance cell of the other
//     parts, from the coordinates the server wrote on each row;
//   - a link to a finished report (part=ended and report=ID) chooses that row,
//     with the soft grey fill, and brings it into view inside the page's own
//     scrolling region.
//
// The browser computes no outcome and no count of history: What happened is the
// server's words, and the only number made here is the great circle distance of
// a Distance cell, for display, never an input to any trust figure.
//
// SECURITY (T-01-03): report text reaches the DOM only through Pinalert.setText
// in ended-cards.js and the fixed strings below. The report id from the address
// is matched against a plain integer pattern before it is sent or compared
// (T-34-63).
window.PinalertEndedNearby = (function () {
  'use strict';

  var root = document.getElementById('ended-nearby');
  if (!root) {
    // Another page: this file is inert there.
    return null;
  }

  // The radius of the read is Nearby's own, written by the server on the page.
  var FALLBACK_RADIUS_KM = 10;
  // A read that stalls is abandoned after 10 seconds and shows the error state,
  // like the Nearby list read.
  var READ_TIMEOUT_MS = 10000;
  // A report id from the address is plain digits, at least 1, at most 18.
  var REPORT_ID_PATTERN = /^[1-9][0-9]{0,17}$/;
  // The earth's mean radius in kilometres, for the great circle distance.
  var EARTH_RADIUS_KM = 6371;
  // The words of a link to a report that cannot be shown here.
  var TOO_OLD_TOAST = 'That report is older than 7 days and is no longer shown.';
  var UNAVAILABLE_TOAST = "That report isn't available.";

  function byId(id) {
    return document.getElementById(id);
  }

  var skeletonEl = byId('ended-skeleton');
  var colsEl = byId('ended-cols');
  var emptyEl = byId('ended-empty');
  var locationEl = byId('ended-location');
  var errorEl = byId('ended-error');
  var retryEl = byId('ended-retry');
  var hubCountEl = byId('ended-hub-count');
  var titleCountEl = byId('ended-title-count');
  var groups = {
    resolved: { el: byId('ended-resolved'), list: byId('ended-resolved-list'), cut: byId('ended-resolved-cut') },
    expired: { el: byId('ended-faded'), list: byId('ended-faded-list'), cut: byId('ended-faded-cut') }
  };

  // rowsById holds the rows of the last draw, for the link highlight.
  var rowsById = {};
  // readSeq numbers the reads, so an answer that is no longer the newest
  // changes nothing.
  var readSeq = 0;
  // chosenId is the report id the address carries, or null.
  var chosenId = readChosenId();

  function setHidden(el, hidden) {
    if (el) {
      el.hidden = hidden;
    }
  }

  // showState shows exactly one of the part's states. 'groups' is the answer;
  // the group blocks themselves are shown by render.
  function showState(name) {
    setHidden(skeletonEl, name !== 'loading');
    setHidden(emptyEl, name !== 'empty');
    setHidden(locationEl, name !== 'location');
    setHidden(errorEl, name !== 'error');
    if (name !== 'groups') {
      setHidden(groups.resolved.el, true);
      setHidden(groups.expired.el, true);
      setHidden(colsEl, true);
    }
  }

  // readChosenId returns the report id of a link to a finished report, or null.
  // Only the part=ended address carries one, and only plain digits count.
  function readChosenId() {
    var search = window.location && window.location.search ? window.location.search : '';
    var params = new URLSearchParams(search);
    var id = params.get('report');
    if (params.get('part') !== 'ended' || id === null || !REPORT_ID_PATTERN.test(id)) {
      return null;
    }
    return id;
  }

  function readRadius() {
    var page = document.querySelector('.profile-page');
    var raw = page ? parseFloat(page.getAttribute('data-default-radius-km')) : NaN;
    return isFinite(raw) && raw > 0 ? raw : FALLBACK_RADIUS_KM;
  }

  function readPosition() {
    return typeof PinalertPosition !== 'undefined' ? PinalertPosition.read() : null;
  }

  // greatCircleKm is the haversine distance between two points, for the
  // Distance cells. Display only: it is never an input to a vote, a count or any
  // trust figure. votes.js keeps its own copy for the too far hint and exports
  // nothing of it by design (TestVoteHint), so this is written with the arctan
  // form of the same formula.
  function greatCircleKm(lat1, lon1, lat2, lon2) {
    var rad = Math.PI / 180;
    var dLat = (lat2 - lat1) * rad;
    var dLon = (lon2 - lon1) * rad;
    var a = Math.sin(dLat / 2) * Math.sin(dLat / 2) +
      Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(dLon / 2) * Math.sin(dLon / 2);
    a = Math.min(1, Math.max(0, a));
    return 2 * EARTH_RADIUS_KM * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  }

  // fillDistances writes "{d} km" into every Distance cell the server drew with
  // coordinates, from the kept position. With no position, or a cell without
  // usable coordinates, the cell reads Unknown. The cells of the Ended nearby
  // rows carry no coordinates: they are written from the server's own distance.
  function fillDistances(position) {
    var cells = document.querySelectorAll('.activity-card__distance');
    for (var i = 0; i < cells.length; i++) {
      var cell = cells[i];
      var rawLat = cell.getAttribute('data-lat');
      var rawLon = cell.getAttribute('data-lon');
      if (rawLat === null && rawLon === null) {
        continue;
      }
      var lat = rawLat === null || rawLat === '' ? NaN : Number(rawLat);
      var lon = rawLon === null || rawLon === '' ? NaN : Number(rawLon);
      var text = PinalertEnded.UNKNOWN_DISTANCE;
      if (position && isFinite(lat) && isFinite(lon)) {
        text = PinalertEnded.kmWords(greatCircleKm(position.lat, position.lon, lat, lon));
      }
      Pinalert.setText(cell, text);
    }
  }

  function clearList(listEl) {
    while (listEl.firstChild) {
      listEl.removeChild(listEl.firstChild);
    }
  }

  // drawGroup fills one group from the reports the server put in it. The block
  // is drawn when it has rows, or when the server says it is partial: a partial
  // group is never left out, so the reader is told some are missing (WR-02). The
  // line under it is built by groupLineText from the server's two flags and the
  // count of rows from the server (not counting a linked row put on top). It
  // returns the number of rows drawn.
  function drawGroup(group, reports, linked, wasCut, wasPartial) {
    clearList(group.list);
    var lineText = PinalertEnded.groupLineText(reports.length, wasCut, wasPartial);
    var all = linked ? [linked].concat(reports) : reports;
    for (var i = 0; i < all.length; i++) {
      var row = PinalertEnded.createEndedRow(all[i].id);
      PinalertEnded.updateEndedRow(row, all[i]);
      group.list.appendChild(row.el);
      rowsById[String(all[i].id)] = row;
    }
    setHidden(group.el, all.length === 0 && !wasPartial);
    setHidden(group.cut, lineText === '');
    Pinalert.setText(group.cut, lineText);
    return all.length;
  }

  function stateOf(report) {
    return report && report.history ? report.history.state : '';
  }

  // splitByState puts the answer's reports in their group, keeping the order
  // the server sent. A report with no history, or an id that is not a whole
  // number, belongs to neither group and is not drawn.
  function splitByState(reports) {
    var out = { resolved: [], expired: [] };
    for (var i = 0; i < reports.length; i++) {
      var report = reports[i];
      if (!report || !Number.isInteger(report.id)) {
        continue;
      }
      var state = stateOf(report);
      if (state === 'resolved' || state === 'expired') {
        out[state].push(report);
      }
    }
    return out;
  }

  // linkedReport returns the linked report of a focus answer when it belongs to
  // a group and is not already in one, else null. A report that is not
  // available is told once, in words.
  function linkedReport(answer, split) {
    var focus = answer && answer.focus;
    if (!focus || chosenId === null) {
      return { report: null, told: false };
    }
    if (focus.status === 'too_old') {
      Pinalert.showToast(TOO_OLD_TOAST);
      return { report: null, told: true };
    }
    if (focus.status === 'unavailable') {
      Pinalert.showToast(UNAVAILABLE_TOAST);
      return { report: null, told: true };
    }
    var report = focus.status === 'ok' ? focus.report : null;
    if (!report || !Number.isInteger(report.id)) {
      return { report: null, told: false };
    }
    var state = stateOf(report);
    if (state !== 'resolved' && state !== 'expired') {
      return { report: null, told: false };
    }
    var all = split.resolved.concat(split.expired);
    for (var i = 0; i < all.length; i++) {
      if (all[i].id === report.id) {
        return { report: null, told: false };
      }
    }
    return { report: report, told: false };
  }

  // writeCount puts the number of rows shown in the hub row and, on a laptop,
  // beside the part title. Both elements are in the page, hidden until now.
  function writeCount(count) {
    [hubCountEl, titleCountEl].forEach(function (el) {
      if (el) {
        Pinalert.setText(el, String(count));
        el.hidden = false;
      }
    });
  }

  // render draws an answer of the Past read. Zero reports and no partial flag is
  // the empty state; a partial flag always keeps its group on screen, so an
  // empty block is never claimed while the server says some are missing.
  function render(answer, quiet) {
    var reports = answer && Array.isArray(answer.reports) ? answer.reports : [];
    var cut = answer && answer.cut ? answer.cut : {};
    var resolvedPartial = cut.resolved_partial === true;
    var expiredPartial = cut.expired_partial === true;
    var split = splitByState(reports);
    var linked = linkedReport(answer, split);
    var linkedState = linked.report ? stateOf(linked.report) : '';

    rowsById = {};
    var shown = drawGroup(groups.resolved, split.resolved, linkedState === 'resolved' ? linked.report : null,
      cut.resolved === true, resolvedPartial);
    shown += drawGroup(groups.expired, split.expired, linkedState === 'expired' ? linked.report : null,
      cut.expired === true, expiredPartial);

    writeCount(shown);
    if (shown === 0 && !resolvedPartial && !expiredPartial) {
      showState('empty');
      return shown;
    }
    showState('groups');
    setHidden(colsEl, shown === 0);
    if (chosenId !== null && !linked.told) {
      highlight(chosenId, quiet === true);
    }
    return shown;
  }

  // scrollRegionOf finds the nearest ancestor that scrolls on its own: the page
  // content element on a phone. It returns null when the page itself scrolls.
  function scrollRegionOf(el) {
    for (var node = el.parentNode; node && node.nodeType === 1; node = node.parentNode) {
      if (typeof window.getComputedStyle !== 'function') {
        return null;
      }
      var overflowY = window.getComputedStyle(node).overflowY;
      if ((overflowY === 'auto' || overflowY === 'scroll') && node.scrollHeight > node.clientHeight) {
        return node;
      }
    }
    return null;
  }

  // highlight chooses the row of a linked report with the soft grey fill and
  // brings it into view inside the page's own scrolling region, never by
  // scrolling the window. It answers whether the row was found. The row takes
  // focus without scrolling when a region scrolls it, so a keyboard reader
  // starts there.
  function highlight(reportId, keepPlace) {
    var row = rowsById[String(reportId)];
    if (!row) {
      return false;
    }
    var previous = root.querySelectorAll('.activity-card--chosen');
    for (var i = 0; i < previous.length; i++) {
      previous[i].classList.remove('activity-card--chosen');
    }
    row.el.classList.add('activity-card--chosen');
    if (keepPlace === true) {
      // A quiet redraw after a take back keeps the fill and moves nothing.
      return true;
    }
    row.el.setAttribute('tabindex', '-1');
    var region = scrollRegionOf(row.el);
    if (region) {
      var inset = row.el.getBoundingClientRect().top - region.getBoundingClientRect().top;
      region.scrollTop = region.scrollTop + inset - 16;
      row.el.focus({ preventScroll: true });
    } else {
      row.el.focus();
    }
    return true;
  }

  // load reads the Past answer once for the kept position and draws it. With no
  // position it sends nothing and shows the location state. It answers a
  // promise that settles when the part is drawn, whatever the result.
  //
  // The quiet form (used by refresh, after a take back) shows no skeleton, asks
  // for no linked report, moves nothing and leaves the rows as they are when the
  // read fails: the caller learns of the failure from the null answer.
  function load(quiet) {
    var position = readPosition();
    if (quiet === true && !position) {
      return Promise.resolve(null);
    }
    var seq = ++readSeq;
    fillDistances(position);
    if (!position) {
      showState('location');
      return Promise.resolve(null);
    }
    if (quiet !== true) {
      showState('loading');
    }

    var url = '/api/reports' +
      '?lat=' + encodeURIComponent(position.lat) +
      '&lon=' + encodeURIComponent(position.lon) +
      '&radius_km=' + encodeURIComponent(readRadius()) +
      '&view=past';
    if (chosenId !== null && quiet !== true) {
      url += '&report=' + encodeURIComponent(chosenId);
    }

    var limit = Pinalert.requestTimeout(READ_TIMEOUT_MS, 'endedNearby');
    var answered = fetch(url, { signal: limit.signal }).then(function (res) {
      if (!res.ok) {
        throw new Error('endedNearby: HTTP ' + res.status);
      }
      return res.json();
    });
    return Promise.race([answered, limit.expired]).then(function (body) {
      limit.clear();
      if (seq !== readSeq) {
        return null;
      }
      render(body, quiet === true);
      return body;
    }).catch(function () {
      limit.clear();
      if (seq === readSeq && quiet !== true) {
        showState('error');
      }
      return null;
    });
  }

  // focusLost says whether focus sits on nothing useful: no element, the page
  // body or an element that left the page with the rows just replaced.
  function focusLost() {
    var active = document.activeElement;
    return !active || active === document.body || active.isConnected === false;
  }

  // focusAfterRedraw puts focus back after a redraw took it: on the report's
  // take back button when the new row still has one, else on its row, else (the
  // report left the part) on the part's title.
  function focusAfterRedraw(reportId) {
    var row = rowsById[String(reportId)];
    if (row) {
      var undo = row.takeBack ? row.takeBack.el.querySelector('.vote-btn--takeback') : null;
      if (undo && !undo.hidden && !undo.disabled) {
        undo.focus();
        return;
      }
      row.el.setAttribute('tabindex', '-1');
      row.el.focus();
      return;
    }
    var title = byId('activity-title-ended');
    if (title) {
      title.setAttribute('tabindex', '-1');
      title.focus();
    }
  }

  // refresh reads the part again after a take back and redraws it in place, with
  // no skeleton. A line a row shows (the refusal of a take back) is carried to
  // the same report's new row, and focus the redraw took goes back with
  // focusAfterRedraw. It resolves true when the part was read again and false
  // when it could not be, and never rejects.
  function refresh(reportId) {
    var carried = {};
    Object.keys(rowsById).forEach(function (id) {
      var row = rowsById[id];
      var line = row.takeBack ? row.takeBack.message() : '';
      if (line !== '') {
        carried[id] = line;
      }
    });
    return load(true).then(function (body) {
      if (body === null) {
        return false;
      }
      Object.keys(carried).forEach(function (id) {
        var row = rowsById[id];
        if (row && row.takeBack) {
          row.takeBack.showMessage(carried[id]);
        }
      });
      if (focusLost()) {
        focusAfterRedraw(reportId);
      }
      return true;
    });
  }

  if (retryEl) {
    retryEl.addEventListener('click', function () {
      load();
    });
  }

  var api = {
    load: load,
    refresh: refresh,
    render: render,
    fillDistances: fillDistances,
    highlight: highlight,
    greatCircleKm: greatCircleKm
  };

  load();
  return api;
}());
