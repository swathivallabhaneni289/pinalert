// official-pins.js: the GDACS hazard layer on the Nearby map (Phase 5, plan 22,
// ROBUST-05, ROBUST-06, D-12, UI-SPEC Surface 3, research Pitfall 12).
//
// GDACS (the Global Disaster Alert and Coordination System) publishes
// automatic estimates of hazard events: earthquakes, cyclones, floods,
// volcanoes and wildfires. This script draws the current ones as official pins
// on a layer group of their own, so a reader can see a cyclone over the Bay of
// Bengal beside the crowd's reports and can never mistake one for the other.
//
// Told apart by shape and word, never by colour. A crowd pin is a round badge on
// a severity fill and a Demo pin is round with a dashed ring. The GDACS pin is a
// flat rectangle with the word GDACS and the hazard glyph, in the text colour
// (official-pins.css), inside a 44px marker box.
//
// Apart from the crowd. The layer is fed by GET <base>/official/pins and nothing
// else. It never reads the report store, never writes to the Nearby list or to
// any count, and a pin has no vote, no trust part, no reliability tag, no Flag
// and no Mute (D-12, Phase 3 D-10). A tap or Enter or Space on a pin opens a
// read only card through PinalertMap.openCard, in the same place a crowd report
// opens (the popup on a laptop, the detail sheet on a phone).
//
// Honesty (T-05-118). The route answers no pins unless the GDACS feed is fresh,
// and this script checks it again: only an answer whose state is exactly fresh
// draws anything. A stale or unavailable answer, a failed request and an answer
// that is not shaped like the route's clear the layer, so an old hazard is never
// shown as current.
//
// Trust boundary (T-05-114, T-05-115). Every feed string enters the page through
// textContent or an attribute, never as markup. The card's link is drawn only
// when it starts with https://www.gdacs.org/ and opens in a new tab with
// noopener noreferrer.
//
// Refresh. One request on load, one on a timer every 5 minutes, and one after a
// pinalert:feed-centre event when the last request is 5 minutes old or older.
// The pins are not tied to the reader's place, so the event is only a cue that
// the reader is looking at the map.
(function () {
  'use strict';

  var REFRESH_MS = 300000;
  // A request with no answer, body included, is given up after this long
  // (Phase 5 review fix WR-B01, the same limit as the report list in app.js).
  var REQUEST_LIMIT_MS = 10000;
  var FEED_CENTRE_EVENT = 'pinalert:feed-centre';
  var DEFAULT_BASE = '/api';
  var CHOSEN_Z_OFFSET = 1000;
  var GDACS_PREFIX = 'https://www.gdacs.org/';
  var MAX_NAME = 200;

  // The hazard kinds the layer knows, by GDACS's two letter type. The word and
  // the glyph class come from this table, never from the feed.
  var KINDS = {
    EQ: { word: 'Earthquake', glyph: 'earthquake' },
    TC: { word: 'Cyclone', glyph: 'cyclone' },
    FL: { word: 'Flood', glyph: 'flood' },
    VO: { word: 'Volcano', glyph: 'volcano' },
    WF: { word: 'Wildfire', glyph: 'wildfire' }
  };

  // The three levels GDACS uses, with the chip tint of the Official alerts page:
  // Red reads as extreme, Orange as severe and Green as the neutral chip.
  var LEVELS = {
    red: { word: 'Red', chip: 'extreme' },
    orange: { word: 'Orange', chip: 'severe' },
    green: { word: 'Green', chip: 'neutral' }
  };

  if (typeof L === 'undefined' || !window.PinalertMap ||
      typeof window.PinalertMap.addOverlay !== 'function' ||
      typeof window.PinalertMap.openCard !== 'function') {
    return;
  }

  var layer = null; // the layer group, made on the first draw
  var entries = {}; // pin id -> { pin, key, marker, node }
  var requestOut = false;
  var requestSeq = 0; // moves on when a request is sent and when it ends
  var openId = null; // the pin id of the open card, or null
  var lastFetchAt = 0;

  // ---- Small helpers ------------------------------------------------------

  function apiBase() {
    var base = window.PinalertApiBase;
    return typeof base === 'string' && base !== '' ? base : DEFAULT_BASE;
  }

  function own(table, key) {
    return typeof key === 'string' && Object.prototype.hasOwnProperty.call(table, key) ? table[key] : null;
  }

  function make(tag, className, text) {
    var node = document.createElement(tag);
    if (className) {
      node.setAttribute('class', className);
    }
    if (typeof text === 'string') {
      node.textContent = text;
    }
    return node;
  }

  function finite(value) {
    return typeof value === 'number' && isFinite(value);
  }

  // safeGdacsLink keeps a link only when it is a plain page of www.gdacs.org:
  // the exact https prefix, then printable ASCII with no space, quote, angle
  // bracket or backslash. Anything else gives an empty string (T-05-115).
  function safeGdacsLink(url) {
    if (typeof url !== 'string' || url.indexOf(GDACS_PREFIX) !== 0) {
      return '';
    }
    for (var i = 0; i < url.length; i += 1) {
      var code = url.charCodeAt(i);
      if (code <= 32 || code >= 127 || code === 34 || code === 39 || code === 60 || code === 62 ||
          code === 92 || code === 96) {
        return '';
      }
    }
    return url;
  }

  // ---- Times --------------------------------------------------------------

  var clockFormat = new Intl.DateTimeFormat('en-IN', { hour: 'numeric', minute: '2-digit', hour12: true });
  var dayFormat = new Intl.DateTimeFormat('en-IN', { year: 'numeric', month: 'numeric', day: 'numeric' });
  var weekdayFormat = new Intl.DateTimeFormat('en-IN', { weekday: 'short' });

  // clockText turns a time into "2:05 pm" in the reader's own zone, built from
  // the parts so a narrow no break space or an upper case am never reaches the
  // page.
  function clockText(ms) {
    var hour = '';
    var minute = '';
    var period = '';
    var parts = clockFormat.formatToParts(new Date(ms));
    for (var i = 0; i < parts.length; i += 1) {
      if (parts[i].type === 'hour') {
        hour = parts[i].value;
      } else if (parts[i].type === 'minute') {
        minute = parts[i].value;
      } else if (parts[i].type === 'dayPeriod') {
        period = parts[i].value.toLowerCase();
      }
    }
    // A no break space (U+00A0) joins the minute and am or pm, so the time never
    // breaks before pm (Phase 5 review fix WR-T02). It is the plain no break
    // space, not the narrow one Intl writes.
    return hour + ':' + minute + '\u00a0' + period;
  }

  // clockWithDay is the clock time, with the short weekday in front when the day
  // is not today ("Sun 8:35 pm").
  function clockWithDay(ms, nowMs) {
    var clock = clockText(ms);
    if (dayFormat.format(new Date(ms)) === dayFormat.format(new Date(nowMs))) {
      return clock;
    }
    return weekdayFormat.format(new Date(ms)) + ' ' + clock;
  }

  // agoText is "just now", "6 min ago", "3 hours ago", "2 days ago".
  function agoText(ms, nowMs) {
    var seconds = Math.max(0, (nowMs - ms) / 1000);
    if (seconds < 60) {
      return 'just now';
    }
    var minutes = Math.floor(seconds / 60);
    if (minutes < 60) {
      return minutes + ' min ago';
    }
    var hours = Math.floor(seconds / 3600);
    if (hours < 24) {
      return hours === 1 ? '1 hour ago' : hours + ' hours ago';
    }
    var days = Math.floor(seconds / 86400);
    return days === 1 ? '1 day ago' : days + ' days ago';
  }

  function timeOf(iso) {
    if (typeof iso !== 'string') {
      return null;
    }
    var ms = Date.parse(iso);
    return isFinite(ms) ? ms : null;
  }

  // ---- One pin ------------------------------------------------------------

  // normalise reads one pin of the answer and returns the parts the layer uses,
  // or null for a pin of a kind, a level or a place the layer does not know.
  function normalise(raw) {
    if (!raw || typeof raw !== 'object') {
      return null;
    }
    var kind = own(KINDS, raw.event_type);
    var level = own(LEVELS, raw.alert_level);
    var name = typeof raw.name === 'string' ? raw.name.trim() : '';
    if (!kind || !level || name === '' || !finite(raw.id)) {
      return null;
    }
    if (!finite(raw.lat) || !finite(raw.lon) || Math.abs(raw.lat) > 90 || Math.abs(raw.lon) > 180) {
      return null;
    }
    return {
      id: raw.id,
      kind: kind,
      level: level,
      name: name.length > MAX_NAME ? name.slice(0, MAX_NAME) : name,
      lat: raw.lat,
      lon: raw.lon,
      from: timeOf(raw.from_date),
      to: timeOf(raw.to_date),
      severity: typeof raw.severity_text === 'string' ? raw.severity_text.trim() : '',
      url: safeGdacsLink(raw.report_url)
    };
  }

  // pinName is the accessible name: "GDACS, Earthquake in Vanuatu Islands, Red".
  function pinName(pin) {
    return 'GDACS, ' + pin.name + ', ' + pin.level.word;
  }

  // pinIcon makes the Leaflet icon of one pin: the flat rectangle with the hazard
  // glyph and the word GDACS, inside a transparent 96 by 44 marker box. The box
  // size is passed to Leaflet because Leaflet writes it inline on the marker.
  function pinIcon(pin) {
    var shape = make('span', 'official-pin');
    var glyph = make('span', 'official-pin__glyph official-glyph--' + pin.kind.glyph);
    glyph.setAttribute('aria-hidden', 'true');
    shape.appendChild(glyph);
    shape.appendChild(make('span', 'official-pin__word', 'GDACS'));
    return L.divIcon({
      html: shape,
      className: 'official-pin-box',
      iconSize: [96, 44],
      iconAnchor: [48, 22],
      popupAnchor: [0, -22]
    });
  }

  // line makes one label and value line of the card. The value starts with a
  // space so the two read as two words.
  function line(label, value) {
    var node = make('p', 'official-card__line');
    node.appendChild(make('span', 'official-card__label', label));
    node.appendChild(make('span', 'official-card__value', ' ' + value));
    return node;
  }

  // buildCard makes the read only card of one pin (UI-SPEC 3b): the header band
  // (the GDACS badge, the hazard glyph, the level chip), the title, the lines,
  // the honesty line, the credit and, for a gdacs.org page, one outlined button.
  function buildCard(pin) {
    var now = Date.now();
    var card = make('div', 'official-card');
    card.setAttribute('tabindex', '-1');

    var head = make('div', 'official-card__head');
    var badge = make('span', 'official-card__badge');
    var badgeIcon = make('span', 'official-card__badge-icon');
    badgeIcon.setAttribute('aria-hidden', 'true');
    badge.appendChild(badgeIcon);
    badge.appendChild(make('span', 'official-card__badge-word', 'GDACS'));
    head.appendChild(badge);
    var glyph = make('span', 'official-card__glyph official-glyph--' + pin.kind.glyph);
    glyph.setAttribute('aria-hidden', 'true');
    head.appendChild(glyph);
    var chip = make('span', 'official-card__chip official-card__chip--' + pin.level.chip);
    var chipIcon = make('span', 'official-card__chip-icon');
    chipIcon.setAttribute('aria-hidden', 'true');
    chip.appendChild(chipIcon);
    chip.appendChild(make('span', 'official-card__chip-word', pin.level.word));
    head.appendChild(chip);
    card.appendChild(head);

    var body = make('div', 'official-card__body');
    var title = make('h3', 'official-card__title', pin.name);
    title.setAttribute('id', 'official-card-title');
    body.appendChild(title);
    body.appendChild(line('Hazard', pin.kind.word));
    if (pin.from !== null) {
      body.appendChild(make('p', 'official-card__line', 'Issued ' + agoText(pin.from, now)));
    }
    if (pin.to !== null) {
      body.appendChild(make('p', 'official-card__line', 'Until ' + clockWithDay(pin.to, now)));
    }
    body.appendChild(make('p', 'official-card__line', 'Issued by GDACS'));
    if (pin.severity !== '') {
      body.appendChild(make('p', 'official-card__detail', pin.severity));
    }
    body.appendChild(make('p', 'official-card__honesty',
      'Automatic estimate. Follow your state disaster management authority.'));
    body.appendChild(make('p', 'official-card__credit',
      'Global Disaster Alert and Coordination System, GDACS'));
    var href = safeGdacsLink(pin.url);
    if (href !== '') {
      var link = make('a', 'official-card__link', 'Open GDACS page');
      link.setAttribute('href', href);
      link.setAttribute('target', '_blank');
      link.setAttribute('rel', 'noopener noreferrer');
      body.appendChild(link);
    }
    card.appendChild(body);
    return card;
  }

  // ---- The layer ----------------------------------------------------------

  // ensureLayer makes the layer group once and hands it to the map. It answers
  // null when the map cannot take it, and nothing is drawn then.
  function ensureLayer() {
    if (layer) {
      return layer;
    }
    var group = L.layerGroup();
    if (window.PinalertMap.addOverlay(group) === false) {
      return null;
    }
    layer = group;
    return layer;
  }

  // setChosen grows the pin of the open card and raises it above the others.
  function setChosen(entry, chosen) {
    entry.node.setAttribute('class', chosen ? 'official-pin is-chosen' : 'official-pin');
    entry.marker.setZIndexOffset(chosen ? CHOSEN_Z_OFFSET : 0);
  }

  // openFor opens the card of one pin. The pin of an earlier card is released by
  // that card's own close, which the map runs before this one opens.
  function openFor(entry) {
    var pin = entry.pin;
    var element = entry.marker.getElement();
    var opened = window.PinalertMap.openCard([pin.lat, pin.lon], buildCard(pin), pinName(pin), {
      opener: element,
      onClose: function () {
        setChosen(entry, false);
        if (openId === pin.id) {
          openId = null;
        }
      }
    });
    if (opened !== false) {
      setChosen(entry, true);
      openId = pin.id;
    }
  }

  function onPinKey(entry, evt) {
    var key = evt.key;
    if (key !== 'Enter' && key !== ' ' && key !== 'Spacebar') {
      return;
    }
    evt.preventDefault();
    if (typeof evt.stopPropagation === 'function') {
      evt.stopPropagation();
    }
    openFor(entry);
  }

  // decorate gives the marker's element its name and its Enter and Space keys. It
  // runs once per element: Leaflet makes the element only when the marker is
  // added to a map that already has a view, which on a slow first fix is after
  // this script has run, so it runs again on the marker's own add event.
  function decorate(entry) {
    var element = entry.marker.getElement();
    if (!element || entry.decorated === element) {
      return;
    }
    entry.decorated = element;
    element.setAttribute('aria-label', pinName(entry.pin));
    element.addEventListener('keydown', function (evt) {
      onPinKey(entry, evt);
    });
  }

  // addPin puts one pin on the layer: a keyboard focusable marker with its name.
  function addPin(group, pin, key) {
    var icon = pinIcon(pin);
    var marker = L.marker([pin.lat, pin.lon], { icon: icon, keyboard: true });
    var entry = { pin: pin, key: key, marker: marker, node: icon.options.html, decorated: null };
    marker.on('add', function () {
      decorate(entry);
    });
    marker.on('click', function () {
      openFor(entry);
    });
    group.addLayer(marker);
    decorate(entry);
    return entry;
  }

  // closeOpenCard closes the open card through the map (Phase 5 review fix
  // WR-B02): the card is a popup apart from the markers, so it must go with its
  // pin, or it would stay up as if the hazard were current.
  function closeOpenCard() {
    if (openId === null) {
      return;
    }
    openId = null;
    if (typeof window.PinalertMap.closeCard === 'function') {
      window.PinalertMap.closeCard();
    }
  }

  function clearAll() {
    closeOpenCard();
    if (layer) {
      layer.clearLayers();
    }
    entries = {};
  }

  // drawPins draws one answer and says how many pins it drew. Only an answer
  // whose state is exactly fresh draws; anything else clears the layer. A pin
  // that has not changed keeps its marker, so keyboard focus on it is kept.
  function drawPins(resp) {
    var fresh = resp && typeof resp === 'object' && resp.state === 'fresh' && Array.isArray(resp.pins);
    if (!fresh) {
      clearAll();
      return 0;
    }
    var group = ensureLayer();
    if (!group) {
      return 0;
    }
    var seen = {};
    var count = 0;
    for (var i = 0; i < resp.pins.length; i += 1) {
      var pin = normalise(resp.pins[i]);
      if (!pin || seen[pin.id] === true) {
        continue;
      }
      seen[pin.id] = true;
      var key = JSON.stringify([pin.kind.glyph, pin.level.word, pin.name, pin.lat, pin.lon, pin.from, pin.to,
        pin.severity, pin.url]);
      var existing = entries[pin.id];
      if (existing && existing.key === key) {
        count += 1;
        continue;
      }
      if (existing) {
        if (openId === pin.id) {
          closeOpenCard();
        }
        group.removeLayer(existing.marker);
      }
      entries[pin.id] = addPin(group, pin, key);
      count += 1;
    }
    var ids = Object.keys(entries);
    for (var j = 0; j < ids.length; j += 1) {
      if (seen[ids[j]] !== true) {
        if (openId !== null && String(openId) === ids[j]) {
          closeOpenCard();
        }
        group.removeLayer(entries[ids[j]].marker);
        delete entries[ids[j]];
      }
    }
    return count;
  }

  // loadPins makes the one request and draws the answer. A failed request or an
  // HTTP error clears the layer: with no answer, nothing is known to be current.
  //
  // Each request has a time limit (Phase 5 review fix WR-B01). The limit is the
  // page's own timer, never the request's promise: WebKit can leave a request's
  // promise unsettled, and a request that never ends must not keep the pins
  // drawn as current or block every later load. At the limit the request is
  // aborted, the layer is cleared and requestOut is let go; a late answer of a
  // request that was given up is dropped.
  function loadPins() {
    if (requestOut || typeof fetch !== 'function') {
      return;
    }
    requestOut = true;
    lastFetchAt = Date.now();
    requestSeq += 1;
    var mine = requestSeq;
    var controller = typeof AbortController === 'function' ? new AbortController() : null;
    var timer = null;
    var expired = new Promise(function (resolve, reject) {
      timer = setTimeout(function () {
        timer = null;
        reject(new Error('official pins: no answer in time'));
        if (controller) {
          controller.abort();
        }
      }, REQUEST_LIMIT_MS);
    });
    expired.catch(function () {});
    var init = { credentials: 'same-origin', headers: { Accept: 'application/json' } };
    if (controller) {
      init.signal = controller.signal;
    }
    var answered = fetch(apiBase() + '/official/pins', init).then(function (res) {
      if (!res.ok) {
        throw new Error('status ' + res.status);
      }
      return Promise.race([res.json(), expired]);
    });
    Promise.race([answered, expired]).then(function (body) {
      if (mine !== requestSeq) {
        return;
      }
      settled(timer);
      drawPins(body);
    }, function () {
      if (mine !== requestSeq) {
        return;
      }
      settled(timer);
      clearAll();
    });
  }

  // settled ends one request: its timer stops and the next load may go out.
  function settled(timer) {
    if (timer !== null) {
      clearTimeout(timer);
    }
    requestSeq += 1;
    requestOut = false;
  }

  document.addEventListener(FEED_CENTRE_EVENT, function () {
    if (Date.now() - lastFetchAt >= REFRESH_MS) {
      loadPins();
    }
  });
  setInterval(loadPins, REFRESH_MS);

  window.PinalertOfficialPins = {
    loadPins: loadPins,
    drawPins: drawPins,
    pinIcon: pinIcon,
    buildCard: buildCard,
    safeGdacsLink: safeGdacsLink
  };

  loadPins();
}());
