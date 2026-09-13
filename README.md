# ⚡ Gemini Swap

> **Multi-Account Switcher & Quota Monitor for Gemini**
> Native macOS Swift App (with Menu Bar / Upper Bar Mode) & Cross-Platform VPS CLI (Linux, macOS, Windows).

[![GitHub Release](https://img.shields.io/github/v/release/Leu3ery/gemini-swap?color=blue)](https://github.com/Leu3ery/gemini-swap/releases)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)]()
[![License](https://img.shields.io/badge/license-MIT-green)]()

---

## ✨ Features

- 🔄 **Zero-Friction Account Swapping**: Switch active Gemini accounts with 1 click in the macOS app or 1 terminal command (`gemini-swap switch <account>`) without logging out and re-authenticating every time.
- 📊 **Real-Time Quota & Usage Tracking**: Direct integration with Google Cloud Code Assist & Generative Language APIs. Displays remaining percentages, exact token/request counts, and reset countdowns for Gemini 2.5 Flash, Gemini 2.5 Pro, and Gemini 3.0 models.
- 🖥️ **Native macOS Swift App**:
  - Designed according to Apple Human Interface Guidelines.
  - **Upper Bar / Menu Bar Toggle**: Switch between full dashboard mode and a discreet Menu Bar popover docked to the top of your Mac screen.
- ⚡ **Cross-Platform CLI for VPS**:
  - Zero-dependency single static binaries for **Linux (x64 & ARM64)**, **macOS (Apple Silicon & Intel)**, and **Windows (x64)**.
  - Interactive browser login on desktop + headless copy-paste auth code mode on remote servers.
- 🤖 **Concurrent Use & Codex / AI Proxy**:
  - Shared state store (`~/.gemini-swap/`) with live file-watching: actions in the CLI or Codex immediately update the macOS app in real time!
  - Built-in local proxy (`http://127.0.0.1:8045/v1`) with **automatic rate limit (429) failover**: if an account exhausts its quota, requests automatically rotate to the next available account without failing your tools or IDE agents!
- 🌐 **Embedded VPS Web Dashboard**:
  - Run `gemini-swap web --port 8080` to view a responsive visual dashboard on any remote headless Linux VPS in your web browser.

---

## 📥 Installation

### macOS (Native Swift App & CLI)

1. Go to the [Releases](https://github.com/Leu3ery/gemini-swap/releases) page.
2. Download **`GeminiSwap-macOS.zip`**.
3. Unzip and move `GeminiSwap.app` to your `/Applications` folder.
4. *(Optional CLI)*: Download `gemini-swap-darwin-arm64.tar.gz`, extract and place `gemini-swap` into `/usr/local/bin`.

### Linux (VPS / Headless Server)

```bash
# Download and install the latest binary (amd64)
curl -sL https://github.com/Leu3ery/gemini-swap/releases/latest/download/gemini-swap-linux-amd64.tar.gz | tar -xz
sudo mv gemini-swap-linux-amd64 /usr/local/bin/gemini-swap
chmod +x /usr/local/bin/gemini-swap
```

### Windows

1. Download **`gemini-swap-windows-amd64.zip`** from [Releases](https://github.com/Leu3ery/gemini-swap/releases).
2. Extract `gemini-swap-windows-amd64.exe` to a folder in your `PATH`.

---

## 🚀 Quick Start

### 1. Launch & Auto-Import

On first launch, Gemini Swap automatically detects and imports your existing Google account from `~/.gemini/`!

```bash
# View all accounts & quotas
gemini-swap list
```

### 2. Add Accounts

```bash
# Add a Google account via browser OAuth:
gemini-swap login

# Add a Google account on a remote headless VPS:
gemini-swap login --headless

# Add a Google AI Studio API Key:
gemini-swap add-key --name "Personal Studio Key" --key "AIzaSy..."
```

### 3. Switch Active Account

```bash
# Switch by email or name:
gemini-swap switch user@gmail.com
```
*Swapping automatically synchronizes `~/.gemini/oauth_creds.json` and `~/.gemini/google_accounts.json` so Gemini CLI, Antigravity, and other tools switch instantly.*

### 4. Check Quotas & Usage

```bash
gemini-swap quota
```

Output:
```
Fetching latest usage & quotas from Google...

=== user@gmail.com (oauth) ===
  Tier: Gemini Code Assist
  gemini-2.5-flash     [████████████████████] 100% (1000 / 1000)
    Reset Time: 2026-09-14T19:00:00Z
  gemini-2.5-pro       [████████████████████] 100% (50 / 50)
    Reset Time: 2026-09-14T19:00:00Z
```

---

## 🤖 Codex & AI Agent Integration

Use Gemini Swap concurrently with Codex, Cline, Cursor, or terminal commands:

### Method 1: Environment Injection (`gemini-swap exec`)

Run any tool with active account credentials pre-configured:

```bash
gemini-swap exec -- codex run
```

Or export credentials into your current shell session:
```bash
eval $(gemini-swap current --env)
```

### Method 2: Local AI Proxy with Auto-Failover

Start the local proxy:
```bash
gemini-swap proxy --port 8045
```

Point Codex or any OpenAI/Gemini compatible client to `http://127.0.0.1:8045/v1`.
- If an account hits a **429 Rate Limit**, the proxy automatically fails over to your other accounts!
- Live request counts and token usage are monitored in real time inside the macOS app.

---

## 💻 macOS Upper Bar / Menu Bar Mode

In the macOS app:
- Click **"Upper Bar Mode"** in the top right to dock the app into your macOS status bar.
- Clicking the Menu Bar icon reveals a compact popover with active quota meters and 1-click account switching.
- Click **"Dashboard"** anytime to bring back the full window.

---

## 🛠️ Building from Source

### Requirements
- Go 1.22+
- Xcode 15+ & Swift 5.9+ (for macOS app)

```bash
git clone https://github.com/Leu3ery/gemini-swap.git
cd gemini-swap

# Build CLI
go build -o bin/gemini-swap ./cmd/gemini-swap

# Build native macOS App
./scripts/build-app.sh

# Cross-compile all platforms
./scripts/build-cli.sh
```

---

## 📄 License

MIT License. See [LICENSE](LICENSE) for details.
