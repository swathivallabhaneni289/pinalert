// data.js: sample reports and the two panel wordings for sketch 003. Mockup only, classic script, no network.
//
// Every name, number and sentence here is sample data. The "current" wording copies internal/service/trustcopy.go
// word for word. The "proposed" wording is the shorter set this sketch asks the owner to accept or reject on its own.
// Both stay inside the closed character set of TestTrustCopy: letters, digits, space, comma, full stop, apostrophe
// and plus, with a hyphen only inside "re-confirming". No relative time, no colon, no parenthesis.
//
// The sample states obey internal/service/currency.go: a standing dispute inside the window raises the bar to
// K = threshold + disputes, the score is 0 when N is below K, and a Provisional report has no still current word.
window.S3 = window.S3 || {};
(function (S3) {
  'use strict';

  var THRESHOLD = 2;       // IndependentAgreementThreshold
  var MIN_JUDGED = 5;      // ReliabilityMinJudged
  var WINDOW = '2 hours';  // windowPhrase for an 8 hour report

  S3.THRESHOLD = THRESHOLD;
  S3.MIN_JUDGED = MIN_JUDGED;
  S3.WINDOW = WINDOW;

  S3.CATEGORY_LABELS = {
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
  S3.SEVERITY_WORDS = { low: 'Low', medium: 'Medium', critical: 'Critical' };
  S3.STATE_WORDS = { provisional: 'Unconfirmed', hidden: 'Disputed', retracted: 'Resolved', expired: 'Expired' };

  // visibility: live | provisional | retracted | expired. A Resolved or Expired report only appears on the Activity
  // page in the app, so those two carry activityOnly. The sketch still draws them on every surface to judge the block.
  S3.STATES = [
    {
      id: 1, menu: '1. Unconfirmed, 0 confirmed, low, New reporter',
      category: 'road_blocked', severity: 'low', visibility: 'provisional', age: 'fresh',
      description: 'Big tree fallen across the road near Adyar bridge, only two wheelers can pass',
      timeText: '4 min ago', shelter: null,
      confirmed: 0, disputed: 0, confirmedText: '0', disputedText: '0',
      currency: null,
      reporter: { label: 'New reporter', tag: 'new', score: null, judged: 0, corroborated: 0 }
    },
    {
      id: 2, menu: '2. Unconfirmed, 1 confirmed, low, New reporter',
      category: 'shelter_open', severity: 'low', visibility: 'provisional', age: 'fresh',
      description: 'Government school on Velachery Main Road is open as a relief shelter',
      timeText: '35 min ago', shelter: 'Limited · 150 people',
      confirmed: 1, disputed: 0, confirmedText: '1', disputedText: '0',
      currency: null,
      reporter: { label: 'New reporter', tag: 'new', score: null, judged: 1, corroborated: 1 }
    },
    {
      id: 3, menu: '3. Live, 2 confirmed, Up to date, low, New reporter',
      category: 'power_outage', severity: 'low', visibility: 'live', age: 'fresh',
      description: 'No power in Gandhi Street and the two lanes next to it since evening',
      timeText: '12 min ago', shelter: null,
      confirmed: 2, disputed: 0, confirmedText: '2', disputedText: '0',
      currency: { label: 'Up to date', band: 'up_to_date', score: 99, recent: 2, k: 2, disputeRaised: false },
      reporter: { label: 'New reporter', tag: 'new', score: null, judged: 2, corroborated: 2 }
    },
    {
      id: 4, menu: '4. Live, 7 confirmed, 1 disputed, Getting old, medium, Reliable reporter',
      category: 'flood', severity: 'medium', visibility: 'live', age: 'aging',
      description: 'Knee deep water on the Mount Road service lane, buses are turning back',
      timeText: '3 hours ago', shelter: null,
      confirmed: 7, disputed: 1, confirmedText: '7', disputedText: '1',
      currency: { label: 'Getting old', band: 'getting_old', score: 52, recent: 1, k: 3, disputeRaised: true },
      reporter: { label: 'Reliable reporter', tag: 'reliable', score: 78, judged: 9, corroborated: 7 }
    },
    {
      id: 5, menu: '5. Critical, 1 confirmed, Too early to tell, Mixed record',
      category: 'rescue_needed', severity: 'critical', visibility: 'live', age: 'fresh',
      description: 'Elderly couple stuck on the first floor, water is rising in Canal Street',
      timeText: '2 min ago', shelter: null,
      confirmed: 1, disputed: 0, confirmedText: '1', disputedText: '0',
      currency: { label: 'Too early to tell', band: 'not_yet_corroborated', score: null, recent: 1, k: 2, disputeRaised: false },
      reporter: { label: 'Mixed record', tag: 'mixed', score: 55, judged: 9, corroborated: 5 }
    },
    {
      id: 6, menu: '6. Critical, 3 confirmed, 2 disputed, Needs re-confirming, Unreliable reporter',
      category: 'fire', severity: 'critical', visibility: 'live', age: 'fresh',
      description: 'Fire near Beach Road junction, smoke is drifting towards the market',
      timeText: '4 hours ago', shelter: null,
      confirmed: 3, disputed: 2, confirmedText: '3', disputedText: '2',
      currency: { label: 'Needs re-confirming', band: 'needs_reconfirming', score: 0, recent: 0, k: 4, disputeRaised: true },
      reporter: { label: 'Unreliable reporter', tag: 'unreliable', score: 12, judged: 9, corroborated: 1 }
    },
    {
      id: 7, menu: '7. Resolved, no reporter, no still current word',
      category: 'flood', severity: 'low', visibility: 'retracted', age: 'fresh', activityOnly: true, canReopen: true,
      description: 'Water has gone down on Cenotaph Road and traffic is moving again',
      timeText: '5 hours ago', shelter: null,
      confirmed: 3, disputed: 0, confirmedText: '3', disputedText: '0',
      currency: null,
      reporter: null
    },
    {
      id: 8, menu: '8. Expired, Reliable reporter, no still current word',
      category: 'power_outage', severity: 'medium', visibility: 'expired', age: 'fresh', activityOnly: true,
      description: 'Transformer sparking near the Kotturpuram bus depot, power is off for the block',
      timeText: '9 hours ago', shelter: null,
      confirmed: 2, disputed: 0, confirmedText: '2', disputedText: '0',
      currency: null,
      reporter: { label: 'Reliable reporter', tag: 'reliable', score: 89, judged: 9, corroborated: 8 }
    },
    {
      id: 9, menu: '9. Stress: 99+ counts, longest labels, long text',
      category: 'flood', severity: 'critical', visibility: 'live', age: 'fresh',
      description: 'Flooding on the service road behind the old railway station has cut off all three lanes and water is entering the ground floor shops, please avoid the whole stretch until the pumps arrive. Details at https://example.invalid/flood/service-road-behind-railway-station',
      timeText: '4 hours ago', shelter: null,
      confirmed: 128, disputed: 104, confirmedText: '99+', disputedText: '99+',
      currency: { label: 'Needs re-confirming', band: 'needs_reconfirming', score: 41, recent: 3, k: 62, disputeRaised: true },
      reporter: { label: 'Unreliable reporter', tag: 'unreliable', score: 12, judged: 30, corroborated: 4 }
    }
  ];

  S3.getState = function (id) {
    for (var i = 0; i < S3.STATES.length; i++) {
      if (String(S3.STATES[i].id) === String(id)) { return S3.STATES[i]; }
    }
    return S3.STATES[0];
  };

  // ---- Wording ---------------------------------------------------------------------------------------------
  // Each wording is a set of small functions. A function returns the sentence for one panel line, or '' when the
  // line does not show. The panel order is fixed by 03-UI-SPEC: lead, confirmed, disputed, still current,
  // still current score (popup only), reporter, reporter score (popup only).
  var CURRENT = {
    key: 'current',
    lead: function (kind) {
      if (kind === 'expired') { return 'This report has expired. It can no longer be confirmed.'; }
      return 'Needs ' + THRESHOLD + ' confirmations before this counts as confirmed.';
    },
    confirmed: function (n) {
      var tail = ' Counts separate places, not votes.';
      if (n <= 0) { return 'No one has confirmed this yet.' + tail; }
      if (n === 1) { return '1 person in 1 place confirmed this.' + tail; }
      return n + ' people in ' + n + ' different places confirmed this.' + tail;
    },
    disputed: function (m) {
      var tail = ' Counts separate places, not votes.';
      if (m <= 0) { return 'No one nearby has disputed this.' + tail; }
      if (m === 1) { return '1 person in 1 place disputed this.' + tail; }
      return m + ' people in ' + m + ' different places disputed this.' + tail;
    },
    currency: function (c) {
      if (c.band === 'not_yet_corroborated') { return 'Fewer than ' + THRESHOLD + ' places have confirmed this so far.'; }
      if (c.band === 'up_to_date') { return c.recent + ' different places confirmed this in the last ' + WINDOW + '.'; }
      if (c.disputeRaised) { return 'Someone nearby disputed this, so it needs ' + c.k + ' places to confirm it in the last ' + WINDOW + '.'; }
      if (c.band === 'getting_old') { return 'Confirmed earlier, but fewer than ' + THRESHOLD + ' places confirmed it in the last ' + WINDOW + '.'; }
      return 'Confirmed earlier, but it has gone quiet. People nearby can confirm it again.';
    },
    currencyScore: function (score) {
      return 'Score ' + score + ' out of 100, based on how recently different places confirmed this.';
    },
    reporter: function (r) {
      if (r.tag === 'new') {
        var lead;
        if (r.judged >= 2) { lead = 'Only ' + r.judged + ' of their reports have had a clear result so far.'; }
        else if (r.judged === 1) { lead = 'Only 1 of their reports has had a clear result so far.'; }
        else { lead = 'None of their reports has had a clear result yet.'; }
        return lead + ' A rating needs ' + MIN_JUDGED + ' reports with a clear result.';
      }
      return r.corroborated + ' of ' + r.judged + ' earlier reports were confirmed by people nearby.';
    },
    reporterScore: function (score) {
      return 'Score ' + score + ' out of 100, based on how often their reports with a clear result were confirmed.';
    }
  };

  // Proposed: the caveat that counts are places and not votes appears once, on the confirmed line. The disputed line
  // drops its tail (this amends A56 and RR-1). Every number stays real and every meaning stays.
  var PROPOSED = {
    key: 'proposed',
    lead: function (kind) {
      if (kind === 'expired') { return 'Expired, so it can no longer be confirmed.'; }
      return 'Needs ' + THRESHOLD + ' confirmations to count as confirmed.';
    },
    confirmed: function (n) {
      var tail = ' Counts places, not votes.';
      if (n <= 0) { return 'No one has confirmed this yet.' + tail; }
      if (n === 1) { return '1 place confirmed this.' + tail; }
      return n + ' different places confirmed this.' + tail;
    },
    disputed: function (m) {
      if (m <= 0) { return 'No one nearby has disputed this.'; }
      if (m === 1) { return '1 place disputed this.'; }
      return m + ' different places disputed this.';
    },
    currency: function (c) {
      if (c.band === 'not_yet_corroborated') { return 'Fewer than ' + THRESHOLD + ' places have confirmed this.'; }
      if (c.band === 'up_to_date') { return c.recent + ' places confirmed this in the last ' + WINDOW + '.'; }
      if (c.disputeRaised) { return 'Disputed nearby, so it needs ' + c.k + ' places in the last ' + WINDOW + '.'; }
      if (c.band === 'getting_old') { return 'Fewer than ' + THRESHOLD + ' places confirmed it in the last ' + WINDOW + '.'; }
      return 'Gone quiet. People nearby can confirm it again.';
    },
    currencyScore: function (score) {
      return 'Score ' + score + ' out of 100, based on recent confirmations.';
    },
    reporter: function (r) {
      if (r.tag === 'new') {
        var have = r.judged === 0 ? 'none' : String(r.judged);
        return 'A rating needs ' + MIN_JUDGED + ' reports with a clear result. They have ' + have + ' so far.';
      }
      return r.corroborated + ' of ' + r.judged + ' earlier reports were confirmed by people nearby.';
    },
    reporterScore: function (score) {
      return 'Score ' + score + ' out of 100, based on how often their reports were confirmed.';
    }
  };

  S3.WORDINGS = { current: CURRENT, proposed: PROPOSED };

  // The lead line kind: Unconfirmed while the report is Provisional, the expiry line once it has expired.
  function leadKind(state) {
    if (state.visibility === 'provisional') { return 'unconfirmed'; }
    if (state.visibility === 'expired') { return 'expired'; }
    return '';
  }

  // The panel lines for one state, in the fixed order. scores is true on the popup only (D-09, A50).
  S3.panelLines = function (state, wordingKey, scores) {
    var w = S3.WORDINGS[wordingKey] || PROPOSED;
    var out = [];
    var kind = leadKind(state);
    if (kind) { out.push({ key: 'lead', text: w.lead(kind) }); }
    out.push({ key: 'confirmed', text: w.confirmed(state.confirmed) });
    out.push({ key: 'disputed', text: w.disputed(state.disputed) });
    if (state.currency) {
      out.push({ key: 'currency', text: w.currency(state.currency) });
      if (scores && state.currency.score !== null) { out.push({ key: 'currency_score', text: w.currencyScore(state.currency.score) }); }
    }
    if (state.reporter) {
      out.push({ key: 'reporter', text: w.reporter(state.reporter) });
      if (scores && state.reporter.score !== null) { out.push({ key: 'reporter_score', text: w.reporterScore(state.reporter.score) }); }
    }
    return out;
  };

  // Category plus time, and for a shelter its capacity line (03-UI-SPEC, Surface: Map Popup).
  S3.metaLines = function (state) {
    var lines = [S3.CATEGORY_LABELS[state.category] + ' · ' + state.timeText];
    if (state.shelter) { lines.push(state.shelter); }
    return lines;
  };
  // The feed row and the Activity row keep one joined string.
  S3.metaText = function (state) {
    return S3.metaLines(state).join(' · ');
  };
}(window.S3));
