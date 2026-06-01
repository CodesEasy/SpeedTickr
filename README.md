<p align="center">
  <img src="docs/logo.png" alt="SpeedTickr" width="120" height="120">
</p>

<h1 align="center">SpeedTickr</h1>

<p align="center">
  A tiny live network speed meter for your taskbar, menu bar, and panel.<br>
  Your real-time download &amp; upload usage, always in view — on <b>Windows, macOS, and Linux</b>.
</p>

<p align="center">
  <a href="https://speedtickr.com"><b>Website</b></a> ·
  <a href="https://github.com/codeseasy/speedtickr/releases/latest"><b>Download</b></a> ·
  <a href="LICENSE">MIT License</a>
</p>

<!-- Screenshot: save a shot of the meter on your taskbar to docs/screenshot.png, then uncomment:
<p align="center"><img src="docs/screenshot.png" alt="SpeedTickr showing live download and upload speed on the taskbar" width="620"></p>
-->

## Features

- **Live speed where you can see it** — the Windows taskbar, the macOS menu bar, or the Linux top panel. Download and upload, side by side.
- **Your unit** — bps, Kbps, Mbps, Gbps, or Tbps.
- **Readable** — Small, Medium, or Large text (Windows).
- **Always on** — starts when you sign in, and you can turn that off.
- **Featherlight** — a few MB of memory and almost no CPU. One small file — no installer, no admin.
- **Private &amp; accurate** — measures locally, sends nothing anywhere, and never double-counts on a VPN.

## Install

**One command.** It picks the right build for your system, verifies its checksum, installs it for your user only (no admin), and launches it — the meter appears right away and **starts at login automatically**. There's nothing else to set up, and no installer left behind.

### Windows 10 & 11

PowerShell:

```powershell
irm https://speedtickr.com/install.ps1 | iex
```

Command Prompt (cmd):

```bat
curl -fsSL https://speedtickr.com/install.cmd -o install.cmd && install.cmd && del install.cmd
```

Your speed appears on the **taskbar**, next to the clock.

### macOS & Linux

```bash
curl -fsSL https://speedtickr.com/install.sh | sh
```

Your speed appears in the **menu bar** (macOS) or the **top panel / tray** (Linux).

<sub>Pin a version with `SPEEDTICKR_VERSION=v1.0.0`, or change where it installs with `BIN_DIR=…` (macOS/Linux) or `SPEEDTICKR_DIR=…` (Windows). Prefer to do it by hand? Download a binary and its `SHA256SUMS` from the **[releases page](https://github.com/codeseasy/speedtickr/releases/latest)**.</sub>

To **uninstall**, right-click the meter → untick **Start at login** → **Quit**, then delete the binary. Nothing else is touched.

## Using it

**Right-click** the meter (or its tray icon) to open the menu:

| Option | What it does |
| --- | --- |
| **Units** | bps · Kbps · Mbps · Gbps · Tbps |
| **Update interval** | 0.5 s · 1 s · 2 s · 5 s |
| **Font size** | Small · Medium · Large *(Windows)* |
| **Start at login** | on by default — untick to disable |
| **Quit** | close SpeedTickr |

Hover over the meter for the exact figures. Your choices are saved automatically.

## Good to know

- **Why a slim bar instead of *inside* the Windows 11 taskbar?** Microsoft removed taskbar add-ons in Windows 11, so SpeedTickr draws its own bar right on the taskbar — the closest thing that still works on Windows 10 **and** 11.
- **On a VPN?** The numbers stay correct — it follows whichever connection actually reaches the internet.
- **GNOME tray missing?** GNOME needs the [AppIndicator extension](https://extensions.gnome.org/extension/615/appindicator-support/) once for the meter to show in the top bar. KDE, XFCE, Cinnamon, and MATE work as-is.
- **Downloaded by hand instead?** A browser-downloaded build is unsigned, so Windows SmartScreen (**More info → Run anyway**) or macOS Gatekeeper (**right-click → Open**) may prompt once. The install command above isn't affected, because a `curl`/`irm` download isn't quarantined.
- **Moved the file?** If you relocate it after enabling auto-start, toggle **Start at login** off and on again.

## Build from source

Requires [Go](https://go.dev/dl/) 1.26+:

```bash
go build -ldflags="-H windowsgui" -o speedtickr.exe ./cmd/speedtickr   # Windows
go build -o speedtickr ./cmd/speedtickr                                # macOS / Linux
```

The build is pure Go (`CGO_ENABLED=0`), so every target cross-compiles from any OS — e.g.
`CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./cmd/speedtickr`. No C toolchain or SDK needed.

## License

MIT — see [LICENSE](LICENSE). Issues and pull requests welcome.
