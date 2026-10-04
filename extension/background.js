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

// ---------------------------------------------------------------------------
// Price reporting
//
// Some stores cannot be read by the tracker process at all: pcdiga.com and
// pccomponentes.pt answer 403 to every server-side client, challenge headless
// Chrome, and 403 even the sitemaps their own robots.txt advertises. Their
// prices exist only inside a real browser session, which is what this is.
//
// Nothing here circumvents anything: the page is loaded by the user's own
// browser, the way it would be if they opened it themselves, and the figure
// read is the one already rendered on screen.
// ---------------------------------------------------------------------------

const TRACKED_TTL_MS = 5 * 60 * 1000;
let trackedCache = { at: 0, offers: [] };
let refreshState = { running: false, done: 0, total: 0, reported: 0, failed: 0 };

function canonical(raw) {
  if (!raw) return "";
  const cut = raw.search(/[?#]/);
  return (cut >= 0 ? raw.slice(0, cut) : raw).replace(/\/+$/, "");
}

async function apiJSON(path, options) {
  const res = await fetch(API_BASE + path, Object.assign({ cache: "no-store" }, options));
  if (!res.ok) throw new Error(path + " -> HTTP " + res.status);
  return res.json();
}

async function getTracked(force) {
  const fresh = Date.now() - trackedCache.at < TRACKED_TTL_MS;
  if (!force && fresh && trackedCache.offers.length) return trackedCache.offers;
  const offers = await apiJSON("/tracked");
  trackedCache = { at: Date.now(), offers };
  return offers;
}

function matchTracked(offers, url) {
  const want = canonical(url);
  return offers.find((o) => canonical(o.url) === want) || null;
}

// readPrice injects the reader and polls it. A Cloudflare-protected page first
// serves the challenge and only then the product, so a single read right after
// "complete" would see "Just a moment..." and nothing else.
async function readPrice(tabId, selector, attempts = 8, gapMs = 1500) {
  for (let i = 0; i < attempts; i++) {
    try {
      await chrome.scripting.executeScript({
        target: { tabId },
        func: (sel) => { window.__npSelector = sel || null; },
        args: [selector || null],
      });
      const [{ result }] = await chrome.scripting.executeScript({
        target: { tabId },
        files: ["reporter.js"],
      });
      if (result && result.price) return result;
    } catch (e) {
      // Tab navigating or not injectable yet; another attempt is cheaper than
      // deciding here that it never will be.
    }
    await new Promise((r) => setTimeout(r, gapMs));
  }
  return null;
}

async function reportPrice(url, price) {
  return apiJSON("/report", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url, price }),
  });
}

// Opportunistic: any time the user lands on a page that is already tracked, its
// price is refreshed. Costs them nothing and keeps the blocked stores current
// just by being browsed.
chrome.tabs.onUpdated.addListener(async (tabId, changeInfo, tab) => {
  if (changeInfo.status !== "complete" || !tab.url || !/^https?:/.test(tab.url)) return;
  if (refreshState.running) return; // the sweep handles its own tabs
  try {
    const offer = matchTracked(await getTracked(false), tab.url);
    if (!offer) return;
    const read = await readPrice(tabId, offer.selector, 3, 1200);
    if (!read || !read.price || read.inStock === false) return;
    const out = await reportPrice(offer.url, read.price);
    console.log("[NinjaPrice] reported", offer.store, read.price, out);
  } catch (e) {
    console.log("[NinjaPrice] opportunistic report skipped:", String(e));
  }
});

// The sweep: open every tracked offer in a background tab, read it, report it,
// close it. Sequential on purpose - a dozen stores opened at once looks like
// exactly the traffic pattern these sites are protecting themselves against.
async function refreshAll() {
  if (refreshState.running) return refreshState;
  const offers = await getTracked(true);
  refreshState = { running: true, done: 0, total: offers.length, reported: 0, failed: 0 };

  for (const offer of offers) {
    let tabId = null;
    try {
      const tab = await chrome.tabs.create({ url: offer.url, active: false });
      tabId = tab.id;
      await new Promise((r) => setTimeout(r, 2500));
      const read = await readPrice(tabId, offer.selector);
      if (read && read.price && read.inStock !== false) {
        await reportPrice(offer.url, read.price);
        refreshState.reported++;
      } else {
        refreshState.failed++;
      }
    } catch (e) {
      refreshState.failed++;
    } finally {
      if (tabId !== null) {
        try { await chrome.tabs.remove(tabId); } catch (e) {}
      }
      refreshState.done++;
    }
  }

  refreshState.running = false;
  return refreshState;
}

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (!msg) return false;
  if (msg.type === "NP_REFRESH_ALL") {
    refreshAll().then(() => sendResponse(refreshState));
    return true;
  }
  if (msg.type === "NP_REFRESH_STATE") {
    sendResponse(refreshState);
    return false;
  }
  return false;
});
