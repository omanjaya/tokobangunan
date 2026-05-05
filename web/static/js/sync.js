/*
 * Tokobangunan offline sync queue.
 *
 * Public API:
 *   window.tbSync.enqueue({ url, method, headers, body, contentType, client_uuid })
 *   window.tbSync.list()
 *   window.tbSync.remove(id)
 *   window.tbSync.size()
 *   window.tbSync.flush()
 *
 * Auto-flush:
 *   - window 'online' event
 *   - polling interval 30s saat tab visible
 *   - service worker message 'SYNC_FLUSH_REQUEST'
 */
(function () {
  "use strict";

  const DB_NAME = "tokobangunan-sync";
  const DB_VERSION = 1;
  const STORE = "pending";

  // ---------- IndexedDB wrapper -------------------------------------------

  class SyncQueue {
    constructor() {
      this.db = null;
    }

    async open() {
      if (this.db) return this.db;
      this.db = await new Promise((resolve, reject) => {
        const req = indexedDB.open(DB_NAME, DB_VERSION);
        req.onupgradeneeded = () => {
          const db = req.result;
          if (!db.objectStoreNames.contains(STORE)) {
            db.createObjectStore(STORE, { keyPath: "id", autoIncrement: true });
          }
        };
        req.onsuccess = () => resolve(req.result);
        req.onerror = () => reject(req.error);
      });
      return this.db;
    }

    async enqueue(record) {
      const db = await this.open();
      const payload = Object.assign(
        {
          url: "",
          method: "POST",
          headers: {},
          body: "",
          contentType: "",
          client_uuid: "",
          created_at: Date.now(),
        },
        record
      );
      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORE, "readwrite");
        const req = tx.objectStore(STORE).add(payload);
        req.onsuccess = () => resolve(req.result);
        req.onerror = () => reject(req.error);
      });
    }

    async list() {
      const db = await this.open();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORE, "readonly");
        const req = tx.objectStore(STORE).getAll();
        req.onsuccess = () => resolve(req.result || []);
        req.onerror = () => reject(req.error);
      });
    }

    async remove(id) {
      const db = await this.open();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORE, "readwrite");
        const req = tx.objectStore(STORE).delete(id);
        req.onsuccess = () => resolve();
        req.onerror = () => reject(req.error);
      });
    }

    async size() {
      const db = await this.open();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORE, "readonly");
        const req = tx.objectStore(STORE).count();
        req.onsuccess = () => resolve(req.result || 0);
        req.onerror = () => reject(req.error);
      });
    }
  }

  // ---------- Sync engine --------------------------------------------------

  class SyncEngine {
    constructor(queue) {
      this.queue = queue;
      this._running = false;
    }

    async flush() {
      if (this._running) return { running: true };
      if (!navigator.onLine) return { skipped: true, reason: "offline" };
      this._running = true;
      let success = 0;
      let kept = 0;
      try {
        const items = await this.queue.list();
        for (const item of items) {
          const result = await this._send(item);
          if (result === "ok") {
            await this.queue.remove(item.id);
            success++;
          } else if (result === "drop") {
            // 4xx selain conflict -> permanen gagal, drop biar tidak loop.
            await this.queue.remove(item.id);
            kept++;
          } else if (result === "network") {
            // Masih offline -> stop loop.
            break;
          } else {
            // 5xx -> keep, lanjut item berikutnya.
            kept++;
          }
        }
        if (success > 0 && window.app && typeof window.app.toast === "function") {
          window.app.toast(
            "success",
            "Sync selesai",
            success + " transaksi offline berhasil dikirim"
          );
        }
      } finally {
        this._running = false;
      }
      // Notify listener.
      window.dispatchEvent(
        new CustomEvent("tbsync:flushed", { detail: { success, kept } })
      );
      return { success, kept };
    }

    async _send(item) {
      const headers = Object.assign({}, item.headers || {});
      // Buang headers yang tidak valid di-set manual.
      delete headers["Cookie"];
      delete headers["Content-Length"];
      delete headers["Host"];
      if (item.contentType && !headers["Content-Type"]) {
        headers["Content-Type"] = item.contentType;
      }
      // Tandai replay agar server / SW bisa skip queueing ulang.
      headers["X-Offline-Replay"] = "1";

      try {
        const res = await fetch(item.url, {
          method: item.method || "POST",
          headers,
          body: item.body,
          credentials: "same-origin",
        });
        if (res.ok) return "ok";
        if (res.status >= 400 && res.status < 500) {
          // 409 conflict (duplikat client_uuid) = sudah ada di server, anggap sukses.
          if (res.status === 409) return "ok";
          return "drop";
        }
        return "retry";
      } catch (err) {
        return "network";
      }
    }
  }

  // ---------- Bootstrap ---------------------------------------------------

  const queue = new SyncQueue();
  const engine = new SyncEngine(queue);

  const api = {
    enqueue: (r) => queue.enqueue(r),
    list: () => queue.list(),
    remove: (id) => queue.remove(id),
    size: () => queue.size(),
    flush: () => engine.flush(),
  };

  // Auto-flush triggers.
  window.addEventListener("online", () => {
    engine.flush().catch(() => {});
  });

  setInterval(() => {
    if (navigator.onLine && document.visibilityState === "visible") {
      engine.flush().catch(() => {});
    }
  }, 30000);

  // SW message hook.
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.addEventListener("message", (e) => {
      const data = e.data || {};
      if (data.type === "SYNC_FLUSH_REQUEST") {
        engine.flush().catch(() => {});
      }
    });
  }

  // Initial flush attempt (tab dibuka, mungkin ada residue).
  document.addEventListener("DOMContentLoaded", () => {
    if (navigator.onLine) {
      setTimeout(() => engine.flush().catch(() => {}), 1500);
    }
  });

  window.tbSync = api;
})();
