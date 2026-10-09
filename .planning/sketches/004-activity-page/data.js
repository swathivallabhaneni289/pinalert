// data.js: sample reports and events for sketch 004, the Activity page. Mockup only, classic script, no network.
//
// Every name, number and sentence here is sample data. A report has the same shape as sketch 003's S3.STATES (category,
// severity, visibility, age, the two counts, the still current signal, the reporter signal), so the same trust block
// builders and the same wording apply. An event is one line of the person's activity: they posted a report, confirmed
// one, disputed one, or (Phase 3.2, drawn dashed) the app told them something changed. The signals obey
// internal/service/currency.go the way sketch 003's data does: a standing dispute raises the bar to K = threshold +
// disputes, the score is 0 when N is below K, and a Provisional report has no still current word.
window.A4 = window.A4 || {};
(function (A4) {
  'use strict';

  var REPORTS = {
    // The person's own reports.
    fire: {
      reportId: 41, category: 'fire', severity: 'critical', visibility: 'live', age: 'fresh',
      description: 'Fire near Beach Road junction, smoke is drifting towards the market',
      shelter: null, confirmed: 3, disputed: 0, confirmedText: '3', disputedText: '0',
      currency: { label: 'Up to date', band: 'up_to_date', score: 99, recent: 3, k: 2, disputeRaised: false },
      reporter: { label: 'Reliable reporter', tag: 'reliable', score: 81, judged: 9, corroborated: 7 }
    },
    tree: {
      reportId: 40, category: 'road_blocked', severity: 'low', visibility: 'provisional', age: 'fresh',
      description: 'Big tree fallen across the road near Adyar bridge, only two wheelers can pass',
      shelter: null, confirmed: 0, disputed: 0, confirmedText: '0', disputedText: '0',
      currency: null,
      reporter: { label: 'New reporter', tag: 'new', score: null, judged: 0, corroborated: 0 }
    },
    outage_resolved: {
      reportId: 33, category: 'power_outage', severity: 'medium', visibility: 'retracted', age: 'fresh', canReopen: true,
      description: 'Power cut near Old Bridge, three streets are dark',
      shelter: null, confirmed: 2, disputed: 0, confirmedText: '2', disputedText: '0',
      currency: null, reporter: null
    },
    road_expired: {
      reportId: 29, category: 'road_blocked', severity: 'low', visibility: 'expired', age: 'fresh',
      description: 'Fallen hoarding is blocking the left lane near Lake View',
      shelter: null, confirmed: 2, disputed: 0, confirmedText: '2', disputedText: '0',
      currency: null,
      reporter: { label: 'Mixed record', tag: 'mixed', score: 55, judged: 9, corroborated: 5 }
    },
    // Reports by other people that the person voted on.
    underpass: {
      reportId: 38, category: 'flood', severity: 'medium', visibility: 'live', age: 'aging',
      description: 'Knee deep water in the underpass on Mount Road, buses are turning back',
      shelter: null, confirmed: 7, disputed: 1, confirmedText: '7', disputedText: '1',
      currency: { label: 'Getting old', band: 'getting_old', score: 52, recent: 1, k: 3, disputeRaised: true },
      reporter: { label: 'Reliable reporter', tag: 'reliable', score: 78, judged: 9, corroborated: 7 }
    },
    market_outage: {
      reportId: 37, category: 'power_outage', severity: 'low', visibility: 'live', age: 'fresh',
      description: 'No power in the Market Road lanes since evening, shops are closed',
      shelter: null, confirmed: 2, disputed: 1, confirmedText: '2', disputedText: '1',
      currency: { label: 'Needs re-confirming', band: 'needs_reconfirming', score: 0, recent: 0, k: 3, disputeRaised: true },
      reporter: { label: 'Mixed record', tag: 'mixed', score: 55, judged: 9, corroborated: 5 }
    },
    station_flood: {
      reportId: 35, category: 'flood', severity: 'medium', visibility: 'hidden', age: 'fresh',
      description: 'Ankle deep water on the platform side of Station Road',
      shelter: null, confirmed: 1, disputed: 3, confirmedText: '1', disputedText: '3',
      currency: null,
      reporter: { label: 'Unreliable reporter', tag: 'unreliable', score: 12, judged: 9, corroborated: 1 }
    },
    shelter_resolved: {
      reportId: 31, category: 'shelter_open', severity: 'low', visibility: 'retracted', age: 'fresh',
      description: 'Government school on Velachery Main Road is open as a relief shelter',
      shelter: 'Limited · 150 people', confirmed: 4, disputed: 0, confirmedText: '4', disputedText: '0',
      currency: null, reporter: null
    }
  };

  // minutes is how long ago the event happened. Every list is built newest first from it. later marks an event that only
  // exists once Phase 3.2 ships (the in app updates), so the board can draw it dashed and say so.
  var EVENTS = [
    { id: 'e01', report: 'fire',             kind: 'post',    minutes: 4 },
    { id: 'e02', report: 'tree',             kind: 'post',    minutes: 12 },
    { id: 'e03', report: 'underpass',        kind: 'vote',    minutes: 60 },
    { id: 'e04', report: 'market_outage',    kind: 'dispute', minutes: 180 },
    { id: 'e05', report: 'shelter_resolved', kind: 'update',  minutes: 300, later: true, text: 'marked Resolved' },
    { id: 'e06', report: 'station_flood',    kind: 'update',  minutes: 360, later: true, text: 'moved to Disputed' },
    { id: 'e07', report: 'shelter_resolved', kind: 'vote',    minutes: 420 },
    { id: 'e08', report: 'outage_resolved',  kind: 'post',    minutes: 480 },
    { id: 'e09', report: 'station_flood',    kind: 'dispute', minutes: 540 },
    { id: 'e10', report: 'road_expired',     kind: 'post',    minutes: 600 }
  ];

  A4.REPORTS = REPORTS;
  A4.EVENTS = EVENTS;

  // The small label above each title. Plain text, like the picture's tag but without the tag look (sketch 003 took the
  // chip look off the reporter words for the same reason: the row already has two labels).
  A4.SOURCE_LABELS = { post: 'Your post', vote: 'Your vote', dispute: 'Your dispute', update: 'Update' };

  var VERB = { vote: 'you confirmed it', dispute: 'you disputed it' };

  // The same coarse buckets the app uses for relative time (just now, N min ago, N hours ago, N days ago).
  A4.ago = function (minutes) {
    if (minutes < 1) { return 'just now'; }
    if (minutes < 60) { return minutes + ' min ago'; }
    var hours = Math.round(minutes / 60);
    if (hours < 24) { return hours === 1 ? '1 hour ago' : hours + ' hours ago'; }
    var days = Math.round(hours / 24);
    return days === 1 ? '1 day ago' : days + ' days ago';
  };

  // A copy of the report with the time the row should show. Only the poster's own row shows the posting time.
  A4.stateFor = function (ev) {
    var report = REPORTS[ev.report];
    var copy = {};
    Object.keys(report).forEach(function (k) { copy[k] = report[k]; });
    copy.timeText = ev.kind === 'post' ? A4.ago(ev.minutes) : '';
    return copy;
  };

  // The line under the title. A post keeps today's line (category, time, a shelter's capacity). A vote or an update says
  // what happened and when, after the category, so the round icon is never the only word for the kind of report.
  A4.metaFor = function (ev, state) {
    var S3 = window.S3;
    if (ev.kind === 'post') { return S3.metaText(state); }
    var category = S3.CATEGORY_LABELS[state.category];
    if (ev.kind === 'update') { return category + ' · ' + ev.text + ' ' + A4.ago(ev.minutes); }
    return category + ' · ' + VERB[ev.kind] + ' ' + A4.ago(ev.minutes);
  };

  // Which state filter a post belongs to inside My posts. Resolved is closed by people. Past is expired. Everything else
  // is still open (Live, Unconfirmed, Disputed). A Past post shows under All only, which is an open point for the
  // Phase 3.2 discussion.
  A4.bucket = function (state) {
    if (state.visibility === 'retracted') { return 'resolved'; }
    if (state.visibility === 'expired') { return 'past'; }
    return 'active';
  };

  // The tab counts. Real in the app (distinct reports), sample here.
  A4.counts = function (events) {
    var votedReports = {};
    var posts = 0;
    events.forEach(function (ev) {
      if (ev.kind === 'post') { posts += 1; }
      if (ev.kind === 'vote' || ev.kind === 'dispute') { votedReports[ev.report] = true; }
    });
    return { posts: posts, votes: Object.keys(votedReports).length };
  };
}(window.A4));
