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

  function onClick(e) {
    e.preventDefault();
    e.stopPropagation();
    var el = e.target;
    var selector = getOptimalSelector(el);
    cleanup();
    showModal(selector, window.location.href, document.title);
  }

  function showModal(sel, url, title) {
    var modal = document.createElement("div");

    var modalHtml = '<div style="position:fixed;top:20px;right:20px;background:white;padding:20px;border-radius:12px;box-shadow:0 15px 30px rgba(0,0,0,0.4);z-index:9999999;font-family:sans-serif;color:#333;width:320px;text-align:left;border: 2px solid #00C853;">';
    modalHtml += '<h3 style="margin-top:0;font-size:18px;color:#00C853;">🥷 Salvar no NinjaPrice</h3>';
    modalHtml += '<label style="display:block;margin-top:12px;font-size:13px;font-weight:bold;">Nome do Produto:</label>';
    modalHtml += '<input id="np-name" type="text" value="' + title.replace(/"/g, "&quot;") + '" style="width:100%;padding:8px;box-sizing:border-box;border:1px solid #ccc;border-radius:6px;margin-top:4px;">';
    modalHtml += '<label style="display:block;margin-top:12px;font-size:13px;font-weight:bold;">Categoria:</label>';
    modalHtml += '<input id="np-cat" type="text" placeholder="Ex: Periféricos" style="width:100%;padding:8px;box-sizing:border-box;border:1px solid #ccc;border-radius:6px;margin-top:4px;">';
    modalHtml += '<label style="display:block;margin-top:12px;font-size:13px;font-weight:bold;">Avisar quando for menor que (€):</label>';
    modalHtml += '<input id="np-target" type="number" step="0.01" placeholder="Ex: 350.00" style="width:100%;padding:8px;box-sizing:border-box;border:1px solid #ccc;border-radius:6px;margin-top:4px;">';
    modalHtml += '<label style="display:block;margin-top:12px;font-size:13px;font-weight:bold;">Seletor CSS:</label>';
    modalHtml += '<input id="np-sel" type="text" value="' + sel.replace(/"/g, "&quot;") + '" style="width:100%;padding:8px;box-sizing:border-box;border:1px solid #ccc;border-radius:6px;margin-top:4px;font-family:monospace;font-size:11px;color:#666;">';
    modalHtml += '<input type="hidden" id="np-url" value="' + url + '">';
    modalHtml += '<div style="margin-top:20px;display:flex;justify-content:space-between;">';
    modalHtml += '<button id="np-cancel" style="padding:10px 15px;cursor:pointer;border:none;background:#f0f2f5;border-radius:6px;color:#555;font-weight:bold;">Cancelar</button>';
    modalHtml += '<button id="np-save" style="padding:10px 15px;background:#00C853;color:white;border:none;border-radius:6px;cursor:pointer;font-weight:bold;">Salvar</button>';
    modalHtml += "</div></div>";

    modal.innerHTML = modalHtml;
    document.body.appendChild(modal);

    document.getElementById("np-cancel").onclick = function () {
      modal.remove();
    };

    document.getElementById("np-save").onclick = function () {
      var btn = this;
      btn.innerText = "A enviar...";

      var payload = {
        id:
          document.getElementById("np-name").value.toLowerCase().replace(/[^a-z0-9]+/g, "-") +
          "-" +
          Date.now().toString().slice(-4),
        name: document.getElementById("np-name").value,
        url: document.getElementById("np-url").value,
        selector: document.getElementById("np-sel").value,
        category: document.getElementById("np-cat").value || "Misc",
        target_price: parseFloat(document.getElementById("np-target").value) || 0,
        active: true,
        alert_any_price_drop: false,
        sticky: false,
      };

      fetch("http://localhost:65452/items", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      })
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
