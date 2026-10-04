// Injected into a tracked page to read the price the browser already rendered.
//
// The server-side scraper cannot reach some stores at all — pcdiga.com and
// pccomponentes.pt answer 403 to every server-side client and challenge
// headless Chrome — so for those the only place the price exists is a real
// browser session. This mirrors the server's extraction chain (JSON-LD first,
// then meta tags, then a stored selector) against the live DOM instead.
//
// The file's last expression is its result, which is what
// chrome.scripting.executeScript resolves to.
(function () {
  function toNumber(raw) {
    if (typeof raw === "number") return isFinite(raw) ? raw : null;
    if (typeof raw !== "string") return null;
    var s = raw.replace(/[^\d.,]/g, "");
    if (!s) return null;
    // Whichever separator comes last is the decimal one: "1.234,56" and
    // "1,234.56" both occur on Portuguese storefronts.
    var lastComma = s.lastIndexOf(",");
    var lastDot = s.lastIndexOf(".");
    if (lastComma > lastDot) {
      s = s.replace(/\./g, "").replace(",", ".");
    } else {
      s = s.replace(/,/g, "");
    }
    var n = parseFloat(s);
    return isFinite(n) && n > 0 ? n : null;
  }

  function unavailable(availability) {
    if (!availability || typeof availability !== "string") return false;
    var v = availability.toLowerCase();
    var slash = v.lastIndexOf("/");
    if (slash >= 0) v = v.slice(slash + 1);
    v = v.replace(/[_-]/g, "");
    return v === "outofstock" || v === "soldout" || v === "discontinued";
  }

  function fromOffers(offers) {
    var list = Array.isArray(offers) ? offers : [offers];
    var fallback = null;
    for (var i = 0; i < list.length; i++) {
      var o = list[i];
      if (!o || typeof o !== "object") continue;
      var price = toNumber(o.price);
      if (price === null && o.priceSpecification) {
        price = toNumber(o.priceSpecification.price);
      }
      if (price === null) continue;
      if (!unavailable(o.availability)) return { price: price, inStock: true };
      if (fallback === null) fallback = { price: price, inStock: false };
    }
    return fallback;
  }

  function fromNode(node) {
    if (!node || typeof node !== "object") return null;
    var type = node["@type"];
    var isProduct =
      (typeof type === "string" && type.indexOf("Product") >= 0) ||
      (Array.isArray(type) && type.some(function (t) { return String(t).indexOf("Product") >= 0; }));
    if (isProduct && node.offers) {
      var hit = fromOffers(node.offers);
      if (hit) return hit;
    }
    if (Array.isArray(node["@graph"])) {
      for (var i = 0; i < node["@graph"].length; i++) {
        var deep = fromNode(node["@graph"][i]);
        if (deep) return deep;
      }
    }
    return null;
  }

  function fromJSONLD() {
    var blocks = document.querySelectorAll('script[type="application/ld+json"]');
    for (var i = 0; i < blocks.length; i++) {
      var parsed;
      try {
        parsed = JSON.parse(blocks[i].textContent);
      } catch (e) {
        continue;
      }
      var nodes = Array.isArray(parsed) ? parsed : [parsed];
      for (var j = 0; j < nodes.length; j++) {
        var hit = fromNode(nodes[j]);
        if (hit) return { price: hit.price, inStock: hit.inStock, source: "json-ld" };
      }
    }
    return null;
  }

  function fromMeta() {
    var names = [
      'meta[property="product:price:amount"]',
      'meta[property="og:price:amount"]',
      'meta[itemprop="price"]',
      '[itemprop="price"]'
    ];
    for (var i = 0; i < names.length; i++) {
      var el = document.querySelector(names[i]);
      if (!el) continue;
      var raw = el.getAttribute("content") || el.getAttribute("data-price") || el.textContent;
      var price = toNumber(raw);
      if (price !== null) return { price: price, inStock: true, source: "meta" };
    }
    return null;
  }

  function fromSelector() {
    var sel = window.__npSelector;
    if (!sel) return null;
    var el = document.querySelector(sel);
    if (!el) return null;
    var price = toNumber(el.textContent);
    return price === null ? null : { price: price, inStock: true, source: "selector" };
  }

  var challenged = /just a moment|verifying you are human|checking your browser/i.test(
    document.title + " " + (document.body ? document.body.innerText.slice(0, 400) : "")
  );

  return fromJSONLD() || fromMeta() || fromSelector() || { price: null, challenged: challenged };
})();
