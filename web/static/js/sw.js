/*
 * Tokobangunan Service Worker.
 *
 * Strategy:
 *   - Cache-first untuk static asset (CSS, JS, font, icon)
 *   - Network-first untuk navigasi & API calls; fallback ke cache saat offline
 *   - POST /penjualan saat offline -> queue ke IndexedDB, return synthetic 202
 *
 * Background sync (jika browser support) auto-trigger 'sync-queue' tag saat
 * koneksi balik.
 */

// __BUILD_SHA__ diganti runtime oleh swHandler (BUILD_SHA env atau timestamp).
const CACHE_VERSION = "tokobangunan-__BUILD_SHA__";

const PRECACHE_URLS = [
  "/login",
  "/dashboard",
  "/penjualan/baru",
  "/static/css/app.css",
  "/static/js/htmx.min.js",
  "/static/js/alpine.min.js",
  "/static/js/app.js",
  "/static/js/sync.js",
  "/static/js/uuid.js",
  "/static/js/penjualan-offline.js",
  "/static/js/sw-register.js",
  "/static/manifest.webmanifest",
];

const STATIC_EXT = /\.(?:css|js|woff2?|ttf|otf|eot|png|jpg|jpeg|gif|svg|ico|webp|webmanifest)$/i;

// IndexedDB constants (mirror sync.js).
const DB_NAME = "tokobangunan-sync";
const DB_VERSION = 1;
const STORE_NAME = "pending";

// ---------- IndexedDB helpers (di dalam SW context) -------------------------

function openDB() {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION);
    req.onupgradeneeded = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        db.createObjectStore(STORE_NAME, { keyPath: "id", autoIncrement: true });
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

async function enqueueRequest(record) {
  const db = await openDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, "readwrite");
    const store = tx.objectStore(STORE_NAME);
    const req = store.add(record);
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

// ---------- Install ---------------------------------------------------------

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE_VERSION).then((cache) =>
      // addAll fail kalau salah satu URL gagal; pakai per-item add agar resilient.
      Promise.all(
        PRECACHE_URLS.map((url) =>
          cache.add(new Request(url, { credentials: "same-origin" })).catch(() => {
            /* ignore single failure */
          })
        )
      )
    )
  );
  self.skipWaiting();
});

// ---------- Activate --------------------------------------------------------

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE_VERSION).map((k) => caches.delete(k)))
    )
  );
  self.clients.claim();
});

// ---------- Fetch -----------------------------------------------------------

self.addEventListener("fetch", (event) => {
  const req = event.request;
  const url = new URL(req.url);

  // Same-origin only.
  if (url.origin !== self.location.origin) return;

  // POST /penjualan saat offline -> queue.
  if (req.method === "POST" && url.pathname === "/penjualan") {
    event.respondWith(handlePenjualanPost(req));
    return;
  }

  // Non-GET selain di atas: passthrough.
  if (req.method !== "GET") return;

  // Static asset: stale-while-revalidate untuk JS/CSS supaya update tidak stuck
  // di cache lama. Asset lain (gambar/icon/font) tetap cache-first.
  if (STATIC_EXT.test(url.pathname) || url.pathname.startsWith("/static/")) {
    if (/\.(?:js|css)$/.test(url.pathname)) {
      event.respondWith(staleWhileRevalidate(req));
    } else {
      event.respondWith(cacheFirst(req));
    }
    return;
  }

  // Navigasi & sisanya: network-first.
  event.respondWith(networkFirst(req));
});

async function staleWhileRevalidate(req) {
  const cache = await caches.open(CACHE_VERSION);
  const cached = await cache.match(req);
  const fetchPromise = fetch(req)
    .then((res) => {
      if (res && res.ok) cache.put(req, res.clone());
      return res;
    })
    .catch(() => cached);
  return cached || fetchPromise;
}

async function cacheFirst(req) {
  const cached = await caches.match(req);
  if (cached) return cached;
  try {
    const res = await fetch(req);
    if (res && res.ok) {
      const cache = await caches.open(CACHE_VERSION);
      cache.put(req, res.clone());
    }
    return res;
  } catch (err) {
    return new Response("Offline", { status: 503, statusText: "Offline" });
  }
}

async function networkFirst(req) {
  try {
    const res = await fetch(req);
    if (res && res.ok && req.method === "GET") {
      const cache = await caches.open(CACHE_VERSION);
      cache.put(req, res.clone());
    }
    return res;
  } catch (err) {
    const cached = await caches.match(req);
    if (cached) return cached;
    // Fallback ke /dashboard cache untuk navigasi.
    if (req.mode === "navigate") {
      const dashFallback = await caches.match("/dashboard");
      if (dashFallback) return dashFallback;
    }
    return new Response("Offline", {
      status: 503,
      statusText: "Offline",
      headers: { "Content-Type": "text/plain; charset=utf-8" },
    });
  }
}

async function handlePenjualanPost(req) {
  // Coba network dulu; offline fallback queue.
  try {
    const clone = req.clone();
    const res = await fetch(req);
    return res;
  } catch (err) {
    // Read body untuk disimpan.
    let bodyText = "";
    let contentType = req.headers.get("Content-Type") || "";
    try {
      bodyText = await req.clone().text();
    } catch (e) {
      bodyText = "";
    }
    const headers = {};
    req.headers.forEach((v, k) => {
      headers[k] = v;
    });

    // Coba ekstrak client_uuid dari body untuk dedup.
    let clientUUID = "";
    try {
      if (contentType.includes("application/x-www-form-urlencoded")) {
        const params = new URLSearchParams(bodyText);
        clientUUID = params.get("client_uuid") || "";
      } else if (contentType.includes("application/json")) {
        const parsed = JSON.parse(bodyText);
        clientUUID = parsed.client_uuid || "";
      }
    } catch (e) {
      /* ignore */
    }

    await enqueueRequest({
      url: req.url,
      method: req.method,
      headers,
      body: bodyText,
      contentType,
      client_uuid: clientUUID,
      created_at: Date.now(),
    });

    // Register background sync kalau didukung.
    try {
      if (self.registration && self.registration.sync) {
        await self.registration.sync.register("sync-queue");
      }
    } catch (e) {
      /* ignore */
    }

    const respBody = JSON.stringify({
      ok: true,
      queued: true,
      client_uuid: clientUUID,
      message: "Tersimpan offline, akan sync saat online",
    });
    return new Response(respBody, {
      status: 202,
      statusText: "Accepted (Queued Offline)",
      headers: { "Content-Type": "application/json; charset=utf-8" },
    });
  }
}

// ---------- Message ---------------------------------------------------------

self.addEventListener("message", (event) => {
  const data = event.data || {};
  if (data.type === "SKIP_WAITING") {
    self.skipWaiting();
  } else if (data.type === "FLUSH_QUEUE") {
    event.waitUntil(notifyClientsToFlush());
  }
});

async function notifyClientsToFlush() {
  const clients = await self.clients.matchAll({ includeUncontrolled: true });
  for (const c of clients) {
    c.postMessage({ type: "SYNC_FLUSH_REQUEST" });
  }
}

// ---------- Background Sync ------------------------------------------------

self.addEventListener("sync", (event) => {
  if (event.tag === "sync-queue") {
    event.waitUntil(notifyClientsToFlush());
  }
});
