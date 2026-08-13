#!/usr/bin/env sh
# ovpn-dash install / upgrade from GitHub Releases.
#
#   curl -fsSL https://github.com/mosimosi228/ovpn-dash/releases/latest/download/install.sh | sudo sh
#   ./scripts/install.sh --version v0.1.0
set -eu

REPO="${OVPNDASH_REPO:-mosimosi228/ovpn-dash}"
VERSION="${OVPNDASH_VERSION:-latest}"
INSTALL_DIR="${OVPNDASH_DIR:-/etc/ovpn-dash}"
BIN_DIR="${OVPNDASH_BIN_DIR:-/usr/local/bin}"

log() { printf '%s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

detect_target() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) die "unsupported arch: $arch" ;;
  esac
  [ "$os" = "linux" ] || die "unsupported os: $os (linux only)"
  printf '%s_%s' "$os" "$arch"
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --dir|-d) INSTALL_DIR="${2:-}"; shift 2 ;;
      --bin-dir) BIN_DIR="${2:-}"; shift 2 ;;
      --version|-v) VERSION="${2:-}"; shift 2 ;;
      --repo) REPO="${2:-}"; shift 2 ;;
      -h|--help)
        sed -n '2,8p' "$0"
        exit 0
        ;;
      *) die "unknown arg: $1" ;;
    esac
  done
}

auth_header() {
  tok="${GH_TOKEN:-${GITHUB_TOKEN:-${OVPNDASH_GITHUB_TOKEN:-}}}"
  if [ -n "$tok" ]; then
    printf '%s' "Authorization: Bearer ${tok}"
  fi
}

download() {
  url=$1
  out=$2
  hdr=$(auth_header)
  if command -v curl >/dev/null 2>&1; then
    if [ -n "$hdr" ]; then
      curl -fsSL -H "$hdr" "$url" -o "$out"
    else
      curl -fsSL "$url" -o "$out"
    fi
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url"
  else
    die "need curl or wget"
  fi
}

# Resolve "latest" via github.com Location — not api.github.com (unauthenticated API is often 403).
resolve_version() {
  if [ "$VERSION" != "latest" ]; then
    case "$VERSION" in
      v*) ;;
      *) VERSION="v${VERSION}" ;;
    esac
    return 0
  fi
  need_cmd curl
  hdr=$(auth_header)
  if [ -n "$hdr" ]; then
    loc=$(curl -fsSI -H "$hdr" "https://github.com/${REPO}/releases/latest")
  else
    loc=$(curl -fsSI "https://github.com/${REPO}/releases/latest")
  fi
  VERSION=$(printf '%s' "$loc" | tr -d '\r' | awk 'tolower($1)=="location:" {print $2; exit}')
  VERSION=$(printf '%s' "$VERSION" | sed -n 's#.*/releases/tag/##p')
  [ -n "$VERSION" ] || die "could not resolve latest release for ${REPO} (github.com redirect had no tag)"
  log "latest → ${VERSION}"
}

install_binary() {
  target=$(detect_target)
  resolve_version
  tmp=$(mktemp -d)
  # shellcheck disable=SC2064
  trap "rm -rf '$tmp'" EXIT INT TERM

  asset="ovpn-dash_${VERSION}_${target}.tar.gz"
  url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
  log "downloading ${url}"
  download "$url" "$tmp/od.tgz"

  if download "https://github.com/${REPO}/releases/download/${VERSION}/SHA256SUMS" "$tmp/SHA256SUMS" 2>/dev/null; then
    expect=$(awk -v a="$asset" '$2==a {print $1}' "$tmp/SHA256SUMS")
    if [ -n "$expect" ]; then
      if command -v sha256sum >/dev/null 2>&1; then
        got=$(sha256sum "$tmp/od.tgz" | awk '{print $1}')
      else
        got=$(shasum -a 256 "$tmp/od.tgz" | awk '{print $1}')
      fi
      [ "$got" = "$expect" ] || die "checksum mismatch for ${asset}"
      log "checksum ok"
    fi
  else
    log "warning: SHA256SUMS missing; skipping checksum"
  fi

  tar -xzf "$tmp/od.tgz" -C "$tmp"
  [ -f "$tmp/ovpn-dash" ] || die "archive missing ovpn-dash binary"
  mkdir -p "$BIN_DIR" "$INSTALL_DIR"
  install -m 0755 "$tmp/ovpn-dash" "${BIN_DIR}/ovpn-dash"
  log "installed ${BIN_DIR}/ovpn-dash"
}

parse_args "$@"
[ "$(id -u)" -eq 0 ] || die "run as root"
install_binary
"${BIN_DIR}/ovpn-dash" install --no-start || true
log "data dir: ${INSTALL_DIR}"
log "start:    systemctl start ovpn-dash"
log "ui:       http://127.0.0.1:7474/dashboard/  (put Caddy/nginx in front)"
