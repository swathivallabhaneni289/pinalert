// sw.js - the Pinalert service worker (Phase 3.3, plan 04).
//
// The worker shows what the server wrote and nothing else. Every push shows
// exactly one notification: Chrome shows its own generic notice when a push
// shows nothing, and Safari withdraws the push permission of a site that does
// not show every push. So all choosing of who gets what stays on the server and
// a bad or empty payload still shows a plain Pinalert notice.
//
// The worker asks the platform for no noise on every kind of notification: it
// builds each one with silent true and never sets a vibration pattern or a
// sound field (silent true together with a vibration pattern is a TypeError in
// the browser). The iPhone may still follow its own settings, which is
// documented in docs/PUSH.md.
//
// It holds no cache and has no network request handler, so it never sits in
// front of the login redirects, the asset versioning or the offline queue. The
// page registers it at /sw.js with no query string, so a cold start does not
// look like a new worker; the browser compares the bytes on each update check.
//
// This is a plain script, not a module: a worker has its own global scope. The
// pure helpers safeTarget, normalise and optionsFor are top level functions so
// the node test can reach them. Payload text goes only into notification
// fields, never into markup.

const SUBSCRIPTION_URL = '/api/me/push/subscription';
const ICON_URL = '/icons/app-192.png';
const BADGE_URL = '/icons/badge-96.png';

const FALLBACK_TITLE = 'Pinalert';
const FALLBACK_BODY = 'Open Pinalert for a new alert near your home area.';
const FALLBACK_TAG = 'pinalert';

// safeTarget returns the address a tap may open: /?report= followed by 1 to 18
// digits, or / for anything else. A value from a push is never trusted.
function safeTarget(u) {
  return typeof u === 'string' && /^\/\?report=\d{1,18}$/.test(u) ? u : '/';
}

// normalise turns a decoded payload into the fields of one notification. A
// payload that is not an object with a text title and a text body becomes the
// plain Pinalert notice.
function normalise(m) {
  if (!m || typeof m !== 'object' || typeof m.title !== 'string' || typeof m.body !== 'string' || m.title === '' || m.body === '') {
    return { title: FALLBACK_TITLE, body: FALLBACK_BODY, tag: FALLBACK_TAG, url: '/' };
  }
  return {
    title: m.title,
    body: m.body,
    tag: typeof m.tag === 'string' && m.tag !== '' ? m.tag : FALLBACK_TAG,
    url: safeTarget(m.url)
  };
}

// optionsFor builds the options object of showNotification: exactly eight keys,
// no vibration pattern and no sound field. The same tag replaces an earlier
// notification for the same report without alerting again (renotify false).
function optionsFor(n) {
  return {
    body: n.body,
    tag: n.tag,
    renotify: false,
    silent: true,
    icon: ICON_URL,
    badge: BADGE_URL,
    lang: 'en',
    data: { url: n.url }
  };
}

self.addEventListener('install', function (event) {
  // The worker holds no cache, so a new copy can take over at once.
  event.waitUntil(self.skipWaiting());
});

self.addEventListener('activate', function (event) {
  event.waitUntil(self.clients.claim());
});

self.addEventListener('push', function (event) {
  var message = null;
  try {
    message = event.data ? event.data.json() : null;
  } catch (e) {
    message = null;
  }
  var n = normalise(message);
  event.waitUntil(self.registration.showNotification(n.title, optionsFor(n)));
});

// sameOrigin says whether a window client shows a page of this site.
function sameOrigin(client) {
  var origin = self.location.origin;
  return typeof client.url === 'string' && (client.url === origin || client.url.indexOf(origin + '/') === 0);
}

self.addEventListener('notificationclick', function (event) {
  event.notification.close();
  var target = safeTarget(event.notification.data && event.notification.data.url);
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(function (list) {
      var open = null;
      for (var i = 0; i < list.length; i++) {
        if (sameOrigin(list[i])) {
          open = list[i];
          break;
        }
      }
      if (!open) return self.clients.openWindow(target);
      return open.focus().then(function () {
        return open.navigate(target);
      }).catch(function () {
        // A window the worker cannot navigate gets a new one instead.
        return self.clients.openWindow(target);
      });
    })
  );
});

// The browser replaced the push subscription: subscribe again with the same
// server key and tell the server. The page also reconciles on Settings load,
// because this event is not reliable in every browser.
self.addEventListener('pushsubscriptionchange', function (event) {
  var old = event.oldSubscription;
  if (!old || !old.options || !old.options.applicationServerKey) return;
  event.waitUntil(
    self.registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: old.options.applicationServerKey
    }).then(function (sub) {
      return self.fetch(SUBSCRIPTION_URL, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(sub)
      });
    }).catch(function () {
      // Settings load reconciles the subscription later.
    })
  );
});
