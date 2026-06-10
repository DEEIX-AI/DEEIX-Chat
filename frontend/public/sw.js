const PWA_ASSET_VERSION = "0.2.3";
const PWA_ASSET_CACHE_KEY = "632cb83037d9";
const PWA_ASSET_MANIFEST = {
  "/pwa/icon.svg": "/pwa/generated/icon.fe1d64d9758c.svg",
  "/pwa/icon-192.png": "/pwa/generated/icon-192.d7a0049daaa7.png",
  "/pwa/icon-512.png": "/pwa/generated/icon-512.bd37e5f66cc5.png",
  "/pwa/icon-maskable-512.png": "/pwa/generated/icon-maskable-512.647d8497d850.png",
  "/pwa/apple-touch-icon.png": "/pwa/generated/apple-touch-icon.0c62df73d41b.png"
};
const STATIC_CACHE = `deeix-chat-static-${PWA_ASSET_VERSION}-${PWA_ASSET_CACHE_KEY}`;
const PAGE_CACHE = `deeix-chat-pages-${PWA_ASSET_VERSION}`;
const STATIC_CACHE_MAX_ENTRIES = 160;
const PAGE_CACHE_MAX_ENTRIES = 24;

const APP_SHELL_URLS = [
  "/",
  "/chat",
  "/logo.svg",
  "/logo-color.svg",
  pwaAsset("/pwa/icon.svg"),
  pwaAsset("/pwa/icon-192.png"),
  pwaAsset("/pwa/icon-512.png"),
];

const BACKEND_PATH_PREFIXES = [
  "/api/",
  "/swagger",
  "/healthz",
  "/readyz",
];

function pwaAsset(path) {
  return PWA_ASSET_MANIFEST[path] ?? path;
}

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(STATIC_CACHE)
      .then((cache) => cache.addAll(APP_SHELL_URLS))
      .catch(() => undefined)
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(
        keys
          .filter((key) => key !== STATIC_CACHE && key !== PAGE_CACHE)
          .map((key) => caches.delete(key)),
      ))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const request = event.request;
  if (request.method !== "GET") {
    return;
  }

  const url = new URL(request.url);
  if (url.origin !== self.location.origin || shouldBypassCache(url)) {
    return;
  }

  if (request.mode === "navigate") {
    event.respondWith(networkFirst(request, PAGE_CACHE, PAGE_CACHE_MAX_ENTRIES));
    return;
  }

  if (isStaticAsset(url)) {
    event.respondWith(staleWhileRevalidate(request, STATIC_CACHE, STATIC_CACHE_MAX_ENTRIES));
  }
});

function shouldBypassCache(url) {
  if (BACKEND_PATH_PREFIXES.some((prefix) => url.pathname === prefix || url.pathname.startsWith(prefix))) {
    return true;
  }
  if (url.pathname.includes("/content") || url.pathname.includes("/download")) {
    return true;
  }
  return false;
}

function isStaticAsset(url) {
  return url.pathname.startsWith("/_next/static/") ||
    url.pathname.startsWith("/pwa/") ||
    url.pathname.startsWith("/vendor/") ||
    /\.(?:css|js|mjs|png|jpg|jpeg|gif|webp|svg|ico|woff2?|ttf|otf|wasm)$/i.test(url.pathname);
}

async function networkFirst(request, cacheName, maxEntries) {
  const cache = await caches.open(cacheName);
  try {
    const response = await fetch(request);
    if (isCacheable(response)) {
      await cache.put(request, response.clone());
      await trimCache(cacheName, maxEntries);
    }
    return response;
  } catch {
    const cached = await cache.match(request);
    if (cached) {
      return cached;
    }
    return cache.match("/") || Response.error();
  }
}

async function staleWhileRevalidate(request, cacheName, maxEntries) {
  const cache = await caches.open(cacheName);
  const cached = await cache.match(request);
  const fresh = fetch(request)
    .then(async (response) => {
      if (isCacheable(response)) {
        await cache.put(request, response.clone());
        await trimCache(cacheName, maxEntries);
      }
      return response;
    })
    .catch(() => undefined);

  return cached || fresh || Response.error();
}

function isCacheable(response) {
  return response && response.ok && response.type === "basic";
}

async function trimCache(cacheName, maxEntries) {
  const cache = await caches.open(cacheName);
  const keys = await cache.keys();
  if (keys.length <= maxEntries) {
    return;
  }
  await Promise.all(keys.slice(0, keys.length - maxEntries).map((key) => cache.delete(key)));
}
