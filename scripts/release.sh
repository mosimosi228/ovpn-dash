#!/usr/bin/env sh
# Build platform archives into releases/<version>/ for GitHub Releases.
# Usage:
#   ./scripts/release.sh 0.1.0
#   make release VERSION=v0.1.0
#   make publish VERSION=v0.1.0
#   GH_PUBLISH=1 ./scripts/release.sh v0.1.0
#
# git push uses origin as-is (SSH host aliases like mmvs-github.com).
# gh talks to github.com — GH_HOST=github.com even when origin is an SSH alias.
set -eu
cd "$(dirname "$0")/.."

PATH="${HOME}/.local/bin:${PATH:-/usr/bin}"
export PATH

PUBLISH_ONLY=0
VERSION="${VERSION:-}"
for arg in "$@"; do
  case "$arg" in
    --publish-only) PUBLISH_ONLY=1 ;;
    -h|--help)
      echo "usage: $0 <version> [--publish-only]" >&2
      exit 0
      ;;
    *) VERSION="$arg" ;;
  esac
done
[ -n "$VERSION" ] || { echo "usage: $0 <version> [--publish-only]" >&2; exit 1; }
case "$VERSION" in
  v*) ;;
  *) VERSION="v${VERSION}" ;;
esac

OUT="${RELEASE_DIR:-releases}/${VERSION}"
TARGETS="${TARGETS:-linux_amd64 linux_arm64}"

repo_from_origin() {
  url=$(git remote get-url origin 2>/dev/null || true)
  url=${url%.git}
  case "$url" in
    git@*:*) printf '%s\n' "${url#*:}" ;;
    ssh://git@*/*)
      printf '%s\n' "${url#ssh://git@*/}"
      ;;
    https://github.com/*) printf '%s\n' "${url#https://github.com/}" ;;
    http://github.com/*) printf '%s\n' "${url#http://github.com/}" ;;
    *) printf '%s\n' "mosimosi228/ovpn-dash" ;;
  esac
}

REPO="${GITHUB_REPO:-$(repo_from_origin)}"
# SSH Host mmvs-github.com has HostName github.com — API is always github.com.
GH_HOST="${GH_HOST:-github.com}"
export GH_HOST

need_gh() {
  if ! command -v gh >/dev/null 2>&1; then
    echo "error: gh not found (needed to upload GitHub Release assets)" >&2
    echo "  install: https://cli.github.com/" >&2
    echo "  or:      mkdir -p ~/.local/bin && extract gh into it" >&2
    echo "SSH alias (mmvs-github.com) only pushes git. Releases still use github.com API." >&2
    exit 1
  fi
  if ! gh auth status -h "$GH_HOST" >/dev/null 2>&1; then
    echo "error: gh is not logged in to ${GH_HOST}" >&2
    echo "  git push already works via SSH Host mmvs-github.com → github.com" >&2
    echo "  log in with the same GitHub account as ~/.ssh/id_ed25519_github:" >&2
    echo "    export PATH=\"\$HOME/.local/bin:\$PATH\"" >&2
    echo "    gh auth login -h github.com -p ssh -w" >&2
    exit 1
  fi
}

publish_release() {
  need_gh
  [ -f "${OUT}/install.sh" ] && [ -f "${OUT}/README.md" ] && [ -f "${OUT}/SHA256SUMS" ] || {
    echo "error: missing ${OUT}/ (run: make release VERSION=${VERSION})" >&2
    exit 1
  }
  set -- "${OUT}/SHA256SUMS" "${OUT}/install.sh" "${OUT}/README.md"
  [ -f "${OUT}/README.ru.md" ] && set -- "$@" "${OUT}/README.ru.md"
  for a in "${OUT}"/ovpn-dash_"${VERSION}"_*.tar.gz; do
    [ -f "$a" ] || continue
    set -- "$@" "$a"
  done
  echo "→ GitHub Release ${VERSION}  repo=${REPO}  host=${GH_HOST}"
  if gh release view "${VERSION}" --repo "${REPO}" >/dev/null 2>&1; then
    gh release upload "${VERSION}" --repo "${REPO}" --clobber "$@"
  else
    gh release create "${VERSION}" --repo "${REPO}" --title "${VERSION}" --notes-file README.md "$@"
  fi
  echo "✓ https://github.com/${REPO}/releases/tag/${VERSION}"
  echo "  install.sh: https://github.com/${REPO}/releases/latest/download/install.sh"
}

if [ "$PUBLISH_ONLY" = "1" ]; then
  publish_release
  exit 0
fi

if [ "${GH_PUBLISH:-0}" = "1" ]; then
  need_gh
fi

host_os=$(uname -s | tr '[:upper:]' '[:lower:]')
host_arch=$(uname -m)
case "$host_arch" in
  x86_64|amd64) host_arch=amd64 ;;
  aarch64|arm64) host_arch=arm64 ;;
esac

echo "→ building SPA"
(cd web && if [ ! -d node_modules ]; then
  if [ -f package-lock.json ]; then npm ci; else npm install; fi
fi && npm run build)

[ -f README.md ] || { echo "error: README.md missing (must be in git)" >&2; exit 1; }
[ -f README.ru.md ] || { echo "error: README.ru.md missing (must be in git)" >&2; exit 1; }
[ -f scripts/install.sh ] || { echo "error: scripts/install.sh missing (must be in git)" >&2; exit 1; }

mkdir -p "$OUT"
rm -f "$OUT"/*

build_one() {
  t=$1
  os=${t%_*}
  arch=${t#*_}
  echo "→ ${t}"

  cc=""
  case "${os}_${arch}" in
    linux_amd64) cc="${CC_LINUX_AMD64:-${CC:-gcc}}" ;;
    linux_arm64) cc="${CC_LINUX_ARM64:-aarch64-linux-gnu-gcc}" ;;
  esac

  if [ "$os" != "$host_os" ] || [ "$arch" != "$host_arch" ]; then
    if [ -z "$cc" ] || ! command -v "$cc" >/dev/null 2>&1; then
      echo "  skip ${t}: no C cross-compiler (set CC_* or install toolchain)"
      return 0
    fi
  else
    cc="${cc:-${CC:-gcc}}"
    if ! command -v "$cc" >/dev/null 2>&1; then
      echo "  skip ${t}: missing compiler ${cc}"
      return 0
    fi
  fi

  dir=$(mktemp -d)
  bin="${dir}/ovpn-dash"
  if ! CGO_ENABLED=1 CC="$cc" GOOS="$os" GOARCH="$arch" go build -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o "$bin" ./cmd/ovpn-dash; then
    rm -rf "$dir"
    echo "  fail ${t}: go build" >&2
    return 1
  fi

  cp -f README.md "${dir}/README.md"
  cp -f README.ru.md "${dir}/README.ru.md"
  cp -f scripts/install.sh "${dir}/install.sh"
  chmod 0755 "${dir}/install.sh"

  asset="ovpn-dash_${VERSION}_${t}.tar.gz"
  (
    cd "$dir"
    tar -czf "${OLDPWD}/${OUT}/${asset}" ovpn-dash README.md README.ru.md install.sh
  )
  rm -rf "$dir"
  echo "  ${OUT}/${asset}"
}

failed=0
built=0
for t in $TARGETS; do
  if build_one "$t"; then
    if [ -f "${OUT}/ovpn-dash_${VERSION}_${t}.tar.gz" ]; then
      built=$((built + 1))
    fi
  else
    failed=$((failed + 1))
  fi
done

[ "$built" -gt 0 ] || { echo "error: no archives built" >&2; exit 1; }

# Standalone GitHub Release assets — same files as in git at this commit.
cp -f scripts/install.sh "${OUT}/install.sh"
chmod 0755 "${OUT}/install.sh"
cp -f README.md "${OUT}/README.md"
cp -f README.ru.md "${OUT}/README.ru.md"
mkdir -p releases
cp -f scripts/install.sh releases/install.sh
chmod 0755 releases/install.sh
cp -f README.md releases/README.md
cp -f README.ru.md releases/README.ru.md
printf '%s\n' "$VERSION" > releases/LATEST

(
  cd "$OUT"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ovpn-dash_*.tar.gz > SHA256SUMS
  else
    shasum -a 256 ovpn-dash_*.tar.gz > SHA256SUMS
  fi
)

echo
echo "✓ ${OUT}/  (${built} archive(s))"
ls -la "$OUT"
echo
echo "publish (tag is git push via origin / mmvs-github.com; assets need gh on github.com):"
echo "  git push origin ${VERSION}"
echo "  make publish VERSION=${VERSION}"
if [ "${GH_PUBLISH:-0}" = "1" ]; then
  publish_release
fi
[ "$failed" -eq 0 ] || echo "warning: ${failed} target(s) failed" >&2
