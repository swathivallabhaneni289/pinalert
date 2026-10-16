// queue.js - window.PinalertQueue, the device queue for reports that could not
// be sent (D-18, D-19, D-20).
//
// A report posted with no answer, a timeout or a 502, 503 or 504 is saved here
// and sent again later through Pinalert.submitReport, the one send path. Votes,
// vote changes and flags are never saved. The queue is decided by real request
// outcomes only. A saved report never ends because of how long it has waited;
// it leaves when it is sent or when the person discards it.
//
// Every row is bound to the account id the server rendered on the page shell
// (T-04-14): another account's rows are never drawn, counted, announced, opened
// or sent. The id is digits only and is never the email.
//
// SECURITY (T-04-01): report text and every server message reach the DOM
// through Pinalert.setText. The tap to call link is built with DOM calls from
// the number in the 429 body.
//
// On a page without a report form (the Activity page) the page passes a link
// builder and a focus fallback. The Rejected row's edit control then becomes a
// link to the page that has the form, and only a single slash path is drawn.
//
// Nothing runs at load time except declaring constants and functions.
// Everything starts in mount.
(function () {
  'use strict';

  var DB_NAME = 'pinalert-queue';
  var DB_VERSION = 1;
  var STORE_NAME = 'reports';
  var ACCOUNT_INDEX = 'account_id';
  var LOCK_NAME = 'pinalert-queue-flush';
  var CHANNEL_NAME = 'pinalert-queue';
  var QUEUE_FLUSH_MS = 30000;
  var WAIT_TICK_MS = 60000;
  var SENDING_LEASE_MS = 15000;
  // The longest wait between two wakes that sent nothing, and between two
  // tries to open a store that cannot be read (WR-07).
  var MAX_BACKOFF_MS = 60000;
  var MIN_BLOCKED_WAIT_S = 5;
  var LONG_TOAST_MS = 6000;
  var COUNT_EVENT = 'pinalert-queue-count';
  var REQUEST_ID_PATTERN = /^[A-Za-z0-9-]{16,64}$/;

  // Sending is derived from sending_until and is never stored as a status.
  var STATUS_WAITING = 'waiting';
  var STATUS_NO_ANSWER = 'no_answer';
  var STATUS_REJECTED = 'rejected';
  var STATUS_BLOCKED = 'blocked';
  var STATUS_SIGNED_OUT = 'signed_out';
  // The server refused the post because another account is signed in now
  // than the one the report was saved under (WR-W09).
  var STATUS_OTHER_ACCOUNT = 'other_account';
  var STATUSES = [STATUS_WAITING, STATUS_NO_ANSWER, STATUS_REJECTED, STATUS_BLOCKED, STATUS_SIGNED_OUT,
    STATUS_OTHER_ACCOUNT];
  // The 409 field the server names for that refusal.
  var OTHER_ACCOUNT_FIELD = 'expected_account_id';
  // How a discard ended (discardOutcome).
  var DISCARD_REMOVED = 'removed';
  var DISCARD_FAILED = 'failed';
  var DISCARD_SENDING = 'sending';
  var DISCARD_SKIPPED = 'skipped';

  // The copy table. Plain characters only.
  var QUEUE_HEADING = 'Waiting to send';
  var SEND_NOW_LABEL = 'Send now';
  var SENDING_LABEL = 'Sending';
  var COUNT_ONE_SUFFIX = ' report saved on this device.';
  var COUNT_MANY_SUFFIX = ' reports saved on this device.';
  var HELP_LINE = 'Reports send by themselves when you have a signal and this page is open.';
  var HELP_LINE_MEMORY = "This browser can't keep reports for later. Keep this page open until they send.";
  var HELP_LINE_UNREADABLE = "This page can't read the saved reports right now. Reload the page to send or discard them.";
  var STATUS_TEXT_WAITING = 'Waiting for a signal.';
  var STATUS_TEXT_SENDING = 'Sending.';
  var STATUS_TEXT_NO_ANSWER = "Pinalert didn't answer. Trying again.";
  var NOT_SENT_PREFIX = 'Not sent. ';
  var BLOCKED_PREFIX = 'Waiting to send. It will send by itself in ';
  var EMERGENCY_CALL_PREFIX = 'In an emergency, call ';
  var EMERGENCY_CALL_SUFFIX = ' now.';
  var CALL_LABEL_PREFIX = 'Call ';
  var STATUS_TEXT_SIGNED_OUT = 'Sign in again to send this report. It stays saved on this device.';
  var SIGN_IN_LABEL = 'Sign in again';
  var EDIT_LABEL = 'Edit and resend';
  var DISCARD_LABEL = 'Discard report';
  var KEEP_LABEL = 'Keep saved report';
  var CONFIRM_DISCARD_LABEL = 'Yes, discard report';
  var DISCARD_QUESTION = "Discard this saved report? It won't be sent.";
  var SENT_TOAST = 'Report sent.';
  var HELD_TOAST_MESSAGE = "Report received. It isn't shown yet. A text check has to finish first.";
  var DISCARDED_TOAST = 'Saved report discarded.';
  var DISCARD_FAILED_TOAST = "This saved report couldn't be discarded. Try again.";
  var DISCARD_SENDING_TOAST = "This report is being sent right now, so it can't be discarded.";
  var GENERIC_REASON = 'Something went wrong. Try again.';
  var ANNOUNCE_ONE_SUFFIX = ' report waiting to send.';
  var ANNOUNCE_MANY_SUFFIX = ' reports waiting to send.';
  var ANNOUNCE_NOT_SENT_PREFIX = 'Report not sent. ';
  var ANNOUNCE_BLOCKED_PREFIX = 'Report waiting to send. It will send by itself in ';
  var ANNOUNCE_SIGNED_OUT = 'Report waiting. Sign in again to send it.';
  var STATUS_TEXT_OTHER_ACCOUNT = 'This report was saved under a different account than the one signed in now. Sign in to that account to send it. It stays saved on this device.';
  var ANNOUNCE_OTHER_ACCOUNT = 'Report not sent. It was saved under a different account.';

  var mounted = false;
  var accountId = '';
  var stripEl = null;
  var headingLevel = 2;
  var onEdit = null;
  var resendHref = null;
  var focusFallback = null;
  var db = null;
  var storeReady = Promise.resolve();
  var memoryOnly = false;
  // storeOpened is true once IndexedDB opened at least once, so a lost
  // connection is opened again rather than read as "no saved reports".
  // storeUnreadable is true while the last read failed even after that; the
  // strip then keeps the rows it last read (lastStored) and sends none of them.
  // pendingRemovals holds request ids whose delete has not reached the store
  // yet, so a discarded or sent row never comes back from lastStored (WR-W06).
  // A delete that reaches the store takes its rows out of lastStored as well
  // (WR-08).
  var storeOpened = false;
  var storeUnreadable = false;
  var reopenPromise = null;
  var lastStored = [];
  var pendingRemovals = {};
  var memoryRows = {};
  var rows = [];
  var rowEls = {};
  var flushPromise = null;
  var passRunning = false;
  var wakeTimer = null;
  // wakeBackoffMs is the wait before the next wake for a row that is due but
  // was not sent (0 while no such row). reopenWaitMs and reopenNotBefore hold
  // back an automatic pass while the store cannot be read, so the store is not
  // opened again on every pass; a good read clears both (WR-07).
  var wakeBackoffMs = 0;
  var reopenWaitMs = 0;
  var reopenNotBefore = 0;
  var sessionLapsed = false;
  var persistAsked = false;
  var questionSeq = 0;

  // The strip frame, built once by ensureFrame.
  var headEl = null;
  var headingEl = null;
  var sendBtn = null;
  var countEl = null;
  var helpEl = null;
  var listEl = null;

  // ----- storage -----

  function openStore() {
    return new Promise(function (resolve) {
      var request;
      // The feature test sits inside the try too: in some frames merely
      // reading the indexedDB global throws, and the page then keeps its
      // reports in memory instead of losing the strip (IN-W04).
      try {
        if (typeof indexedDB === 'undefined' || !indexedDB) {
          resolve(null);
          return;
        }
        request = indexedDB.open(DB_NAME, DB_VERSION);
      } catch (e) {
        resolve(null);
        return;
      }
      request.onupgradeneeded = function () {
        var database = request.result;
        if (!database.objectStoreNames.contains(STORE_NAME)) {
          var store = database.createObjectStore(STORE_NAME, { keyPath: 'request_id' });
          store.createIndex(ACCOUNT_INDEX, 'account_id', { unique: false });
        }
      };
      request.onsuccess = function () {
        var database = request.result;
        // A connection the browser closes (a tab sent to the background, a
        // clear of site data) or one closed here for a version change is
        // dropped, so the next storage call opens a new one.
        database.onversionchange = function () {
          database.close();
          if (db === database) {
            db = null;
          }
        };
        database.onclose = function () {
          if (db === database) {
            db = null;
          }
        };
        resolve(database);
      };
      request.onerror = function () {
        resolve(null);
      };
      request.onblocked = function () {
        resolve(null);
      };
    });
  }

  function belongsToAccount(record) {
    return !!record && accountId !== '' && record.account_id === accountId;
  }

  // ensureStore opens IndexedDB again when a connection that once worked was
  // lost. Only one open runs at a time; every caller shares it.
  function ensureStore() {
    if (db !== null || !storeOpened) {
      return Promise.resolve(db);
    }
    if (reopenPromise === null) {
      reopenPromise = openStore().then(function (database) {
        reopenPromise = null;
        if (database !== null && db === null) {
          db = database;
        }
        return db;
      });
    }
    return reopenPromise;
  }

  // readStoredOnce resolves this account's stored rows, [] when this browser
  // never had a store, and null when the read failed (unknown, not empty).
  function readStoredOnce() {
    return new Promise(function (resolve) {
      if (db === null) {
        resolve(storeOpened ? null : []);
        return;
      }
      var used = db;
      var failed = function () {
        if (db === used) {
          db = null;
        }
        resolve(null);
      };
      try {
        var tx = used.transaction(STORE_NAME, 'readonly');
        var request = tx.objectStore(STORE_NAME).index(ACCOUNT_INDEX).getAll(IDBKeyRange.only(accountId));
        request.onsuccess = function () {
          resolve(request.result || []);
        };
        request.onerror = failed;
      } catch (e) {
        failed();
      }
    });
  }

  // readStored reads once, and after a failed read drops the connection,
  // opens a new one and reads again. It resolves null when both fail.
  function readStored() {
    return ensureStore().then(readStoredOnce).then(function (list) {
      if (list !== null) {
        return list;
      }
      return ensureStore().then(readStoredOnce);
    });
  }

  // loadRows reads this account's rows (the stored ones and any kept in
  // memory), oldest written first, into rows. When the store cannot be read
  // the rows it last read stay, so they neither leave the strip nor look sent.
  function loadRows() {
    return readStored().then(function (stored) {
      if (stored === null) {
        storeUnreadable = true;
        stored = lastStored;
      } else {
        storeUnreadable = false;
        lastStored = stored;
        reopenWaitMs = 0;
        reopenNotBefore = 0;
      }
      var byId = {};
      stored.forEach(function (record) {
        if (!pendingRemovals[record.request_id]) {
          byId[record.request_id] = record;
        }
      });
      Object.keys(memoryRows).forEach(function (id) {
        byId[id] = memoryRows[id];
      });
      rows = Object.keys(byId).map(function (id) {
        return byId[id];
      }).filter(function (record) {
        return belongsToAccount(record);
      }).sort(function (a, b) {
        // Oldest written first, then by request id so the order is stable.
        if (a.written_at !== b.written_at) {
          return a.written_at < b.written_at ? -1 : 1;
        }
        if (a.request_id === b.request_id) {
          return 0;
        }
        return a.request_id < b.request_id ? -1 : 1;
      });
      return rows;
    });
  }

  // putRow writes one row. It resolves true when the row reached IndexedDB and
  // false when it is kept in memory only.
  function putRow(record) {
    // A row written again is wanted, even if an earlier delete of the same
    // request id has not reached the store.
    delete pendingRemovals[record.request_id];
    // A connection that died without telling the page throws on its first
    // use. It is dropped and the write tried once more on a new connection,
    // so a lost connection never marks the browser as unable to keep reports.
    return ensureStore().then(function () {
      return writeOnce(record);
    }).then(function (result) {
      if (result !== 'dead') {
        return result;
      }
      return ensureStore().then(function () {
        return writeOnce(record);
      });
    }).then(function (result) {
      return result === true;
    });
  }

  // writeOnce resolves true when the row reached the store, false when it is
  // kept in memory, and 'dead' when the connection threw on use.
  function writeOnce(record) {
    return new Promise(function (resolve) {
      var keepInMemory = function () {
        memoryRows[record.request_id] = record;
        memoryOnly = true;
        resolve(false);
      };
      if (db === null) {
        memoryRows[record.request_id] = record;
        resolve(false);
        return;
      }
      var used = db;
      var tx;
      try {
        tx = used.transaction(STORE_NAME, 'readwrite');
      } catch (e) {
        if (db === used) {
          db = null;
        }
        memoryRows[record.request_id] = record;
        resolve('dead');
        return;
      }
      try {
        tx.objectStore(STORE_NAME).put(record);
        tx.oncomplete = function () {
          delete memoryRows[record.request_id];
          resolve(true);
        };
        tx.onerror = keepInMemory;
        tx.onabort = keepInMemory;
      } catch (e) {
        keepInMemory();
      }
    });
  }

  // removeRow is the only function that deletes a stored row. It resolves true
  // when the row is gone: its delete reached the store, or this browser never
  // had a store and the row lived in memory only. It resolves false when the
  // delete could not reach the store (WR-03). Such a delete stays pending: the
  // row is hidden and never sent from this page, and the next removeRow
  // deletes it together with its own row. Like putRow, a connection that died
  // without telling the page is dropped and the delete tried once more on a
  // new one.
  function removeRow(requestId) {
    delete memoryRows[requestId];
    if (!storeOpened) {
      return Promise.resolve(true);
    }
    pendingRemovals[requestId] = true;
    // attempt resolves true when the delete reached the store, false when it
    // did not, and 'dead' when the connection threw on use.
    var attempt = function () {
      return new Promise(function (resolve) {
        if (!pendingRemovals[requestId]) {
          // Another removeRow deleted this row meanwhile.
          resolve(true);
          return;
        }
        if (db === null) {
          resolve(false);
          return;
        }
        var used = db;
        var ids = Object.keys(pendingRemovals);
        var tx;
        try {
          tx = used.transaction(STORE_NAME, 'readwrite');
        } catch (e) {
          if (db === used) {
            db = null;
          }
          resolve('dead');
          return;
        }
        try {
          var store = tx.objectStore(STORE_NAME);
          ids.forEach(function (id) {
            store.delete(id);
          });
          tx.oncomplete = function () {
            ids.forEach(function (id) {
              delete pendingRemovals[id];
            });
            // The deleted rows leave the last good read too, so a read that
            // fails right after a send never draws a sent report again (WR-08).
            lastStored = lastStored.filter(function (stored) {
              return ids.indexOf(stored.request_id) === -1;
            });
            resolve(true);
          };
          tx.onerror = function () {
            resolve(false);
          };
          tx.onabort = function () {
            resolve(false);
          };
        } catch (e) {
          resolve(false);
        }
      });
    };
    return ensureStore().then(attempt).then(function (result) {
      if (result !== 'dead') {
        return result;
      }
      return ensureStore().then(attempt);
    }).then(function (result) {
      return result === true;
    });
  }

  function findRow(requestId) {
    for (var i = 0; i < rows.length; i++) {
      if (rows[i].request_id === requestId) {
        return rows[i];
      }
    }
    return null;
  }

  // ----- the public surface -----

  function mount(options) {
    if (mounted) {
      return;
    }
    var id = String((options && options.accountId) || '');
    if (!/^[0-9]+$/.test(id) || !options || !options.strip) {
      return;
    }
    mounted = true;
    accountId = id;
    stripEl = options.strip;
    headingLevel = options.headingLevel === 3 ? 3 : 2;
    onEdit = typeof options.onEdit === 'function' ? options.onEdit : null;
    resendHref = typeof options.resendHref === 'function' ? options.resendHref : null;
    focusFallback = typeof options.focusFallback === 'function' ? options.focusFallback : null;

    storeReady = openStore().then(function (database) {
      db = database;
      if (db === null) {
        memoryOnly = true;
      } else {
        storeOpened = true;
      }
      return loadRows();
    }).then(function (list) {
      // A page that passed the gate proves the session for this account, so a
      // row that was waiting for a sign in, or refused because another account
      // was signed in, is tried again.
      var writes = [];
      list.forEach(function (record) {
        if (record.status === STATUS_SIGNED_OUT || record.status === STATUS_OTHER_ACCOUNT) {
          record.status = STATUS_WAITING;
          writes.push(putRow(record));
        }
      });
      return Promise.all(writes);
    });

    storeReady.then(function () {
      if (rows.length > 0) {
        requestPersist();
      }
      renderStrip();
      if (typeof onEdit === 'function') {
        openResendFromAddress();
      }
      flush();
      scheduleWake();
    });

    window.addEventListener('online', flush);
    window.setInterval(flush, QUEUE_FLUSH_MS);
    window.setInterval(refreshWaits, WAIT_TICK_MS);

    if (typeof BroadcastChannel === 'function') {
      try {
        var channel = new BroadcastChannel(CHANNEL_NAME);
        channel.onmessage = function (evt) {
          if (evt && evt.data && evt.data.account_id !== accountId) {
            sessionLapsed = true;
            renderStrip();
          }
        };
        channel.postMessage({ account_id: accountId });
      } catch (e) {
        // A browser that refuses the channel keeps working on its own.
      }
    }
  }

  function save(record) {
    if (!mounted) {
      return Promise.reject(new Error('The queue is not mounted.'));
    }
    var requestId = record && record.request_id;
    if (typeof requestId !== 'string' || !REQUEST_ID_PATTERN.test(requestId) ||
        !record.payload || typeof record.payload !== 'object') {
      return Promise.reject(new Error('The saved report is not valid.'));
    }
    return storeReady.then(function () {
      var payload = JSON.parse(JSON.stringify(record.payload));
      payload.request_id = requestId;
      var row = {
        request_id: requestId,
        account_id: accountId,
        payload: payload,
        written_at: typeof record.written_at === 'string' ? record.written_at : new Date().toISOString(),
        status: STATUSES.indexOf(record.status) !== -1 ? record.status : STATUS_WAITING,
        attempts: typeof record.attempts === 'number' ? record.attempts : 0,
        next_try_at: typeof record.next_try_at === 'number' ? record.next_try_at : 0,
        last_error: typeof record.last_error === 'string' ? record.last_error : '',
        serious: record.serious === true,
        emergency_number: typeof record.emergency_number === 'string' ? record.emergency_number : '',
        sending_until: 0
      };
      return putRow(row).then(function (persisted) {
        return loadRows().then(function () {
          requestPersist();
          renderStrip();
          scheduleWake();
          if (row.status === STATUS_BLOCKED) {
            announce(blockedAnnouncement(row));
          } else {
            announce(rows.length + (rows.length === 1 ? ANNOUNCE_ONE_SUFFIX : ANNOUNCE_MANY_SUFFIX));
          }
          return persisted;
        });
      });
    });
  }

  function flush() {
    return runFlush(false);
  }

  // discard resolves true when the row was removed and false when it was not.
  function discard(requestId) {
    return discardOutcome(requestId).then(function (outcome) {
      return outcome === DISCARD_REMOVED;
    });
  }

  // discardOutcome removes one row and resolves how that went:
  // DISCARD_REMOVED, DISCARD_FAILED when the delete could not reach the store
  // (WR-03), DISCARD_SENDING for a row whose lease is live (WR-04), or
  // DISCARD_SKIPPED for no such row, or a stored row the page
  // cannot read right now, whose delete would come back on a reload (WR-W06).
  // A row that is not removed stays on the strip and can still be sent: the
  // person is told it was kept, so it is neither hidden nor deleted later.
  function discardOutcome(requestId) {
    return storeReady.then(function () {
      return loadRows();
    }).then(function () {
      var record = findRow(requestId);
      if (!record || !belongsToAccount(record) || isStale(record)) {
        renderStrip();
        return DISCARD_SKIPPED;
      }
      // A row another tab, or a pass of this one, is sending right now may be
      // posted whatever happens here, so it is never discarded (WR-04).
      if (isLeased(record)) {
        renderStrip();
        return DISCARD_SENDING;
      }
      var memoryCopy = Object.prototype.hasOwnProperty.call(memoryRows, requestId) ? memoryRows[requestId] : null;
      return removeRow(requestId).then(function (removed) {
        if (!removed) {
          delete pendingRemovals[requestId];
          if (memoryCopy !== null) {
            memoryRows[requestId] = memoryCopy;
          }
        }
        return loadRows().then(function () {
          renderStrip();
          scheduleWake();
          return removed ? DISCARD_REMOVED : DISCARD_FAILED;
        });
      });
    });
  }

  function get(requestId) {
    return storeReady.then(function () {
      return loadRows();
    }).then(function (list) {
      var record = null;
      list.forEach(function (candidate) {
        if (candidate.request_id === requestId && belongsToAccount(candidate)) {
          record = candidate;
        }
      });
      return record;
    });
  }

  // ----- sending -----

  function runFlush(manual) {
    if (!mounted || sessionLapsed) {
      return Promise.resolve();
    }
    if (flushPromise) {
      return flushPromise;
    }
    // While the store cannot be read, an automatic pass (the timer, the
    // interval, the online event) waits before it opens the store again: 1
    // second after the first failed pass, then 2, 4 and so on, up to a minute.
    // A save, a discard and Send now still try at once (WR-07).
    if (manual !== true && storeUnreadable && Date.now() < reopenNotBefore) {
      return Promise.resolve();
    }
    var pass = function () {
      return storeReady.then(function () {
        return loadRows();
      }).then(function (list) {
        var now = Date.now();
        var due = list.filter(function (record) {
          if (record.sending_until > now || isStale(record) || questionOpen(record.request_id)) {
            return false;
          }
          if (record.status === STATUS_WAITING || record.status === STATUS_NO_ANSWER) {
            return true;
          }
          if (record.status === STATUS_BLOCKED) {
            return record.next_try_at <= now;
          }
          if (record.status === STATUS_REJECTED || record.status === STATUS_SIGNED_OUT) {
            return manual === true;
          }
          return false;
        });
        if (due.length === 0) {
          return null;
        }
        passRunning = true;
        renderStrip();
        var step = function (index) {
          if (index >= due.length || sessionLapsed) {
            return Promise.resolve();
          }
          return sendRow(due[index]).then(function (outcome) {
            if (outcome === 'stop') {
              return null;
            }
            return step(index + 1);
          });
        };
        return step(0);
      });
    };
    var run;
    if (typeof navigator !== 'undefined' && navigator.locks && typeof navigator.locks.request === 'function') {
      run = navigator.locks.request(LOCK_NAME, { ifAvailable: true }, function (lock) {
        if (lock === null) {
          return null;
        }
        return pass();
      });
    } else {
      run = pass();
    }
    var done = function () {
      flushPromise = null;
      passRunning = false;
      if (storeUnreadable) {
        reopenWaitMs = Math.min(MAX_BACKOFF_MS, reopenWaitMs > 0 ? 2 * reopenWaitMs : 1000);
        reopenNotBefore = Date.now() + reopenWaitMs;
      }
      renderStrip();
      scheduleWake();
    };
    flushPromise = run.then(done, done);
    return flushPromise;
  }

  // sendRow sends one saved report through Pinalert.submitReport and resolves
  // 'sent', 'stop' (no answer or signed out, so the pass ends) or 'kept'.
  function sendRow(record) {
    if (!belongsToAccount(record)) {
      return Promise.resolve('skip');
    }
    // Read the row again, so a row another tab sent, discarded or leased in the
    // meantime is not sent from a stale copy.
    return loadRows().then(function () {
      var current = findRow(record.request_id);
      // A row whose Discard question is open waits until the person answers it
      // (WR-W07); a pass that picked it before the question opened skips it.
      if (!current || !belongsToAccount(current) || isLeased(current) || isStale(current) ||
          questionOpen(current.request_id)) {
        return 'skip';
      }
      current.sending_until = Date.now() + SENDING_LEASE_MS;
      current.attempts = (current.attempts || 0) + 1;
      current.payload.request_id = current.request_id;
      // The post names the account the report was saved under, so the server
      // refuses it when another account is signed in now (WR-W09). It is the
      // digits only id the page already carries and nothing more. It is added
      // to a copy, so the saved payload and the form that edits it never hold it.
      var body = JSON.parse(JSON.stringify(current.payload));
      body.expected_account_id = current.account_id;
      return putRow(current).then(function () {
        renderStrip();
        return Pinalert.submitReport(body).then(function (report) {
          return removeRow(current.request_id).then(function () {
            if (report && report.held === true) {
              Pinalert.showToast(HELD_TOAST_MESSAGE, null, LONG_TOAST_MS);
            } else {
              Pinalert.showToast(SENT_TOAST);
            }
            announce(SENT_TOAST);
            return loadRows();
          }).then(function () {
            renderStrip();
            return 'sent';
          });
        }, function (err) {
          return classify(current, err);
        });
      });
    });
  }

  // classify turns a failed send into the row's next state and resolves
  // 'stop' or 'kept'.
  function classify(record, err) {
    record.sending_until = 0;
    var status = err && err.status;
    var outcome = 'kept';
    var held = '';
    if (err && err.queueable === true) {
      record.status = (status === 502 || status === 503 || status === 504) ? STATUS_NO_ANSWER : STATUS_WAITING;
      outcome = 'stop';
    } else if (status === 429) {
      var seconds = typeof err.retryAfterSeconds === 'number' ? err.retryAfterSeconds : 60;
      // A wait of 0, or one that cannot be read, would resend every second
      // for as long as the server keeps refusing (IN-W07).
      if (!isFinite(seconds) || seconds < MIN_BLOCKED_WAIT_S) {
        seconds = MIN_BLOCKED_WAIT_S;
      }
      record.status = STATUS_BLOCKED;
      record.next_try_at = Date.now() + 1000 * seconds;
      record.serious = err.serious === true;
      record.emergency_number = err.emergencyNumber || '';
      announce(blockedAnnouncement(record));
    } else if (status === 401 || (status === 409 && err.field === OTHER_ACCOUNT_FIELD)) {
      // Signed out, or another account is signed in now than the one this
      // page and its saved reports belong to (WR-W09). Every other report of
      // this account would get the same answer, so they all wait, nothing more
      // is sent from this page and Discard stays on each row. For another
      // account the Not sent rows are held too: their Edit and resend would
      // post through the form under the account signed in now.
      held = status === 401 ? STATUS_SIGNED_OUT : STATUS_OTHER_ACCOUNT;
      rows.forEach(function (other) {
        if (belongsToAccount(other) && (other.status !== STATUS_REJECTED || held === STATUS_OTHER_ACCOUNT)) {
          other.status = held;
          other.sending_until = 0;
        }
      });
      record.status = held;
      sessionLapsed = true;
      announce(status === 401 ? ANNOUNCE_SIGNED_OUT : ANNOUNCE_OTHER_ACCOUNT);
      outcome = 'stop';
    } else {
      record.status = STATUS_REJECTED;
      record.last_error = (err && err.fieldMessage) || GENERIC_REASON;
      announce(ANNOUNCE_NOT_SENT_PREFIX + record.last_error);
    }
    var writes = [putRow(record)];
    if (held !== '') {
      rows.forEach(function (other) {
        if (other !== record && other.status === held) {
          writes.push(putRow(other));
        }
      });
    }
    return Promise.all(writes).then(function () {
      renderStrip();
      scheduleWake();
      return outcome;
    });
  }

  // scheduleWake sets one timer for the Blocked row whose wait ends first. A
  // row a pass would skip (shown from the last good read while the store
  // cannot be read, or with its Discard question open) sets none; the question
  // sets it again when it closes. A row whose wait is over but that a wake did
  // not send (another tab holds the flush lock, say) is tried after 1 second,
  // then 2, 4 and so on, up to a minute (WR-07). A row being sent wakes the
  // page when its lease ends.
  function scheduleWake() {
    if (wakeTimer !== null) {
      window.clearTimeout(wakeTimer);
      wakeTimer = null;
    }
    if (sessionLapsed) {
      return;
    }
    var earliest = null;
    rows.forEach(function (record) {
      if (record.status !== STATUS_BLOCKED || !belongsToAccount(record) || isStale(record) ||
          questionOpen(record.request_id)) {
        return;
      }
      var at = Math.max(record.next_try_at, record.sending_until || 0);
      if (earliest === null || at < earliest) {
        earliest = at;
      }
    });
    if (earliest === null) {
      wakeBackoffMs = 0;
      return;
    }
    var now = Date.now();
    var delay;
    if (earliest > now) {
      wakeBackoffMs = 0;
      delay = Math.min(QUEUE_FLUSH_MS, Math.max(1000, earliest - now));
    } else {
      delay = wakeBackoffMs > 0 ? wakeBackoffMs : 1000;
    }
    wakeTimer = window.setTimeout(function () {
      wakeTimer = null;
      // The next wake for a row still due after this one waits twice as long.
      wakeBackoffMs = Math.min(MAX_BACKOFF_MS, 2 * (wakeBackoffMs > 0 ? wakeBackoffMs : 1000));
      flush();
    }, delay);
  }

  // refreshWaits rewrites only the wait text of each Blocked row. It never
  // rebuilds a status line or a link, so focus on the link stays put, and it
  // announces nothing.
  function refreshWaits() {
    rows.forEach(function (record) {
      var rowEl = rowEls[record.request_id];
      if (!rowEl || record.status !== STATUS_BLOCKED || !belongsToAccount(record)) {
        return;
      }
      var waitEl = rowEl.querySelector('.queue-strip__wait');
      if (waitEl) {
        Pinalert.setText(waitEl, waitText(record));
      }
    });
  }

  function waitSeconds(record) {
    return Math.max(1, Math.ceil((record.next_try_at - Date.now()) / 1000));
  }

  function waitText(record) {
    return BLOCKED_PREFIX + Pinalert.formatWait(waitSeconds(record)) + '.';
  }

  // ----- the strip -----

  function ensureFrame() {
    if (headEl) {
      return;
    }
    headEl = document.createElement('div');
    headEl.className = 'queue-strip__head';

    headingEl = document.createElement(headingLevel === 3 ? 'h3' : 'h2');
    headingEl.id = 'queue-strip-heading';
    headingEl.className = 'queue-strip__heading';
    headingEl.setAttribute('tabindex', '-1');
    Pinalert.setText(headingEl, QUEUE_HEADING);
    headEl.appendChild(headingEl);

    sendBtn = document.createElement('button');
    sendBtn.type = 'button';
    sendBtn.className = 'btn';
    sendBtn.id = 'queue-send-now';
    Pinalert.setText(sendBtn, SEND_NOW_LABEL);
    sendBtn.addEventListener('click', function () {
      if (sendBtn.getAttribute('aria-disabled') === 'true') {
        return;
      }
      runFlush(true);
    });
    headEl.appendChild(sendBtn);

    countEl = document.createElement('p');
    countEl.className = 'queue-strip__count';
    helpEl = document.createElement('p');
    helpEl.className = 'queue-strip__help';
    listEl = document.createElement('ul');
    listEl.className = 'queue-strip__rows';

    stripEl.appendChild(headEl);
    stripEl.appendChild(countEl);
    stripEl.appendChild(helpEl);
    stripEl.appendChild(listEl);
  }

  function isLeased(record) {
    return record.sending_until > Date.now();
  }

  // isStale is true for a stored row shown from the last good read while the
  // store cannot be read. It is never sent: another tab may have sent it.
  function isStale(record) {
    return storeUnreadable && !Object.prototype.hasOwnProperty.call(memoryRows, record.request_id);
  }

  // questionOpen is true while the Discard question of that row is on screen.
  function questionOpen(requestId) {
    var rowEl = rowEls[requestId];
    return !!rowEl && !!rowEl.querySelector('.queue-strip__confirm');
  }

  function renderStrip() {
    var list = rows.filter(function (record) {
      return belongsToAccount(record);
    });
    var wanted = {};
    list.forEach(function (record) {
      wanted[record.request_id] = true;
    });

    // Rows that left the list, and where focus goes when one of them had it.
    if (listEl) {
      var before = Array.prototype.slice.call(listEl.children);
      before.forEach(function (rowEl) {
        var id = rowEl.getAttribute('data-request-id');
        if (wanted[id]) {
          return;
        }
        var hadFocus = rowEl.contains(document.activeElement);
        var at = Array.prototype.indexOf.call(listEl.children, rowEl);
        listEl.removeChild(rowEl);
        delete rowEls[id];
        if (hadFocus) {
          moveFocusAfterRemoval(at);
        }
      });
    }

    if (list.length === 0) {
      stripEl.hidden = true;
    } else {
      ensureFrame();
      var count = list.length;
      Pinalert.setText(countEl, count + (count === 1 ? COUNT_ONE_SUFFIX : COUNT_MANY_SUFFIX));
      var help = HELP_LINE;
      if (memoryOnly) {
        help = HELP_LINE_MEMORY;
      } else if (storeUnreadable) {
        help = HELP_LINE_UNREADABLE;
      }
      Pinalert.setText(helpEl, help);
      var now = Date.now();
      var nothingToSend = list.every(function (record) {
        return isLeased(record) || isStale(record) ||
          (record.status === STATUS_BLOCKED && record.next_try_at > now);
      });
      Pinalert.setText(sendBtn, passRunning ? SENDING_LABEL : SEND_NOW_LABEL);
      if (sessionLapsed || nothingToSend || passRunning) {
        sendBtn.setAttribute('aria-disabled', 'true');
      } else {
        sendBtn.removeAttribute('aria-disabled');
      }
      list.forEach(function (record, index) {
        var rowEl = rowEls[record.request_id];
        if (rowEl) {
          updateRow(rowEl, record);
        } else {
          rowEl = buildRow(record);
          rowEls[record.request_id] = rowEl;
        }
        if (listEl.children[index] !== rowEl) {
          listEl.insertBefore(rowEl, listEl.children[index] || null);
        }
      });
      stripEl.hidden = false;
    }

    stripEl.dispatchEvent(new CustomEvent(COUNT_EVENT, { detail: { count: list.length } }));
  }

  function buildRow(record) {
    var rowEl = document.createElement('li');
    rowEl.className = 'queue-strip__row';
    rowEl.setAttribute('data-request-id', record.request_id);

    var metaEl = document.createElement('div');
    metaEl.className = 'queue-strip__meta';
    if (window.PinalertTrust) {
      var label = window.PinalertTrust.createSeverityLabel();
      window.PinalertTrust.updateSeverityLabel(label, { severity: record.payload.severity });
      metaEl.appendChild(label);
    }
    var categoryEl = document.createElement('span');
    categoryEl.className = 'queue-strip__category';
    metaEl.appendChild(categoryEl);
    rowEl.appendChild(metaEl);

    var textEl = document.createElement('p');
    textEl.className = 'queue-strip__text';
    rowEl.appendChild(textEl);

    var statusEl = document.createElement('p');
    statusEl.className = 'queue-strip__status';
    rowEl.appendChild(statusEl);

    var actionsEl = document.createElement('div');
    actionsEl.className = 'queue-strip__actions';
    rowEl.appendChild(actionsEl);

    updateRow(rowEl, record);
    return rowEl;
  }

  // updateRow refreshes the text of a row every time, and rebuilds the status
  // line and the buttons only when the row's state changed, so a button that
  // has focus keeps it across a render.
  function updateRow(rowEl, record) {
    var payload = record.payload || {};
    var categoryEl = rowEl.querySelector('.queue-strip__category');
    var textEl = rowEl.querySelector('.queue-strip__text');
    var statusEl = rowEl.querySelector('.queue-strip__status');
    var actionsEl = rowEl.querySelector('.queue-strip__actions');

    var known = Object.prototype.hasOwnProperty.call(Pinalert.CATEGORY_LABELS, payload.category);
    Pinalert.setText(categoryEl, known ? Pinalert.CATEGORY_LABELS[payload.category] : Pinalert.CATEGORY_LABELS.other);
    Pinalert.setText(textEl, payload.description);

    // An open Discard question is never torn down by a render: the status line
    // and the buttons catch up when the question closes (WR-W07).
    if (rowEl.querySelector('.queue-strip__confirm')) {
      return;
    }

    var leased = isLeased(record);
    // A row shown from the last good read while the store cannot be read gets
    // no buttons: its delete could not reach the store (WR-W06).
    var stale = isStale(record);
    var signature = [leased ? 'sending' : record.status, record.last_error, record.serious === true,
      record.emergency_number, record.status === STATUS_BLOCKED ? record.next_try_at : '', stale].join('|');
    if (rowEl.getAttribute('data-state') === signature) {
      return;
    }
    rowEl.setAttribute('data-state', signature);
    if (leased) {
      rowEl.setAttribute('aria-busy', 'true');
    } else {
      rowEl.removeAttribute('aria-busy');
    }

    // The status line.
    Pinalert.setText(statusEl, '');
    statusEl.className = 'queue-strip__status inline-note';
    if (leased) {
      Pinalert.setText(statusEl, STATUS_TEXT_SENDING);
    } else if (record.status === STATUS_NO_ANSWER) {
      Pinalert.setText(statusEl, STATUS_TEXT_NO_ANSWER);
    } else if (record.status === STATUS_BLOCKED) {
      var waitEl = document.createElement('span');
      waitEl.className = 'queue-strip__wait';
      Pinalert.setText(waitEl, waitText(record));
      statusEl.appendChild(waitEl);
      if (record.serious === true) {
        appendEmergencyCall(statusEl, record.emergency_number);
      }
    } else if (record.status === STATUS_REJECTED) {
      statusEl.className = 'queue-strip__status inline-error';
      Pinalert.setText(statusEl, NOT_SENT_PREFIX + record.last_error);
    } else if (record.status === STATUS_SIGNED_OUT) {
      Pinalert.setText(statusEl, STATUS_TEXT_SIGNED_OUT);
    } else if (record.status === STATUS_OTHER_ACCOUNT) {
      Pinalert.setText(statusEl, STATUS_TEXT_OTHER_ACCOUNT);
    } else {
      Pinalert.setText(statusEl, STATUS_TEXT_WAITING);
    }

    // The buttons.
    var hadFocus = actionsEl.contains(document.activeElement);
    Pinalert.setText(actionsEl, '');
    actionsEl.hidden = false;
    if (leased || stale) {
      actionsEl.hidden = true;
      if (stale && hadFocus && headingEl) {
        headingEl.focus();
      }
      return;
    }
    if (record.status === STATUS_REJECTED) {
      var edit = buildEditControl(record);
      if (edit) {
        actionsEl.appendChild(edit);
      }
    }
    if (record.status === STATUS_SIGNED_OUT) {
      var signIn = document.createElement('a');
      signIn.className = 'btn';
      signIn.setAttribute('href', '/login');
      Pinalert.setText(signIn, SIGN_IN_LABEL);
      actionsEl.appendChild(signIn);
    }
    var discardBtn = document.createElement('button');
    discardBtn.type = 'button';
    discardBtn.className = 'btn queue-strip__discard';
    Pinalert.setText(discardBtn, DISCARD_LABEL);
    discardBtn.addEventListener('click', function () {
      openDiscardConfirm(rowEl, record);
    });
    actionsEl.appendChild(discardBtn);
  }

  // safeResendHref returns value only when it is a path on this site: one
  // leading slash, not followed by a second slash or a backslash (browsers read
  // a backslash there as a second slash, which names another host).
  function safeResendHref(value) {
    if (typeof value === 'string' && value.charAt(0) === '/' &&
        value.charAt(1) !== '/' && value.charAt(1) !== '\\') {
      return value;
    }
    return '';
  }

  // createResendLink makes the Edit and resend link with DOM calls. It takes the
  // look of the Sign in again link, so it needs no rule of its own.
  function createResendLink(href) {
    var link = document.createElement('a');
    link.className = 'btn';
    link.setAttribute('href', href);
    Pinalert.setText(link, EDIT_LABEL);
    return link;
  }

  // buildEditControl is the one place an Edit and resend control is made. It
  // returns null when this page has no form to open and no link to one. A page
  // that passes a link builder gets a link, and the edit callback is not used.
  function buildEditControl(record) {
    if (typeof resendHref === 'function') {
      var target = safeResendHref(resendHref(record));
      return target === '' ? null : createResendLink(target);
    }
    if (typeof onEdit !== 'function') {
      return null;
    }
    var button = document.createElement('button');
    button.type = 'button';
    button.className = 'btn';
    Pinalert.setText(button, EDIT_LABEL);
    button.addEventListener('click', function () {
      onEdit(record, button, noteFor(record));
    });
    return button;
  }

  function openDiscardConfirm(rowEl, record) {
    var actionsEl = rowEl.querySelector('.queue-strip__actions');
    if (rowEl.querySelector('.queue-strip__confirm')) {
      return;
    }
    questionSeq += 1;
    var questionId = 'queue-question-' + questionSeq;

    var group = document.createElement('div');
    group.className = 'queue-strip__confirm';
    group.setAttribute('role', 'group');
    group.setAttribute('aria-labelledby', questionId);

    var question = document.createElement('p');
    question.className = 'queue-strip__question';
    question.id = questionId;
    Pinalert.setText(question, DISCARD_QUESTION);
    group.appendChild(question);

    var keep = document.createElement('button');
    keep.type = 'button';
    keep.className = 'btn';
    Pinalert.setText(keep, KEEP_LABEL);
    group.appendChild(keep);

    var yes = document.createElement('button');
    yes.type = 'button';
    yes.className = 'btn btn--destructive';
    Pinalert.setText(yes, CONFIRM_DISCARD_LABEL);
    group.appendChild(yes);

    // closeQuestion lets the row catch up with any change held back while the
    // question was open, then puts focus on the row's Discard button as it is
    // now (a catch up may have rebuilt it).
    var closeQuestion = function () {
      if (group.parentNode) {
        group.parentNode.removeChild(group);
      }
      actionsEl.hidden = false;
      renderStrip();
      scheduleWake();
      var current = rowEls[record.request_id];
      var target = current ? current.querySelector('.queue-strip__discard') : null;
      if (target) {
        target.focus();
      } else if (headingEl && !stripEl.hidden) {
        headingEl.focus();
      }
    };
    keep.addEventListener('click', closeQuestion);
    group.addEventListener('keydown', function (evt) {
      if (evt.key === 'Escape') {
        evt.stopPropagation();
        closeQuestion();
      }
    });
    yes.addEventListener('click', function () {
      discardOutcome(record.request_id).then(function (outcome) {
        if (outcome === DISCARD_REMOVED) {
          Pinalert.showToast(DISCARDED_TOAST);
          return;
        }
        closeQuestion();
        if (outcome === DISCARD_FAILED) {
          Pinalert.showToast(DISCARD_FAILED_TOAST);
        } else if (outcome === DISCARD_SENDING) {
          Pinalert.showToast(DISCARD_SENDING_TOAST);
        }
      });
    });

    actionsEl.hidden = true;
    rowEl.appendChild(group);
    keep.focus();
  }

  // appendEmergencyCall adds "In an emergency, call {NUMBER} now." to el, the
  // number being a tap to call link built with DOM calls.
  function appendEmergencyCall(el, number) {
    if (typeof number !== 'string' || number === '') {
      return;
    }
    el.appendChild(document.createTextNode(' ' + EMERGENCY_CALL_PREFIX));
    var link = document.createElement('a');
    link.className = 'queue-strip__call';
    link.setAttribute('href', 'tel:' + number);
    link.setAttribute('aria-label', CALL_LABEL_PREFIX + number);
    Pinalert.setText(link, number);
    el.appendChild(link);
    el.appendChild(document.createTextNode(EMERGENCY_CALL_SUFFIX));
  }

  // blockedAnnouncement is the one builder of the Blocked announcement. It is
  // plain text, with no link.
  function blockedAnnouncement(record) {
    var text = ANNOUNCE_BLOCKED_PREFIX + Pinalert.formatWait(waitSeconds(record)) + '.';
    if (record.serious === true && typeof record.emergency_number === 'string' && record.emergency_number !== '') {
      text += ' ' + EMERGENCY_CALL_PREFIX + record.emergency_number + EMERGENCY_CALL_SUFFIX;
    }
    return text;
  }

  function noteFor(record) {
    if (record && record.status === STATUS_REJECTED && record.last_error) {
      return NOT_SENT_PREFIX + record.last_error;
    }
    return '';
  }

  function announce(text) {
    var live = document.getElementById('notice-live');
    if (!live) {
      return;
    }
    Pinalert.setText(live, '');
    window.setTimeout(function () {
      Pinalert.setText(live, text);
    }, 50);
  }

  function requestPersist() {
    if (persistAsked) {
      return;
    }
    persistAsked = true;
    try {
      if (navigator.storage && typeof navigator.storage.persist === 'function') {
        navigator.storage.persist();
      }
    } catch (e) {
      // The browser keeps its own rules about storage.
    }
  }

  // moveFocusAfterRemoval runs only when focus was inside the removed row: the
  // next row's first button, else the previous row's, else the fallback.
  function moveFocusAfterRemoval(index) {
    var rowsNow = listEl ? listEl.children : [];
    var candidates = [rowsNow[index], rowsNow[index - 1]];
    for (var i = 0; i < candidates.length; i++) {
      if (candidates[i]) {
        var button = candidates[i].querySelector('button, a');
        if (button) {
          button.focus();
          return;
        }
      }
    }
    if (typeof focusFallback === 'function') {
      var target = focusFallback();
      if (target && typeof target.focus === 'function') {
        target.focus();
      }
    }
  }

  // openResendFromAddress opens the form for ?resend=<request id>, only for a
  // saved row of this account. Any other case does nothing (T-04-14).
  function openResendFromAddress() {
    var requestId = null;
    var url = null;
    try {
      url = new URL(window.location.href);
      requestId = url.searchParams.get('resend');
    } catch (e) {
      return;
    }
    if (typeof requestId !== 'string' || !REQUEST_ID_PATTERN.test(requestId)) {
      return;
    }
    var record = findRow(requestId);
    if (!record || !belongsToAccount(record)) {
      return;
    }
    onEdit(record, null, noteFor(record));
    try {
      var kept = [];
      url.searchParams.forEach(function (value, key) {
        if (key !== 'resend') {
          kept.push([key, value]);
        }
      });
      var search = new URLSearchParams(kept).toString();
      window.history.replaceState(window.history.state, '', url.pathname + (search ? '?' + search : '') + url.hash);
    } catch (e) {
      // The address stays as it is.
    }
  }

  window.PinalertQueue = {
    mount: mount,
    save: save,
    flush: flush,
    discard: discard,
    get: get
  };
}());
