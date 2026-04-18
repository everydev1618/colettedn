#!/bin/bash
set -e

REPO="everydev1618/colettedn"
BINARY="colettedn"
INSTALL_DIR="/usr/local/bin"

# --- 1. Dependency check ---
for cmd in curl sha256sum; do
  if ! command -v "$cmd" &>/dev/null; then
    # macOS ships shasum instead of sha256sum
    if [ "$cmd" = "sha256sum" ] && command -v shasum &>/dev/null; then
      sha256sum() { shasum -a 256 "$@"; }
      export -f sha256sum
    else
      echo "Error: $cmd is required but not installed."
      exit 1
    fi
  fi
done

# --- 2. Detect OS and architecture ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Error: unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  darwin|linux) ;;
  *) echo "Error: unsupported OS: $OS"; exit 1 ;;
esac

# --- 3. Existing install detection ---
if command -v "$BINARY" &>/dev/null; then
  CURRENT=$("$BINARY" --version 2>/dev/null || echo "unknown")
  echo "ColetteDN is already installed (version: $CURRENT)"
fi

# --- 4. Fetch latest release ---
TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
if [ -z "$TAG" ]; then
  echo "Error: failed to fetch latest release from GitHub"
  exit 1
fi

if [ "${CURRENT:-}" = "$TAG" ]; then
  echo "Already up to date ($TAG). Nothing to do."
  exit 0
fi

echo "Installing ColetteDN $TAG ($OS/$ARCH)..."

# --- 5. Download binary and checksums ---
ASSET="${BINARY}-${OS}-${ARCH}"
BASE_URL="https://github.com/$REPO/releases/download/$TAG"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL "$BASE_URL/$ASSET" -o "$TMP_DIR/$ASSET"
curl -fsSL "$BASE_URL/checksums-sha256.txt" -o "$TMP_DIR/checksums-sha256.txt"

# --- 6. Checksum verification ---
EXPECTED=$(grep "$ASSET" "$TMP_DIR/checksums-sha256.txt" | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
  echo "Error: no checksum found for $ASSET in release"
  exit 1
fi

ACTUAL=$(sha256sum "$TMP_DIR/$ASSET" | awk '{print $1}')
if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "Error: checksum mismatch!"
  echo "  Expected: $EXPECTED"
  echo "  Got:      $ACTUAL"
  echo "The download may be corrupted. Aborting."
  exit 1
fi
echo "Checksum verified."

# --- 7. Install binary ---
chmod +x "$TMP_DIR/$ASSET"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_DIR/$ASSET" "$INSTALL_DIR/$BINARY"
else
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$TMP_DIR/$ASSET" "$INSTALL_DIR/$BINARY"
fi

# --- 8. Smoke test ---
INSTALLED_VERSION=$("$INSTALL_DIR/$BINARY" --version 2>/dev/null || true)
if [ "$INSTALLED_VERSION" != "$TAG" ]; then
  echo "Warning: binary installed but version check returned '$INSTALLED_VERSION' (expected '$TAG')"
  echo "The binary may not run correctly on this system."
  exit 1
fi
echo "Installed $BINARY $TAG to $INSTALL_DIR/$BINARY"

# --- 9. API key setup with validation ---
if [ -f "$HOME/.vega/env" ] && grep -q "ANTHROPIC_API_KEY=sk-" "$HOME/.vega/env" 2>/dev/null; then
  echo "API key already configured in ~/.vega/env"
else
  echo ""
  echo "You need an Anthropic API key to use ColetteDN."
  echo "Get one at: https://console.anthropic.com/"
  echo ""
  printf "Enter your API key (or press Enter to skip): "
  read -r KEY

  if [ -n "$KEY" ]; then
    # Validate the key against the Anthropic API
    echo "Validating API key..."
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
      https://api.anthropic.com/v1/messages \
      -H "x-api-key: $KEY" \
      -H "content-type: application/json" \
      -H "anthropic-version: 2023-06-01" \
      -d '{"model":"claude-sonnet-4-20250514","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}')

    if [ "$HTTP_CODE" = "200" ]; then
      echo "API key is valid."
      mkdir -p "$HOME/.vega"
      echo "ANTHROPIC_API_KEY=$KEY" > "$HOME/.vega/env"
      chmod 600 "$HOME/.vega/env"
      echo "Saved to ~/.vega/env"
    else
      echo "Warning: API key validation failed (HTTP $HTTP_CODE)."
      echo "The key may be invalid or expired. You can set it later in ~/.vega/env"
    fi
  else
    echo "Skipped. Set your key later:"
    echo "  mkdir -p ~/.vega && echo 'ANTHROPIC_API_KEY=sk-ant-...' > ~/.vega/env"
  fi
fi

echo ""
echo "You're all set! Run:"
echo "  colettedn"
echo ""
echo "Then open http://localhost:3001"
