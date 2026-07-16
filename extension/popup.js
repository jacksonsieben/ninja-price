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
};

const state = { url: "", selector: "" };

function slugify(name) {
  return (
    name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-") +
    "-" +
    Date.now().toString().slice(-4)
  );
}

async function init() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  state.url = tab.url;
  els.name.value = tab.title || "";

  els.status.textContent = "A detetar preço...";

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

els.saveBtn.addEventListener("click", async () => {
  const name = els.name.value.trim();
  if (!name) {
    els.status.textContent = "O nome é obrigatório.";
    return;
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

  els.saveBtn.textContent = "A guardar...";
  els.saveBtn.disabled = true;

  try {
    const res = await fetch(`${API_BASE}/items`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error(await res.text());
    els.status.textContent = "✅ Guardado no NinjaPrice!";
    els.saveBtn.textContent = "Guardado";
  } catch (err) {
    els.status.textContent = `❌ Erro: ${err.message}`;
    els.saveBtn.textContent = "Tentar novamente";
    els.saveBtn.disabled = false;
  }
});

init();
