// slow-notice.js - the "Taking long?" notice of the main page.
//
// This is the first script in the head of the main page and it is not
// deferred, so it runs before the map libraries are requested. It must not
// depend on them or on the app store (window.Pinalert): the whole point is
// that the notice still shows when the map libraries hang or fail and the
// app scripts never run. It also never waits for the document parsed event,
// because a hung script tag delays that event.
//
// It starts a 6 second timer when the first list request is sent (app.js sets
// data-feed-requested on the root element). If the list has not loaded by
// then (feed.js sets data-feed-state to ok), the Slow row of the notice card
// shows and the live region speaks the sentence once. When no request was
// ever sent and the app scripts or the map library never arrived, a timer from
// the first run shows it the same way. The text reaches the page only through
// textContent.
(function () {
  'use strict';

  var SLOW_NOTICE_MS = 6000;
  var CARD_RETRY_MS = 250;
  var SLOW_SENTENCE = 'Taking long? Open the low data view.';

  var root = document.documentElement;
  var shown = false;

  function listLoaded() {
    return root.getAttribute('data-feed-state') === 'ok';
  }

  // The card is hidden exactly when none of its rows is visible. Every script
  // that changes a row follows this rule.
  function syncCard(card) {
    card.hidden = card.querySelector('.notice-card__row:not([hidden])') === null;
  }

  function showSlowRow() {
    if (shown || listLoaded()) {
      return;
    }
    var card = document.getElementById('notice-card');
    var row = document.getElementById('notice-slow');
    if (!card || !row) {
      // The card sits before the blocking script tags, so it is parsed even
      // when they hang; try again shortly if this ran too early.
      window.setTimeout(showSlowRow, CARD_RETRY_MS);
      return;
    }
    shown = true;
    row.hidden = false;
    syncCard(card);
    var live = document.getElementById('notice-live');
    if (live) {
      live.textContent = SLOW_SENTENCE;
    }
  }

  // First run timer: used only when no list request was sent by then and the
  // app store or the map library is missing.
  window.setTimeout(function () {
    if (root.hasAttribute('data-feed-requested')) {
      return;
    }
    if (typeof window.Pinalert === 'undefined' || typeof window.L === 'undefined') {
      showSlowRow();
    }
  }, SLOW_NOTICE_MS);

  // Request timer: starts when the first list request is sent.
  if (typeof MutationObserver !== 'undefined') {
    var observer = new MutationObserver(function () {
      if (root.hasAttribute('data-feed-requested')) {
        observer.disconnect();
        window.setTimeout(showSlowRow, SLOW_NOTICE_MS);
      }
    });
    observer.observe(root, { attributes: true, attributeFilter: ['data-feed-requested'] });
  }
}());
