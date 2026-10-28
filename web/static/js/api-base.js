// api-base.js: where the page's report calls go, and whether this is the Demo.
//
// The Demo (plan 05-13, D-05) runs the real map, list and vote scripts on a
// sandbox copy of the report API. The one difference those scripts see is the
// base of their report URLs: window.PinalertApiBase, read by app.js, votes.js
// and ended-nearby.js, which each fall back to '/api' when it is not set. On the
// real pages this file is not loaded at all, so nothing changes there.
//
// The base is a path on this site that the server wrote into the page
// (data-api-base on the app shell or on main). It is checked before it is used:
// only a path that starts with a single slash and holds letters, digits, slash,
// underscore and hyphen is taken, so a value that names another site, a script
// address or a path with dot segments leaves the default in place (T-05-74).
//
// window.PinalertDemoPage is true when the page says data-demo="1". The row, the
// popup and the pin read it to draw the Demo marks.
//
// This file reads the page and sets two window values. It stores nothing in the
// browser and never asks for a location.
(function () {
  'use strict';

  var SAFE_BASE = /^\/(?!\/)[A-Za-z0-9\/_-]*$/;

  function carrier() {
    var shell = document.getElementById('app-shell');
    if (shell && shell.getAttribute('data-api-base') !== null) {
      return shell;
    }
    return document.querySelector('main[data-api-base]');
  }

  function init() {
    var el = carrier();
    if (!el) {
      return;
    }
    var base = el.getAttribute('data-api-base');
    if (typeof base === 'string' && SAFE_BASE.test(base)) {
      window.PinalertApiBase = base.replace(/\/+$/, '');
    }
    if (el.getAttribute('data-demo') === '1') {
      window.PinalertDemoPage = true;
    }
  }

  init();
}());
