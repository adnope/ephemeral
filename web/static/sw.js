const CACHE_VERSION = "ephemeral-v3";

const SHELL_ASSETS = [
  "/",
  "/static/app.min.js",
  "/static/app.css",
  "/static/manifest.json",
];

const BYPASS_PATTERNS = [
  /^\/api\/events$/,
  /^\/api\/upload$/,
  /^\/api\/message$/,
  /^\/api\/files\//,
  /^\/api\/file-preview\//,
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(CACHE_VERSION)
      .then((cache) => cache.addAll(SHELL_ASSETS))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys
            .filter((key) => key !== CACHE_VERSION)
            .map((key) => caches.delete(key)),
        ),
      )
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);

  if (BYPASS_PATTERNS.some((pattern) => pattern.test(url.pathname))) return;
  if (event.request.method !== "GET") return;
  if (event.request.headers.has("range")) return;

  if (SHELL_ASSETS.includes(url.pathname)) {
    event.respondWith(
      caches
        .match(event.request)
        .then((cached) => cached || fetch(event.request)),
    );
    return;
  }

  event.respondWith(fetch(event.request));
});
