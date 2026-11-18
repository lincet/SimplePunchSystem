// Service worker：只快取 app 外殼讓啟動更快；API 一律走網路、不快取。
const VERSION = 'v1';
const CACHE = `punch-shell-${VERSION}`;
const SHELL = [
  '/', '/index.html', '/manifest.webmanifest', '/css/app.css',
  '/js/app.js', '/js/api.js', '/js/util.js', '/js/camera.js',
  '/js/views/login.js', '/js/views/change-password.js', '/js/views/punch.js', '/js/views/my-records.js',
  '/js/views/shared.js', '/js/views/admin-employees.js', '/js/views/admin-employee-form.js',
  '/js/views/admin-attendance.js', '/js/views/admin-hours.js', '/js/views/admin-export.js',
  '/icons/icon-192.png', '/icons/apple-touch-icon.png',
];

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (e) => {
  const url = new URL(e.request.url);
  if (e.request.method !== 'GET' || url.origin !== location.origin || url.pathname.startsWith('/api/')) return;
  // 網路優先（確保拿到最新版），離線時才用快取
  e.respondWith(
    fetch(e.request)
      .then((res) => {
        if (res.ok) {
          const copy = res.clone();
          caches.open(CACHE).then((c) => c.put(e.request, copy));
        }
        return res;
      })
      .catch(() => caches.match(e.request).then((r) => r || caches.match('/index.html'))),
  );
});
