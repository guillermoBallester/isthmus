#!/bin/sh
# Isthmus installer — https://isthmus.dev
#
# Usage:
#   curl -fsSL https://isthmus.dev/install.sh | sh
#
# What it does:
#   1. Detects OS and architecture
#   2. Installs via `go install` if Go >= 1.25 is available
#   3. Verifies the binary
#   4. Falls back to clone+build instructions if Go is missing

set -e

REPO="github.com/guillermoBallester/isthmus"
MODULE="${REPO}/cmd/isthmus"
MIN_GO_VERSION="1.25"

# --- Helpers ---

info()  { printf "\033[1;34m==>\033[0m %s\n" "$1"; }
ok()    { printf "\033[1;32m==>\033[0m %s\n" "$1"; }
warn()  { printf "\033[1;33m==>\033[0m %s\n" "$1"; }
error() { printf "\033[1;31m==>\033[0m %s\n" "$1" >&2; }

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux"  ;;
    Darwin*) echo "darwin" ;;
    *)       echo "unsupported" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64)  echo "amd64" ;;
    aarch64) echo "arm64" ;;
    arm64)   echo "arm64" ;;
    *)       echo "unsupported" ;;
  esac
}

# Compare version strings: returns 0 if $1 >= $2
version_gte() {
  [ "$(printf '%s\n' "$1" "$2" | sort -V | head -n1)" = "$2" ]
}

has_go() {
  command -v go >/dev/null 2>&1
}

go_version() {
  go version | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?' | head -1
}

# --- Main ---

main() {
  OS=$(detect_os)
  ARCH=$(detect_arch)

  info "Detected: ${OS}/${ARCH}"

  if [ "$OS" = "unsupported" ] || [ "$ARCH" = "unsupported" ]; then
    error "Unsupported platform: $(uname -s)/$(uname -m)"
    exit 1
  fi

  # --- Phase 2 (future): Download prebuilt binary from GitHub Releases ---
  # When releases are available, uncomment this block and it will be tried first.
  #
  # VERSION=$(curl -fsSL "https://api.github.com/repos/guillermoBallester/isthmus/releases/latest" \
  #   | grep '"tag_name"' | head -1 | sed 's/.*"v\(.*\)".*/\1/')
  #
  # if [ -n "$VERSION" ]; then
  #   BINARY_URL="https://github.com/guillermoBallester/isthmus/releases/download/v${VERSION}/isthmus-${OS}-${ARCH}"
  #   info "Downloading isthmus v${VERSION} for ${OS}/${ARCH}..."
  #   curl -fsSL -o /tmp/isthmus "$BINARY_URL"
  #   chmod +x /tmp/isthmus
  #   INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
  #   mv /tmp/isthmus "${INSTALL_DIR}/isthmus"
  #   ok "Installed isthmus v${VERSION} to ${INSTALL_DIR}/isthmus"
  #   exit 0
  # fi

  # --- Phase 1: Build from source via go install ---
  if has_go; then
    CURRENT_GO=$(go_version)
    if version_gte "$CURRENT_GO" "$MIN_GO_VERSION"; then
      info "Go ${CURRENT_GO} found — installing via go install..."
      go install "${MODULE}@latest"

      # Verify
      if command -v isthmus >/dev/null 2>&1; then
        ok "isthmus installed successfully: $(isthmus --version 2>&1 || echo 'installed')"
        ok "Binary location: $(command -v isthmus)"
      else
        warn "Binary installed but not in PATH."
        warn "Add \$GOPATH/bin to your PATH:"
        warn "  export PATH=\"\$PATH:\$(go env GOPATH)/bin\""
      fi
      exit 0
    else
      warn "Go ${CURRENT_GO} found but ${MIN_GO_VERSION}+ is required."
    fi
  fi

  # --- Fallback: manual instructions ---
  error "Go ${MIN_GO_VERSION}+ is required to install Isthmus."
  echo ""
  echo "Install Go: https://go.dev/doc/install"
  echo ""
  echo "Then run:"
  echo "  go install ${MODULE}@latest"
  echo ""
  echo "Or clone and build:"
  echo "  git clone https://${REPO}.git"
  echo "  cd isthmus && make build"
  echo "  # Binary: ./bin/isthmus"
  exit 1
}

main
