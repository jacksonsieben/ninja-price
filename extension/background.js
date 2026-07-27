const API_BASE = "http://localhost:65452";

// Content scripts run inside the target page, so their own fetch() calls are
// subject to that page's Content-Security-Policy (connect-src) and get
// silently blocked on sites that restrict it. The background service worker
// runs at the extension's own origin and isn't affected by page CSP, so
// content_script.js relays every API call here instead of fetching directly.
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (!msg || msg.type !== "NP_FETCH") return false;

  // no-store so the extension always sees the current config/history rather
  // than a stale browser-cached copy (a freshly added product would otherwise
  // be missing from the "existing product" dropdown).
  const options = { method: msg.method || "GET", cache: "no-store" };
  if (msg.body !== undefined) {
    options.headers = { "Content-Type": "application/json" };
    options.body = JSON.stringify(msg.body);
  }

  fetch(API_BASE + msg.path, options)
    .then(async (res) => {
      const text = await res.text();
      sendResponse({ ok: res.ok, status: res.status, text });
    })
    .catch((err) => {
      sendResponse({ ok: false, status: 0, text: String(err) });
    });

  return true; // keep the message channel open for the async sendResponse
});
