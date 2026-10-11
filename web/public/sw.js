// Content-free push (FR-026). Pushes carry no payload; the wording is the member's own,
// saved on this device. No logo, no name, no excerpt.
const STORE = "notify-settings";
const KEY = "/__notify_text";

async function wording() {
  try {
    const c = await caches.open(STORE);
    const r = await c.match(KEY);
    if (r) return await r.text();
  } catch (e) {
    /* fall through */
  }
  return "You have an update";
}

self.addEventListener("push", (event) => {
  event.waitUntil(wording().then((text) => self.registration.showNotification(text, { tag: "update", renotify: false })));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  event.waitUntil(self.clients.openWindow("/"));
});
