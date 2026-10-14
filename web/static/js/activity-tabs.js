// activity-tabs.js: in place tab switching for the Activity page (quick task
// 261013-wm5). The six tabs are links, so they work with no script: each one is
// its own address and the server renders the selected tab. This file only makes
// a choice instant. It filters the cards already on the page, moves the
// selected mark, writes the tab into the address and brings the chosen tab into
// view inside its own strip.
//
// It computes nothing and fetches nothing. Every fact it reads was written by
// the server: each card's tab tokens, each link's key, count and address. It
// reads no date, builds no markup and writes no words, and it needs none of
// the Pinalert globals.
//
// The address is replaced and never pushed (the feed filter precedent), so a
// tab choice adds no Back steps. Back from another page, and a bookmark, land
// on the same tab because the server renders the tab named in the address.
(function () {
  'use strict';

  var nav = document.querySelector('nav.activity-tabs');
  if (!nav) {
    // Inert on any page without the tab strip.
    return;
  }

  var ALL_KEY = 'all';

  // revealTab brings one link into view by changing only the strip's own
  // horizontal scroll position. It never scrolls the page: a window scroll
  // would move the whole page under the finger, and the strip is the only
  // thing that may move here.
  function revealTab(link) {
    var strip = link.getBoundingClientRect();
    var frame = nav.getBoundingClientRect();
    if (strip.left < frame.left) {
      nav.scrollLeft += strip.left - frame.left;
    } else if (strip.right > frame.right) {
      nav.scrollLeft += strip.right - frame.right;
    }
  }

  // showTab marks one link as the current one and shows exactly the cards and
  // the empty state that belong to it.
  function showTab(link) {
    var key = link.dataset.tab;

    var links = nav.querySelectorAll('a.activity-tab');
    for (var i = 0; i < links.length; i++) {
      if (links[i] === link) {
        links[i].setAttribute('aria-current', 'page');
      } else {
        links[i].removeAttribute('aria-current');
      }
    }

    var cards = document.querySelectorAll('.profile-report-list > .report-row');
    for (var j = 0; j < cards.length; j++) {
      var tokens = (cards[j].dataset.tabs || '').split(' ');
      cards[j].hidden = !(key === ALL_KEY || tokens.indexOf(key) !== -1);
    }

    // The tab with nothing in it shows its own plain words. Every other empty
    // state stays hidden.
    var empties = document.querySelectorAll('[data-tab-empty]');
    for (var k = 0; k < empties.length; k++) {
      empties[k].hidden = !(empties[k].dataset.tabEmpty === key && link.dataset.count === '0');
    }
  }

  nav.addEventListener('click', function (evt) {
    var link = evt.target.closest('a.activity-tab');
    if (!link || !nav.contains(link)) {
      return;
    }
    // A modified or non primary click keeps the browser's own behaviour, so
    // "open in a new tab" still opens the tab's address.
    if (evt.defaultPrevented || evt.button !== 0 || evt.metaKey || evt.ctrlKey || evt.shiftKey || evt.altKey) {
      return;
    }
    evt.preventDefault();
    showTab(link);
    window.history.replaceState(null, '', link.href);
    revealTab(link);
  });

  // The server selected one tab. A phone may have clipped it out of the strip,
  // so bring it into view the same way.
  var current = nav.querySelector('a.activity-tab[aria-current="page"]');
  if (current) {
    revealTab(current);
  }
}());
