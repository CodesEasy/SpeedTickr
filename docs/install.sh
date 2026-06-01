#!/bin/sh
# SpeedTickr installer for Linux & macOS.
#
#   curl -fsSL https://speedtickr.com/install.sh | sh
#
# Optional environment overrides:
#   SPEEDTICKR_VERSION   release tag to install (default: latest)
#   BIN_DIR              install directory   (default: $HOME/.local/bin)
#
# SpeedTickr is a single self-contained binary - this script only downloads it,
# verifies its checksum, and drops it on disk. Nothing is installed system-wide
# and no admin rights are needed.
set -eu

REPO="codeseasy/speedtickr"
VERSION="${SPEEDTICKR_VERSION:-latest}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
BIN="speedtickr"

info() { printf '\033[36m==>\033[0m %s\n' "$1"; }
warn() { printf '\033[33mwarning:\033[0m %s\n' "$1" >&2; }
die()  { printf '\033[31merror:\033[0m %s\n' "$1" >&2; exit 1; }

# Map uname -> the published release asset name.
case "$(uname -s)" in
  Linux)  os="linux" ;;
  Darwin) os="macos" ;;
  *) die "unsupported OS '$(uname -s)' - SpeedTickr runs on Linux, macOS and Windows" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64)  arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) die "unsupported architecture '$(uname -m)'" ;;
esac
asset="speedtickr-${os}-${arch}"

if [ "$VERSION" = latest ]; then
  base="https://github.com/$REPO/releases/latest/download"
else
  base="https://github.com/$REPO/releases/download/$VERSION"
fi

# Pick whichever downloader is present.
if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL "$1" -o "$2"; }
  slurp() { curl -fsSL "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -qO "$2" "$1"; }
  slurp() { wget -qO- "$1"; }
else
  die "need curl or wget to download"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

info "Downloading $asset ($VERSION)..."
fetch "$base/$asset" "$tmp/$BIN" \
  || die "download failed - is the $VERSION release published yet? ($base/$asset)"

# Verify the download against the published SHA256SUMS before trusting it.
info "Verifying checksum..."
if sums="$(slurp "$base/SHA256SUMS" 2>/dev/null)" && [ -n "$sums" ]; then
  want="$(printf '%s\n' "$sums" | awk -v a="$asset" '$2 == a {print $1}')"
  if command -v sha256sum >/dev/null 2>&1; then
    got="$(sha256sum "$tmp/$BIN" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    got="$(shasum -a 256 "$tmp/$BIN" | awk '{print $1}')"
  else
    got=""
  fi
  if [ -z "$want" ]; then
    warn "no checksum listed for $asset - skipping verification"
  elif [ -z "$got" ]; then
    warn "no sha256 tool (sha256sum/shasum) found - skipping verification"
  elif [ "$got" != "$want" ]; then
    die "checksum mismatch for $asset!
  expected: $want
  actual:   $got
The download may be corrupt or tampered with - aborting."
  else
    info "Checksum OK - $want"
  fi
else
  warn "could not download SHA256SUMS - skipping verification"
fi

chmod +x "$tmp/$BIN"
# macOS: clear any quarantine flag (curl normally doesn't set one, but be safe).
if [ "$os" = macos ]; then xattr -c "$tmp/$BIN" 2>/dev/null || true; fi

mkdir -p "$BIN_DIR"
mv -f "$tmp/$BIN" "$BIN_DIR/$BIN"
info "Installed $BIN_DIR/$BIN"

# Nudge if the install dir isn't on PATH.
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) info "Add it to your PATH:  export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac

# Launch it now. On first run SpeedTickr shows the meter and turns on
# start-at-login by itself, so this one command is all the user needs.
launch() {
  if command -v setsid >/dev/null 2>&1; then
    setsid "$1" >/dev/null 2>&1 </dev/null &
  elif command -v nohup >/dev/null 2>&1; then
    nohup "$1" >/dev/null 2>&1 </dev/null &
  else
    "$1" >/dev/null 2>&1 </dev/null &
  fi
}

# If an older copy is already running, stop it so re-running this script updates
# in place and the freshly installed version is what relaunches.
if command -v pkill >/dev/null 2>&1; then
  pkill -x "$BIN" 2>/dev/null && sleep 1 || true
fi

# Launch when a GUI/tray session is reachable: macOS always; on Linux an X11
# ($DISPLAY) or Wayland ($WAYLAND_DISPLAY) session, or at least a session D-Bus
# ($DBUS_SESSION_BUS_ADDRESS) - which is what the StatusNotifier tray actually uses.
if [ "$os" = macos ] || [ -n "${DISPLAY:-}" ] || [ -n "${WAYLAND_DISPLAY:-}" ] || [ -n "${DBUS_SESSION_BUS_ADDRESS:-}" ]; then
  launch "$BIN_DIR/$BIN"
  if [ "$os" = macos ]; then
    info "SpeedTickr is running - see your menu bar. It starts at login automatically."
  else
    info "SpeedTickr is running - see your top panel / tray. It starts at login automatically."
  fi
else
  info "Installed. No desktop session detected - launch it from your desktop with:  $BIN"
fi
