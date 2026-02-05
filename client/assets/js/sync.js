// IndexedDB Sync Module for Cheapskate Finance Tracker
// Maintains a browser-side cache of the current year's transactions in IndexedDB.
// If the SQLite database is deleted, data can be reconstructed from this cache.

(function () {
  "use strict";

  var DB_NAME = "cheapskate-sync";
  var DB_VERSION = 1;
  var STORE_TRANSACTIONS = "transactions";
  var STORE_CATEGORIES = "categories";
  var STORE_META = "meta";

  // Open (or create) the IndexedDB database
  function openDB() {
    return new Promise(function (resolve, reject) {
      var request = indexedDB.open(DB_NAME, DB_VERSION);

      request.onupgradeneeded = function (event) {
        var idb = event.target.result;
        if (!idb.objectStoreNames.contains(STORE_TRANSACTIONS)) {
          var txStore = idb.createObjectStore(STORE_TRANSACTIONS, {
            keyPath: "id",
          });
          txStore.createIndex("date", "date", { unique: false });
          txStore.createIndex("category_name", "category_name", {
            unique: false,
          });
        }
        if (!idb.objectStoreNames.contains(STORE_CATEGORIES)) {
          idb.createObjectStore(STORE_CATEGORIES, { keyPath: "id" });
        }
        if (!idb.objectStoreNames.contains(STORE_META)) {
          idb.createObjectStore(STORE_META, { keyPath: "key" });
        }
      };

      request.onsuccess = function (event) {
        resolve(event.target.result);
      };

      request.onerror = function (event) {
        reject(event.target.error);
      };
    });
  }

  // Clear an object store
  function clearStore(idb, storeName) {
    return new Promise(function (resolve, reject) {
      var tx = idb.transaction(storeName, "readwrite");
      var store = tx.objectStore(storeName);
      var request = store.clear();
      request.onsuccess = function () {
        resolve();
      };
      request.onerror = function (event) {
        reject(event.target.error);
      };
    });
  }

  // Put multiple items into a store
  function putAll(idb, storeName, items) {
    return new Promise(function (resolve, reject) {
      var tx = idb.transaction(storeName, "readwrite");
      var store = tx.objectStore(storeName);
      for (var i = 0; i < items.length; i++) {
        store.put(items[i]);
      }
      tx.oncomplete = function () {
        resolve();
      };
      tx.onerror = function (event) {
        reject(event.target.error);
      };
    });
  }

  // Get all items from a store
  function getAll(idb, storeName) {
    return new Promise(function (resolve, reject) {
      var tx = idb.transaction(storeName, "readonly");
      var store = tx.objectStore(storeName);
      var request = store.getAll();
      request.onsuccess = function (event) {
        resolve(event.target.result);
      };
      request.onerror = function (event) {
        reject(event.target.error);
      };
    });
  }

  // Put a single meta entry
  function putMeta(idb, key, value) {
    return new Promise(function (resolve, reject) {
      var tx = idb.transaction(STORE_META, "readwrite");
      var store = tx.objectStore(STORE_META);
      store.put({ key: key, value: value });
      tx.oncomplete = function () {
        resolve();
      };
      tx.onerror = function (event) {
        reject(event.target.error);
      };
    });
  }

  // Get a meta entry
  function getMeta(idb, key) {
    return new Promise(function (resolve, reject) {
      var tx = idb.transaction(STORE_META, "readonly");
      var store = tx.objectStore(STORE_META);
      var request = store.get(key);
      request.onsuccess = function (event) {
        var result = event.target.result;
        resolve(result ? result.value : null);
      };
      request.onerror = function (event) {
        reject(event.target.error);
      };
    });
  }

  // Fetch current year data from server and store in IndexedDB
  function syncFromServer() {
    var year = new Date().getFullYear();
    return fetch("/api/sync/export?year=" + year)
      .then(function (res) {
        if (!res.ok) throw new Error("Sync export failed: " + res.status);
        return res.json();
      })
      .then(function (data) {
        return openDB().then(function (idb) {
          // Clear and repopulate stores
          return clearStore(idb, STORE_TRANSACTIONS)
            .then(function () {
              return clearStore(idb, STORE_CATEGORIES);
            })
            .then(function () {
              if (data.transactions && data.transactions.length > 0) {
                return putAll(idb, STORE_TRANSACTIONS, data.transactions);
              }
            })
            .then(function () {
              if (data.categories && data.categories.length > 0) {
                return putAll(idb, STORE_CATEGORIES, data.categories);
              }
            })
            .then(function () {
              return putMeta(idb, "last_sync", new Date().toISOString());
            })
            .then(function () {
              return putMeta(idb, "sync_year", String(year));
            })
            .then(function () {
              idb.close();
            });
        });
      });
  }

  // Check server status and potentially restore from IndexedDB
  function checkAndRestore() {
    return fetch("/api/sync/status")
      .then(function (res) {
        if (!res.ok) throw new Error("Sync status failed: " + res.status);
        return res.json();
      })
      .then(function (status) {
        if (status.transaction_count > 0) {
          // Server has data, sync from server to IndexedDB
          return syncFromServer();
        }

        // Server is empty - check if IndexedDB has data to restore
        return openDB().then(function (idb) {
          return getAll(idb, STORE_TRANSACTIONS).then(function (transactions) {
            idb.close();
            if (transactions && transactions.length > 0) {
              return showRestorePrompt(transactions.length);
            }
          });
        });
      })
      .catch(function (err) {
        console.error("[sync] check failed:", err);
      });
  }

  // Show a restore prompt to the user
  function showRestorePrompt(count) {
    var banner = document.createElement("div");
    banner.id = "sync-restore-banner";
    banner.className =
      "fixed top-16 left-0 right-0 z-50 flex items-center justify-center gap-4 p-3 bg-amber-50 border-b border-amber-200 text-amber-800 text-sm";
    banner.innerHTML =
      '<span>Found <strong>' +
      count +
      "</strong> cached transaction" +
      (count !== 1 ? "s" : "") +
      " in your browser. Restore to server?</span>" +
      '<button id="sync-restore-btn" class="px-3 py-1 bg-amber-600 text-white rounded hover:bg-amber-700 transition text-xs font-medium">Restore</button>' +
      '<button id="sync-dismiss-btn" class="px-3 py-1 bg-gray-200 text-gray-600 rounded hover:bg-gray-300 transition text-xs font-medium">Dismiss</button>';

    document.body.appendChild(banner);

    document
      .getElementById("sync-restore-btn")
      .addEventListener("click", function () {
        restoreToServer().then(function () {
          banner.remove();
          window.location.reload();
        });
      });

    document
      .getElementById("sync-dismiss-btn")
      .addEventListener("click", function () {
        banner.remove();
      });
  }

  // Push IndexedDB data to server
  function restoreToServer() {
    return openDB().then(function (idb) {
      return getAll(idb, STORE_TRANSACTIONS).then(function (transactions) {
        idb.close();
        if (!transactions || transactions.length === 0) return;

        return fetch("/api/sync/import", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ transactions: transactions }),
        }).then(function (res) {
          if (!res.ok) throw new Error("Sync import failed: " + res.status);
          return res.json();
        });
      });
    });
  }

  // Listen for HTMX events to trigger sync after mutations
  function setupHTMXListeners() {
    document.body.addEventListener("htmx:afterRequest", function (evt) {
      var xhr = evt.detail.xhr;
      if (!xhr) return;

      var method = (evt.detail.requestConfig || {}).verb || "";
      var path = (evt.detail.requestConfig || {}).path || "";
      method = method.toUpperCase();

      // Sync after transaction create or delete
      var isCreate =
        method === "POST" && path.indexOf("/api/transaction") !== -1;
      var isDelete =
        method === "DELETE" && path.indexOf("/api/transaction") !== -1;

      if ((isCreate || isDelete) && xhr.status >= 200 && xhr.status < 300) {
        syncFromServer().catch(function (err) {
          console.error("[sync] post-mutation sync failed:", err);
        });
      }
    });
  }

  // Initialize sync on page load
  function init() {
    if (!window.indexedDB) {
      console.warn("[sync] IndexedDB not supported");
      return;
    }

    setupHTMXListeners();

    // Delay initial sync check to avoid blocking page render
    setTimeout(function () {
      checkAndRestore();
    }, 500);
  }

  // Run when DOM is ready
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
