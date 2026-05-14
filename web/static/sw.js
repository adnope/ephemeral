const CACHE_VERSION = 'leandrop-v1';

const SHELL_ASSETS = [
    '/',
    '/static/app.min.js',
    '/static/app.css',
    '/static/manifest.json',
];

// Never intercept these paths; they must hit the origin
const BYPASS_PATTERNS = [
    /^\/events$/,
    /^\/upload$/,
    /^\/message$/,
    /^\/files\//,
];

self.addEventListener('install', event => {
    event.waitUntil(
        caches.open(CACHE_VERSION)
            .then(cache => cache.addAll(SHELL_ASSETS))
            .then(() => self.skipWaiting())
    );
});

self.addEventListener('activate', event => {
    event.waitUntil(
        caches.keys().then(keys =>
            Promise.all(
                keys.filter(k => k !== CACHE_VERSION)
                    .map(k => caches.delete(k))
            )
        ).then(() => self.clients.claim())
    );
});

self.addEventListener('fetch', event => {
    const url = new URL(event.request.url);

    // Bypass: SSE, uploads, POSTs
    if (BYPASS_PATTERNS.some(p => p.test(url.pathname))) return;
    if (event.request.method !== 'GET') return;

    // Shell assets: Cache-First
    if (SHELL_ASSETS.includes(url.pathname)) {
        event.respondWith(
            caches.match(event.request).then(cached =>
                cached || fetch(event.request)
            )
        );
        return;
    }

    // API routes: Network-First with cache fallback
    event.respondWith(
        fetch(event.request)
            .then(response => {
                const clone = response.clone();
                caches.open(CACHE_VERSION)
                    .then(c => c.put(event.request, clone));
                return response;
            })
            .catch(() =>
                caches.match(event.request)
            )
    );
});
