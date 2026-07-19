# NinjaPrice 🥷💵

![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)
![Platform](https://img.shields.io/badge/Platform-Linux%20%28Fedora%29-blue?style=flat&logo=linux)
![License](https://img.shields.io/badge/License-MIT-green?style=flat)

**NinjaPrice** is a personal shopping assistant designed specifically to run silently in the background of a Linux environment (focused on Fedora). Its main goal is to monitor product prices across various online stores, consuming the absolute minimum of system resources (RAM and CPU). It tracks a product across every store you add it to, and notifies you natively (and optionally by email) whenever the best price across those stores hits your target or drops.

---

## 📑 Table of Contents
1. [Features](#-features)
2. [Prerequisites](#-prerequisites)
3. [Installation](#-installation)
4. [Configuration](#-configuration)
5. [Usage](#-usage)
6. [LLM-Assisted Discovery](#-llm-assisted-discovery)
7. [Roadmap / To-Do](#-roadmap--to-do)
8. [Contributing](#-contributing)
9. [License](#-license)
10. [Acknowledgments / Contact](#-acknowledgments--contact)

---

## ✨ Features

- **One Product, Many Stores:** Products aren't tied to a single URL — each product tracks a list of *offers* (one per store), and the dashboard/notifications always work off the best (lowest) price across all of them.
- **Ultra-Lightweight:** Written in Go, uses a minimal memory footprint and compiles to a single binary. No Docker or heavy DBs required.
- **Native OS + Email Notifications:** Uses Linux `notify-send` for critical, sticky pop-up alerts, with optional per-product email notifications over SMTP.
- **System Tray Integration:** Runs in your desktop environment's system tray (panel) for quick manual checks and graceful exits.
- **Local JSON Storage:** Keeps track of prices cleanly in a local `prices_history.json` and sources products from `config.json`.
- **Smart Data Sanitization:** Automatically converts global price formats (like `€ 476.00` or `476,00 €`) into standard numeric formats.
- **Automatic Price Detection:** No CSS selector required for most stores — NinjaPrice reads `schema.org` JSON-LD/meta price data that most e-commerce sites already publish, with a small hardcoded fallback table for outliers (like Amazon) that don't.
- **Lightweight-first Scraping:** Tries a fast, low-RAM HTTP fetch first and only launches a full headless browser when a site's bot protection (Cloudflare, DataDome, etc.) requires it.
- **Instant Price on Add:** Newly created products/offers are scraped immediately when added (via the extension, `discover.html`, or the API), so the dashboard shows a real price right away instead of waiting for the next hourly check.
- **Local Mini API & Browser Extension:** Add new products straight from your web browser using a small Manifest V3 extension that auto-detects the price, falling back to click-to-pick only when needed — and lets you either create a new product or attach the page as another offer on an existing one.
- **Optional LLM-Assisted Discovery:** An on-demand "find this elsewhere" flow that asks a pluggable LLM provider to locate the same product on other stores, verifies every candidate through the same scraper pipeline before showing it to you, and never saves anything without your approval. Fully optional — manual mode works with zero LLM dependency.

---

## 🛠 Prerequisites

To compile and run NinjaPrice on Fedora Linux, you need:

1. **Go (Golang)** >= 1.24
2. **libnotify** (Usually pre-installed on Fedora for `notify-send`)
3. **AppIndicator Development Libraries** (Required to compile the Go graphical systray tracker):
   ```bash
   sudo dnf install libappindicator-gtk3-devel libayatana-appindicator-gtk3-devel
   ```
4. *(Optional, for LLM-assisted discovery only)* One of: the [Claude Code CLI](https://code.claude.com) already logged into a subscription (`claude_cli`, the default), an `ANTHROPIC_API_KEY` (`anthropic`), or a local/self-hosted OpenAI-compatible endpoint like Ollama or LM Studio (`openai_compatible`). See [LLM-Assisted Discovery](#-llm-assisted-discovery) below.

---

## 🚀 Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/jacksonsieben/ninja-price.git
   cd ninja-price
   ```

2. **Download dependencies:**
   ```bash
   go mod tidy
   ```

3. **Build the binary:**
   ```bash
   CGO_CFLAGS="-Wno-deprecated-declarations" go build -o ninjaprice
   ```
   *(Note: The `CGO_CFLAGS` suppresses a benign C-level deprecation warning stemming from the system tray C library inside Gnome/Fedora.)*

---

## ⚙️ Configuration

NinjaPrice stores its tracked products in `config.json` inside the same folder as the binary. If it doesn't exist, it'll generate a blank one automatically (with LLM discovery defaulted on via `claude_cli`) or you can create it via the API.

Here is an example format for `config.json`:
```json
{
  "cooldown_period": 5,
  "items": [
    {
      "id": "playstation-5",
      "name": "Sony PlayStation 5 Console",
      "category": "Gaming",
      "target_price": 399.99,
      "active": true,
      "alert_any_price_drop": false,
      "sticky": false,
      "notify_email": false,
      "offers": [
        {
          "id": "playstation-5-amazon-es",
          "store": "amazon.es",
          "url": "https://example-store.com/ps5",
          "selector": ""
        }
      ]
    }
  ],
  "llm": {
    "provider": "claude_cli",
    "model": "haiku",
    "cli_path": "claude",
    "api_key_env": "ANTHROPIC_API_KEY",
    "discover_stores": ["amazon.es", "worten.pt", "fnac.pt", "pcdiga.com"]
  },
  "smtp": {
    "host": "smtp.gmail.com",
    "port": 587,
    "username": "you@gmail.com",
    "password_env": "NINJAPRICE_SMTP_PASSWORD",
    "from": "you@gmail.com",
    "to": "you@gmail.com"
  }
}
```

Key fields:
- **`offers[].selector`** is optional. Leave it as `""` to let NinjaPrice auto-detect the price on every check (via JSON-LD/meta structured data, or the known-site table). Only set it if you've manually picked a specific element with the browser extension, or auto-detection doesn't work for that store.
- **`notify_email`** opts a single product into email notifications. It's silently ignored if the top-level `smtp` block is missing/incomplete — desktop notifications keep working regardless.
- **`smtp.password_env`** names an environment variable holding the SMTP password/app-password (e.g. `export NINJAPRICE_SMTP_PASSWORD=...` in your shell profile) rather than storing the plaintext password in `config.json`, which is tracked in this repo.
- **`llm`** is entirely optional — see [LLM-Assisted Discovery](#-llm-assisted-discovery). Set `"provider": ""` (or omit the block after it's been created once) to disable it; manual multi-store tracking works fine either way.

---

## 💻 Usage

### 1. Running the Tracker
Start the application from your terminal or add it to your desktop startup applications:
```bash
./ninjaprice
```
The NinjaPrice icon (`assets/ninja-price-logo-systray.png`) will appear in your system tray. It will automatically re-check prices every hour. You can click the icon to manually force a **"Check Now"** or to **"Quit"**.

### 2. The Dashboard
Open `http://localhost:65452` in your browser for a live view of every tracked product, with each store's offer and current price listed underneath it (the cheapest one highlighted). From here you can edit a product, toggle it active/inactive, opt it into email notifications, delete it, or jump into **Descobrir** (discovery) for any product to search for it on other stores.

### 3. Installing the Browser Extension
NinjaPrice exposes a lightweight local API on `http://localhost:65452`. The `extension/` folder contains a Manifest V3 browser extension that talks to it:

1. Open `chrome://extensions` (or the equivalent in your Chromium-based browser), enable **Developer mode**.
2. Click **Load unpacked** and select the `extension/` folder in this repo.
3. Navigate to a product page on any store and click the NinjaPrice icon in your toolbar.
4. Choose **Novo Produto** to track it as a brand-new product, or **Produto Existente** to attach this page as another store offer on a product you already track (picked from a dropdown).
5. If the price is auto-detected, fill in the details and click **Salvar**. If it can't be auto-detected, click **Escolher preço na página** to click-to-pick the price element manually — the same new-vs-existing choice is available there too.
6. The product/offer is pushed straight to `config.json`, scraped immediately for its first price, and shows up on the dashboard — no restart needed.
7. If the current page is already tracked as an offer, a **Find elsewhere** button appears in the popup — it opens `discover.html` for that product so you can search for it on other stores (see below).

---

## 🤖 LLM-Assisted Discovery

Beyond manually attaching offers, NinjaPrice can ask an LLM to find the same product on other stores for you, verifying every result through the same price-detection pipeline used everywhere else before ever showing it to you — nothing is auto-saved.

- **Standalone**, at `http://localhost:65452/discover.html`: type a product name to bootstrap a brand-new product purely from search results.
- **From an existing product**, via the dashboard's **Descobrir** link or the extension's **Find elsewhere** button: searches for that product specifically, skipping stores you're already tracking.

The LLM backend is pluggable via `config.json`'s `llm` block — pick exactly one, it's not switched dynamically per request:

| `provider` | How it works | Cost |
|---|---|---|
| `claude_cli` *(default)* | Shells out to your local `claude` CLI in non-interactive mode | Rides your existing Claude Pro/Max subscription — no extra billing |
| `anthropic` | Calls the Anthropic API directly (`api_key_env`, default model `claude-haiku-4-5`) | Pay-per-token, billed separately from any subscription |
| `openai_compatible` | Plain HTTP to any `/chat/completions`-compatible endpoint (Ollama, LM Studio, vLLM, OpenRouter, `base_url`) | Free if self-hosted; verification (`MatchProduct`) only — no built-in web search, so `Discover` isn't supported for this provider |
| `""` | Disables the feature entirely | — |

If discovery ever comes back empty or fails, check the terminal/log output where `ninjaprice` is running — every step (search query sent, candidate URLs returned, why each candidate was skipped) is logged there.

---

## 🗺 Roadmap / To-Do

- [x] External Configuration File (`config.json`).
- [x] System Tray Icon (Systray) background operations.
- [x] Local Mini API.
- [x] Browser Extension integration with automatic price detection.
- [x] Automated extraction of CSS selectors (via structured JSON-LD/meta data, no LLM needed).
- [x] Multi-store product tracking (one product, many offers).
- [x] Pluggable LLM-assisted store discovery.
- [x] Email notifications via SMTP, opt-in per product.
- [ ] Add history charting/visualizations for price variations over time.
- [ ] Implement robust error retry backoffs for rate-limited stores.

---

## 🤝 Contributing

Contributions make the open-source community an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

---

## 📜 License

Distributed under the MIT License. See `LICENSE` for more information.

---

## 📫 Acknowledgments / Contact

**Jackson Sieben** - [GitHub Profile](https://github.com/jacksonsieben)

Libraries used in this project:
- [PuerkitoBio/goquery](https://github.com/PuerkitoBio/goquery) - For easy HTML page scraping and structured-data extraction.
- [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) - Browser-fingerprinted HTTP client used for the lightweight fetch tier.
- [chromedp/chromedp](https://github.com/chromedp/chromedp) - Headless browser fallback for bot-protected stores.
- [getlantern/systray](https://github.com/getlantern/systray) - For the background system tray integration.
- [anthropics/anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) - Official Anthropic Go SDK, used by the `anthropic` LLM discovery provider.
