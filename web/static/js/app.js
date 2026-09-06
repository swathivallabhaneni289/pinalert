// app.js — window.Pinalert, the single shared client store.
//
// Plans 01-05 and 01-06 build entirely against this surface: the one
// GET /api/reports fetch, the submit call, cross-panel selection, and the
// formatting helpers both the map and the list need for identical output.
// Nothing else in the client fetches reports — see map.js, which reads only
// from Pinalert.state.
//
// SECURITY (T-01-03): report descriptions are authored by anonymous
// strangers and rendered in other visitors' browsers. Every piece of
// report-authored data must reach the DOM through Pinalert.setText (a
// text-node assignment) or an equivalent, never through a markup string a
// browser then parses. html/template protects only the server-rendered
// shell (index.html.tmpl); this file is the whole defence for anything
// JavaScript builds afterward.
window.Pinalert = (function () {
  'use strict';

  // Nine categories, in the server's declared order (service.Categories).
  var CATEGORIES = [
    'flood', 'earthquake', 'fire', 'storm_cyclone', 'road_blocked',
    'power_outage', 'shelter_open', 'rescue_needed', 'other'
  ];

  var SEVERITIES = ['low', 'medium', 'critical'];

  var CAPACITY_STATUSES = ['available', 'limited', 'full', 'closed'];

  // Numeric-plus-word form, per D-05: the raw slider value "2" alone does
  // not convey "Medium" to a screen reader or a sighted reader either.
  var SEVERITY_LABELS = {
    low: '1 · Low',
    medium: '2 · Medium',
    critical: '3 · Critical'
  };

  var CATEGORY_LABELS = {
    flood: 'Flood',
    earthquake: 'Earthquake',
    fire: 'Fire',
    storm_cyclone: 'Storm/Cyclone damage',
    road_blocked: 'Road blocked',
    power_outage: 'Power outage',
    shelter_open: 'Shelter open',
    rescue_needed: 'Rescue needed',
    other: 'Other'
  };

  var shellEl = document.getElementById('app-shell');

  function readNumberAttr(name, fallback) {
    if (!shellEl) {
      return fallback;
    }
    var raw = shellEl.getAttribute(name);
    if (raw === null || raw === '') {
      return fallback;
    }
    var parsed = parseFloat(raw);
    return isNaN(parsed) ? fallback : parsed;
  }

  // Fallback centre, default radius, and poll interval, read from the
  // shell's data attributes (set server-side by handlers.Page) so none of
  // this is hard-coded in JS.
  var config = {
    fallbackLat: readNumberAttr('data-fallback-lat', 12.9716),
    fallbackLon: readNumberAttr('data-fallback-lon', 77.5946),
    defaultRadiusKm: readNumberAttr('data-default-radius-km', 10),
    pollIntervalMs: readNumberAttr('data-poll-interval-ms', 30000)
  };

  var state = {
    reports: [],
    status: 'idle', // idle | loading | ok | error
    error: null,
    selectedId: null,
    center: { lat: config.fallbackLat, lon: config.fallbackLon }
  };

  var listeners = [];
  var selectListeners = [];
  var pollTimer = null;

  function notify() {
    for (var i = 0; i < listeners.length; i++) {
      listeners[i](state);
    }
  }

  // subscribe registers fn to be called with state on every transition.
  // Returns an unsubscribe function.
  function subscribe(fn) {
    listeners.push(fn);
    return function unsubscribe() {
      var idx = listeners.indexOf(fn);
      if (idx !== -1) {
        listeners.splice(idx, 1);
      }
    };
  }

  function setCenter(lat, lon) {
    state.center = { lat: lat, lon: lon };
  }

  // fetchReports performs the one and only GET /api/reports call. A
  // background refresh (status already "ok") leaves status untouched so the
  // list does not flash skeletons on every 30s poll.
  function fetchReports() {
    var isBackgroundRefresh = state.status === 'ok';
    if (!isBackgroundRefresh) {
      state.status = 'loading';
      notify();
    }

    var url = '/api/reports' +
      '?lat=' + encodeURIComponent(state.center.lat) +
      '&lon=' + encodeURIComponent(state.center.lon) +
      '&radius_km=' + encodeURIComponent(config.defaultRadiusKm);

    return fetch(url)
      .then(function (res) {
        if (!res.ok) {
          throw new Error('fetchReports: HTTP ' + res.status);
        }
        return res.json();
      })
      .then(function (body) {
        state.reports = (body && body.reports) || [];
        state.status = 'ok';
        state.error = null;
        notify();
      })
      .catch(function (err) {
        state.status = 'error';
        state.error = err;
        notify();
      });
  }

  // startPolling refreshes on config.pollIntervalMs. Safe to call more than
  // once — only the first call schedules a timer.
  function startPolling() {
    if (pollTimer !== null) {
      return;
    }
    pollTimer = window.setInterval(function () {
      fetchReports();
    }, config.pollIntervalMs);
  }

  // submitReport POSTs to /api/reports, resolves with the created report or
  // rejects with an Error carrying .field/.fieldMessage from the server's
  // {field, message} shape, and refreshes the store on success.
  function submitReport(payload) {
    return fetch('/api/reports', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    }).then(function (res) {
      return res.json().catch(function () {
        return {};
      }).then(function (body) {
        if (!res.ok) {
          var fieldError = (body && body.error) || {
            field: 'body',
            message: 'Something went wrong. Try again.'
          };
          var err = new Error(fieldError.message);
          err.field = fieldError.field;
          err.fieldMessage = fieldError.message;
          throw err;
        }
        return (body && body.report) || null;
      });
    }).then(function (report) {
      return fetchReports().then(function () {
        return report;
      });
    });
  }

  // select carries a selection between panels. source is 'map' or 'list' so
  // neither panel echoes an event it originated back to itself.
  function select(id, source) {
    state.selectedId = id;
    for (var i = 0; i < selectListeners.length; i++) {
      selectListeners[i](id, source);
    }
  }

  function onSelect(fn) {
    selectListeners.push(fn);
    return function unsubscribe() {
      var idx = selectListeners.indexOf(fn);
      if (idx !== -1) {
        selectListeners.splice(idx, 1);
      }
    };
  }

  function severityClass(report) {
    var severity = report && report.severity;
    if (SEVERITIES.indexOf(severity) === -1) {
      severity = 'low';
    }
    return 'sev-' + severity;
  }

  // iconPath validates category against CATEGORIES before building a path
  // (T-01-17): an unexpected value from the API can never be reflected into
  // a path unchecked. Falls back to other.svg for anything unrecognised.
  function iconPath(category) {
    if (CATEGORIES.indexOf(category) === -1) {
      return '/static/icons/other.svg';
    }
    return '/static/icons/' + category + '.svg';
  }

  function relativeTime(iso) {
    var then = new Date(iso).getTime();
    var now = Date.now();
    var diffSeconds = Math.max(0, Math.round((now - then) / 1000));

    if (diffSeconds < 60) {
      return 'just now';
    }
    var diffMinutes = Math.round(diffSeconds / 60);
    if (diffMinutes < 60) {
      return diffMinutes + ' min ago';
    }
    var diffHours = Math.round(diffMinutes / 60);
    if (diffHours < 24) {
      return diffHours + (diffHours === 1 ? ' hour ago' : ' hours ago');
    }
    var diffDays = Math.round(diffHours / 24);
    return diffDays + (diffDays === 1 ? ' day ago' : ' days ago');
  }

  // ageStage implements D-15/D-17: a report fades over roughly the last
  // quarter of its lifetime, expressed as three discrete desaturation
  // stages. remainingFraction > 0.25 -> fresh; 0.125 <= remainingFraction
  // <= 0.25 -> aging; remainingFraction < 0.125 -> stale.
  function ageStage(report) {
    var created = new Date(report.created_at).getTime();
    var expires = new Date(report.expires_at).getTime();
    var now = Date.now();
    var lifetime = expires - created;
    if (lifetime <= 0) {
      return 'stale';
    }
    var remainingFraction = (expires - now) / lifetime;
    if (remainingFraction > 0.25) {
      return 'fresh';
    }
    if (remainingFraction >= 0.125) {
      return 'aging';
    }
    return 'stale';
  }

  // setText assigns value to el as a text node only — see the file-level
  // security note above. Every report-authored string must go through this
  // (or an equivalent .textContent assignment) rather than a markup-parsing
  // sink.
  function setText(el, value) {
    el.textContent = value === null || value === undefined ? '' : String(value);
  }

  return {
    CATEGORIES: CATEGORIES,
    SEVERITIES: SEVERITIES,
    CAPACITY_STATUSES: CAPACITY_STATUSES,
    SEVERITY_LABELS: SEVERITY_LABELS,
    CATEGORY_LABELS: CATEGORY_LABELS,
    config: config,
    state: state,
    subscribe: subscribe,
    setCenter: setCenter,
    fetchReports: fetchReports,
    startPolling: startPolling,
    submitReport: submitReport,
    select: select,
    onSelect: onSelect,
    severityClass: severityClass,
    iconPath: iconPath,
    relativeTime: relativeTime,
    ageStage: ageStage,
    setText: setText
  };
}());
