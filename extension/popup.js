const API_BASE = "http://localhost:65452";

const els = {
  status: document.getElementById("status"),
  form: document.getElementById("item-form"),
  name: document.getElementById("name"),
  category: document.getElementById("category"),
  target: document.getElementById("target"),
  priceInfo: document.getElementById("price-info"),
  pickBtn: document.getElementById("pick-btn"),
  saveBtn: document.getElementById("save-btn"),
  modeNewBtn: document.getElementById("mode-new-btn"),
  modeExistingBtn: document.getElementById("mode-existing-btn"),
  newFields: document.getElementById("new-fields"),
  existingFields: document.getElementById("existing-fields"),
  existingSelect: document.getElementById("existing-select"),
  findElsewhereBtn: document.getElementById("find-elsewhere-btn"),
};

const state = { url: "", selector: "", mode: "new" };

function slugify(name) {
  return (
    name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-") +
    "-" +
    Date.now().toString().slice(-4)
  );
}

function setMode(mode) {
  state.mode = mode;
  els.modeNewBtn.classList.toggle("active", mode === "new");
  els.modeExistingBtn.classList.toggle("active", mode === "existing");
  els.newFields.hidden = mode !== "new";
  els.existingFields.hidden = mode !== "existing";
}

els.modeNewBtn.addEventListener("click", () => setMode("new"));
els.modeExistingBtn.addEventListener("click", () => setMode("existing"));

async function loadExistingProducts() {
  try {
    const res = await fetch(`${API_BASE}/config`);
    if (!res.ok) return { products: [] };
    const config = await res.json();
    const products = config.items || [];

    els.existingSelect.innerHTML = "";
    if (products.length === 0) {
      const opt = document.createElement("option");
      opt.value = "";
      opt.textContent = "Nenhum produto guardado ainda";
      els.existingSelect.appendChild(opt);
    } else {
      products.forEach((p) => {
        const opt = document.createElement("option");
        opt.value = p.id;
        opt.textContent = p.name;
        els.existingSelect.appendChild(opt);
      });
    }

    return { products };
  } catch (err) {
    return { products: [] };
  }
}

// Find a product that already tracks the current tab's URL as one of its
// offers, so we know which product "Find elsewhere" should search for.
function findProductForURL(products, url) {
  for (const p of products) {
    if ((p.offers || []).some((o) => o.url === url)) return p;
  }
  return null;
}

async function init() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  state.url = tab.url;
  els.name.value = tab.title || "";

  els.status.textContent = "A detetar preço...";

  const { products } = await loadExistingProducts();
  const trackedProduct = findProductForURL(products, tab.url);
  if (trackedProduct) {
    els.findElsewhereBtn.hidden = false;
    els.findElsewhereBtn.addEventListener("click", () => {
      chrome.tabs.create({ url: `${API_BASE}/discover.html?product_id=${encodeURIComponent(trackedProduct.id)}` });
      window.close();
    });
  }

  try {
    const res = await fetch(`${API_BASE}/detect?url=${encodeURIComponent(tab.url)}`);
    if (!res.ok) throw new Error("not found");
    const data = await res.json();

    // Leave selector empty: the backend will keep re-detecting this item
    // the same way (JSON-LD/meta/known-site) on every future check.
    state.selector = "";
    if (data.name) els.name.value = data.name;

    els.status.textContent = `Preço detetado: €${Number(data.price).toFixed(2)}`;
    els.priceInfo.textContent = `Deteção automática (via ${data.source}) — o NinjaPrice vai continuar a detetar o preço da mesma forma nas próximas verificações.`;
    els.form.hidden = false;
    els.pickBtn.hidden = true;
  } catch (err) {
    els.status.textContent = "Não foi possível detetar o preço automaticamente.";
    els.pickBtn.hidden = false;
    els.form.hidden = true;
  }
}

els.pickBtn.addEventListener("click", async () => {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  await chrome.scripting.executeScript({
    target: { tabId: tab.id },
    files: ["content_script.js"],
  });
  window.close();
});

async function saveNewProduct() {
  const name = els.name.value.trim();
  if (!name) {
    els.status.textContent = "O nome é obrigatório.";
    return false;
  }

  const payload = {
    id: slugify(name),
    name,
    url: state.url,
    selector: state.selector,
    category: els.category.value || "Misc",
    target_price: parseFloat(els.target.value) || 0,
    active: true,
    alert_any_price_drop: false,
    sticky: false,
  };

  const res = await fetch(`${API_BASE}/items`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(await res.text());
  return true;
}

async function attachToExistingProduct() {
  const productID = els.existingSelect.value;
  if (!productID) {
    els.status.textContent = "Escolhe um produto existente da lista.";
    return false;
  }

  const res = await fetch(`${API_BASE}/products/${encodeURIComponent(productID)}/offers`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url: state.url, selector: state.selector }),
  });
  if (!res.ok) throw new Error(await res.text());
  return true;
}

els.saveBtn.addEventListener("click", async () => {
  els.saveBtn.textContent = "A guardar...";
  els.saveBtn.disabled = true;

  try {
    const ok = state.mode === "existing" ? await attachToExistingProduct() : await saveNewProduct();
    if (!ok) {
      els.saveBtn.textContent = "Salvar";
      els.saveBtn.disabled = false;
      return;
    }
    els.status.textContent = "✅ Guardado no NinjaPrice!";
    els.saveBtn.textContent = "Guardado";
  } catch (err) {
    els.status.textContent = `❌ Erro: ${err.message}`;
    els.saveBtn.textContent = "Tentar novamente";
    els.saveBtn.disabled = false;
  }
});

init();
