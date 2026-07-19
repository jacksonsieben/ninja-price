(function () {
  var style = document.createElement("style");
  style.innerHTML =
    ".ninja-picker-hover{outline: 3px solid #00C853 !important; cursor: crosshair !important; background: rgba(0, 200, 83, 0.2) !important;}";
  document.head.appendChild(style);
  var body = document.body;
  var lastElement = null;

  function onMouseOver(e) {
    if (lastElement) lastElement.classList.remove("ninja-picker-hover");
    e.target.classList.add("ninja-picker-hover");
    lastElement = e.target;
  }

  function onMouseOut(e) {
    e.target.classList.remove("ninja-picker-hover");
  }

  // Walks up from the clicked element looking for a stable, short selector:
  // an id, then a known "anchor" attribute (data-testid, itemprop, etc.),
  // falling back to a positional path as a last resort.
  function getOptimalSelector(el) {
    var path = [];
    var preferredAttrs = ["data-test-id", "data-testid", "data-cy", "itemprop", "data-price"];

    while (el && el.nodeType === Node.ELEMENT_NODE && el.tagName !== "HTML" && el.tagName !== "BODY") {
      var selector = el.tagName.toLowerCase();

      if (el.id && !/^\d+$/.test(el.id)) {
        path.unshift(selector + "#" + el.id);
        break;
      }

      var foundAttr = false;
      for (var i = 0; i < preferredAttrs.length; i++) {
        var attr = preferredAttrs[i];
        if (el.hasAttribute(attr)) {
          selector += "[" + attr + '="' + el.getAttribute(attr) + '"]';
          foundAttr = true;
          break;
        }
      }

      if (foundAttr) {
        path.unshift(selector);
        break;
      }

      var siblings = Array.from(el.parentNode.children).filter(function (e) {
        return e.tagName === el.tagName;
      });
      if (siblings.length > 1) {
        var index = siblings.indexOf(el) + 1;
        selector += ":nth-of-type(" + index + ")";
      }

      path.unshift(selector);
      el = el.parentNode;
    }
    return path.join(" > ");
  }

  // Relays a request through the background service worker instead of
  // fetching directly, since fetch() from a content script runs in the
  // target page's context and is subject to that page's CSP — some sites
  // silently block it while others (e.g. pccomponentes) don't.
  function npFetch(path, options) {
    options = options || {};
    return new Promise(function (resolve, reject) {
      chrome.runtime.sendMessage(
        { type: "NP_FETCH", path: path, method: options.method || "GET", body: options.body },
        function (response) {
          if (chrome.runtime.lastError) {
            reject(new Error(chrome.runtime.lastError.message));
            return;
          }
          if (!response) {
            reject(new Error("no response from background worker"));
            return;
          }
          resolve({
            ok: response.ok,
            status: response.status,
            text: function () {
              return Promise.resolve(response.text);
            },
            json: function () {
              return Promise.resolve(response.text ? JSON.parse(response.text) : null);
            },
          });
        }
      );
    });
  }

  function onClick(e) {
    e.preventDefault();
    e.stopPropagation();
    var el = e.target;
    var selector = getOptimalSelector(el);
    cleanup();
    showModal(selector, window.location.href, document.title);
  }

  function showModal(sel, url, title) {
    var mode = "new";

    var modal = document.createElement("div");

    var modalHtml = '<div style="position:fixed;top:20px;right:20px;background:white;padding:20px;border-radius:12px;box-shadow:0 15px 30px rgba(0,0,0,0.4);z-index:9999999;font-family:sans-serif;color:#333;width:320px;text-align:left;border: 2px solid #00C853;">';
    modalHtml += '<h3 style="margin-top:0;font-size:18px;color:#00C853;">🥷 Salvar no NinjaPrice</h3>';

    modalHtml += '<div style="display:flex;gap:6px;margin-top:12px;">';
    modalHtml += '<button type="button" id="np-mode-new" style="flex:1;padding:8px;font-size:12px;border:none;border-radius:6px;background:#00C853;color:white;font-weight:bold;cursor:pointer;">Novo Produto</button>';
    modalHtml += '<button type="button" id="np-mode-existing" style="flex:1;padding:8px;font-size:12px;border:none;border-radius:6px;background:#f0f2f5;color:#555;font-weight:bold;cursor:pointer;">Produto Existente</button>';
    modalHtml += "</div>";

    modalHtml += '<div id="np-new-fields">';
    modalHtml += '<label style="display:block;margin-top:12px;font-size:13px;font-weight:bold;">Nome do Produto:</label>';
    modalHtml += '<input id="np-name" type="text" value="' + title.replace(/"/g, "&quot;") + '" style="width:100%;padding:8px;box-sizing:border-box;border:1px solid #ccc;border-radius:6px;margin-top:4px;">';
    modalHtml += '<label style="display:block;margin-top:12px;font-size:13px;font-weight:bold;">Categoria:</label>';
    modalHtml += '<input id="np-cat" type="text" placeholder="Ex: Periféricos" style="width:100%;padding:8px;box-sizing:border-box;border:1px solid #ccc;border-radius:6px;margin-top:4px;">';
    modalHtml += '<label style="display:block;margin-top:12px;font-size:13px;font-weight:bold;">Avisar quando for menor que (€):</label>';
    modalHtml += '<input id="np-target" type="number" step="0.01" placeholder="Ex: 350.00" style="width:100%;padding:8px;box-sizing:border-box;border:1px solid #ccc;border-radius:6px;margin-top:4px;">';
    modalHtml += "</div>";

    modalHtml += '<div id="np-existing-fields" style="display:none;">';
    modalHtml += '<label style="display:block;margin-top:12px;font-size:13px;font-weight:bold;">Associar a:</label>';
    modalHtml += '<select id="np-existing-select" style="width:100%;padding:8px;box-sizing:border-box;border:1px solid #ccc;border-radius:6px;margin-top:4px;font-family:inherit;">';
    modalHtml += '<option value="">A carregar produtos...</option>';
    modalHtml += "</select>";
    modalHtml += "</div>";

    modalHtml += '<label style="display:block;margin-top:12px;font-size:13px;font-weight:bold;">Seletor CSS:</label>';
    modalHtml += '<input id="np-sel" type="text" value="' + sel.replace(/"/g, "&quot;") + '" style="width:100%;padding:8px;box-sizing:border-box;border:1px solid #ccc;border-radius:6px;margin-top:4px;font-family:monospace;font-size:11px;color:#666;">';
    modalHtml += '<input type="hidden" id="np-url" value="' + url + '">';
    modalHtml += '<div style="margin-top:20px;display:flex;justify-content:space-between;">';
    modalHtml += '<button id="np-cancel" style="padding:10px 15px;cursor:pointer;border:none;background:#f0f2f5;border-radius:6px;color:#555;font-weight:bold;">Cancelar</button>';
    modalHtml += '<button id="np-save" style="padding:10px 15px;background:#00C853;color:white;border:none;border-radius:6px;cursor:pointer;font-weight:bold;">Salvar</button>';
    modalHtml += "</div></div>";

    modal.innerHTML = modalHtml;
    document.body.appendChild(modal);

    function setMode(newMode) {
      mode = newMode;
      var isNew = newMode === "new";
      document.getElementById("np-mode-new").style.background = isNew ? "#00C853" : "#f0f2f5";
      document.getElementById("np-mode-new").style.color = isNew ? "white" : "#555";
      document.getElementById("np-mode-existing").style.background = isNew ? "#f0f2f5" : "#00C853";
      document.getElementById("np-mode-existing").style.color = isNew ? "#555" : "white";
      document.getElementById("np-new-fields").style.display = isNew ? "block" : "none";
      document.getElementById("np-existing-fields").style.display = isNew ? "none" : "block";
    }

    document.getElementById("np-mode-new").onclick = function () {
      setMode("new");
    };
    document.getElementById("np-mode-existing").onclick = function () {
      setMode("existing");
    };

    // Populate the "associate with existing product" dropdown in the background.
    npFetch("/config")
      .then(function (res) {
        return res.ok ? res.json() : { items: [] };
      })
      .then(function (cfg) {
        var products = cfg.items || [];
        var select = document.getElementById("np-existing-select");
        select.innerHTML = "";
        if (products.length === 0) {
          var empty = document.createElement("option");
          empty.value = "";
          empty.textContent = "Nenhum produto guardado ainda";
          select.appendChild(empty);
          return;
        }
        products.forEach(function (p) {
          var opt = document.createElement("option");
          opt.value = p.id;
          opt.textContent = p.name;
          select.appendChild(opt);
        });
      })
      .catch(function () {});

    document.getElementById("np-cancel").onclick = function () {
      modal.remove();
    };

    document.getElementById("np-save").onclick = function () {
      var btn = this;

      var offerURL = document.getElementById("np-url").value;
      var offerSelector = document.getElementById("np-sel").value;

      var request;
      if (mode === "existing") {
        var productID = document.getElementById("np-existing-select").value;
        if (!productID) {
          alert("Escolhe um produto existente da lista.");
          return;
        }
        request = npFetch("/products/" + encodeURIComponent(productID) + "/offers", {
          method: "POST",
          body: { url: offerURL, selector: offerSelector },
        });
      } else {
        var payload = {
          id:
            document.getElementById("np-name").value.toLowerCase().replace(/[^a-z0-9]+/g, "-") +
            "-" +
            Date.now().toString().slice(-4),
          name: document.getElementById("np-name").value,
          url: offerURL,
          selector: offerSelector,
          category: document.getElementById("np-cat").value || "Misc",
          target_price: parseFloat(document.getElementById("np-target").value) || 0,
          active: true,
          alert_any_price_drop: false,
          sticky: false,
        };
        request = npFetch("/items", {
          method: "POST",
          body: payload,
        });
      }

      btn.innerText = "A enviar...";

      request
        .then(function (res) {
          if (res.ok) {
            alert("✅ Produto guardado no NinjaPrice!");
            modal.remove();
          } else {
            res.text().then(function (text) {
              alert("❌ Erro a salvar: " + text);
              btn.innerText = "Tentar Novamente";
            });
          }
        })
        .catch(function () {
          alert("❌ Servidor não encontrado. Confirma que o NinjaPrice está a correr.");
          btn.innerText = "Salvar";
        });
    };
  }

  function cleanup() {
    body.removeEventListener("mouseover", onMouseOver);
    body.removeEventListener("mouseout", onMouseOut);
    body.removeEventListener("click", onClick);
    if (lastElement) lastElement.classList.remove("ninja-picker-hover");
    style.remove();
  }

  body.addEventListener("mouseover", onMouseOver);
  body.addEventListener("mouseout", onMouseOut);
  body.addEventListener("click", onClick, { once: true });
})();
