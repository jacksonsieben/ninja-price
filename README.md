# NinjaPrice 🥷💵

![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)
![Platform](https://img.shields.io/badge/Platform-Linux%20%28Fedora%29-blue?style=flat&logo=linux)
![License](https://img.shields.io/badge/License-MIT-green?style=flat)

**NinjaPrice** is a personal shopping assistant designed specifically to run silently in the background of a Linux environment (focused on Fedora). Its main goal is to monitor product prices across various online stores, consuming the absolute minimum of system resources (RAM and CPU). It tracks target prices and price drops, notifying you natively when it's the perfect time to buy.

---

## 📑 Table of Contents
1. [Features](#-features)
2. [Prerequisites](#-prerequisites)
3. [Installation](#-installation)
4. [Configuration](#-configuration)
5. [Usage](#-usage)
6. [Roadmap / To-Do](#-roadmap--to-do)
7. [Contributing](#-contributing)
8. [License](#-license)
9. [Acknowledgments / Contact](#-acknowledgments--contact)

---

## ✨ Features

- **Multi-Store Tracking:** Pull prices from any online store by configuring its URL and the exact CSS Selector for the price.
- **Ultra-Lightweight:** Written in Go, uses a minimal memory footprint and compiles to a single binary. No Docker or heavy DBs required.
- **Native OS Notifications:** Uses Linux `notify-send` for critical, sticky pop-up alerts.
- **System Tray Integration:** Runs in your desktop environment's system tray (panel) for quick manual checks and graceful exits.
- **Local JSON Storage:** Keeps track of prices cleanly in a local `prices_history.json` and sources items from `config.json`.
- **Smart Data Sanitization:** Automatically converts global price formats (like `€ 476.00` or `476,00 €`) into standard numeric formats.
- **Automatic Price Detection:** No CSS selector required for most stores — NinjaPrice reads `schema.org` JSON-LD/meta price data that most e-commerce sites already publish, with a small hardcoded fallback table for outliers (like Amazon) that don't.
- **Lightweight-first Scraping:** Tries a fast, low-RAM HTTP fetch first and only launches a full headless browser when a site's bot protection (Cloudflare, DataDome, etc.) requires it.
- **Local Mini API & Browser Extension:** Quickly add new products to monitor straight from your web browser using a small Manifest V3 extension that auto-detects the price, falling back to click-to-pick only when needed.

---

## 🛠 Prerequisites

To compile and run NinjaPrice on Fedora Linux, you need:

1. **Go (Golang)** >= 1.20
2. **libnotify** (Usually pre-installed on Fedora for `notify-send`)
3. **AppIndicator Development Libraries** (Required to compile the Go graphical systray tracker):
   ```bash
   sudo dnf install libappindicator-gtk3-devel libayatana-appindicator-gtk3-devel
   ```

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

NinjaPrice stores its tracked items in `config.json` inside the same folder as the binary. If it doesn't exist, it'll generate a blank one automatically or you can create it via the API.

Here is an example format for `config.json`:
```json
{
  "items": [
    {
      "id": "playstation-5",
      "name": "Sony PlayStation 5 Console",
      "url": "https://example-store.com/ps5",
      "selector": "",
      "category": "Gaming",
      "target_price": 399.99
    }
  ]
}
```

`selector` is optional. Leave it as `""` to let NinjaPrice auto-detect the price on every check (via JSON-LD/meta structured data, or the known-site table). Only set it if you've manually picked a specific element with the browser extension, or auto-detection doesn't work for that store.

---

## 💻 Usage

### 1. Running the Tracker
Start the application from your terminal or add it to your desktop startup applications:
```bash
./ninjaprice
```
A small icon will appear in your system tray. It will automatically re-check prices every hour. You can click the icon to manually force a **"Check Now"** or to **"Quit"**.

### 2. Installing the Browser Extension
NinjaPrice exposes a lightweight local API on `http://localhost:65452`. The `extension/` folder contains a Manifest V3 browser extension that talks to it:

1. Open `chrome://extensions` (or the equivalent in your Chromium-based browser), enable **Developer mode**.
2. Click **Load unpacked** and select the `extension/` folder in this repo.
3. Navigate to a product page on any store and click the NinjaPrice icon in your toolbar.
4. If the price is auto-detected, fill in the category/target price and click **Salvar**.
5. If it can't be auto-detected, click **Escolher preço na página** to click-to-pick the price element manually, same as before.
6. The product will be pushed cleanly to your `config.json` without needing to restart the app!

---

## 🗺 Roadmap / To-Do

- [x] External Configuration File (`config.json`).
- [x] System Tray Icon (Systray) background operations.
- [x] Local Mini API.
- [x] Browser Extension integration with automatic price detection.
- [x] Automated extraction of CSS selectors (via structured JSON-LD/meta data, no LLM needed).
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
