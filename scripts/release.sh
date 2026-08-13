#!/usr/bin/env sh
# Build platform archives into releases/<version>/ for GitHub Releases.
# Usage:
#   ./scripts/release.sh 0.1.0
#   make release VERSION=v0.1.0
#   GH_PUBLISH=1 ./scripts/release.sh v0.1.0
set -eu
cd "$(dirname "$0")/.."

VERSION="${1:-${VERSION:-}}"
[ -n "$VERSION" ] || { echo "usage: $0 <version>" >&2; exit 1; }
case "$VERSION" in
  v*) ;;
  *) VERSION="v${VERSION}" ;;
esac

OUT="${RELEASE_DIR:-releases}/${VERSION}"
TARGETS="${TARGETS:-linux_amd64 linux_arm64}"
REPO="${GITHUB_REPO:-mosimosi228/ovpn-dash}"

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
  cp -f scripts/install.sh "${dir}/install.sh"
  chmod 0755 "${dir}/install.sh"

  asset="ovpn-dash_${VERSION}_${t}.tar.gz"
  (
    cd "$dir"
    tar -czf "${OLDPWD}/${OUT}/${asset}" ovpn-dash README.md install.sh
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
mkdir -p releases
cp -f scripts/install.sh releases/install.sh
chmod 0755 releases/install.sh
cp -f README.md releases/README.md
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
echo "publish (push the git tag first so the release matches this commit):"
echo "  git tag ${VERSION} && git push origin ${VERSION}"
echo "  gh release create ${VERSION} --repo ${REPO} --title ${VERSION} --notes-file README.md \\"
echo "    ${OUT}/ovpn-dash_${VERSION}_*.tar.gz ${OUT}/SHA256SUMS ${OUT}/install.sh ${OUT}/README.md"
if [ "${GH_PUBLISH:-0}" = "1" ]; then
  gh release create "${VERSION}" --repo "${REPO}" --title "${VERSION}" --notes-file README.md \
    "${OUT}"/ovpn-dash_"${VERSION}"_*.tar.gz "${OUT}/SHA256SUMS" "${OUT}/install.sh" "${OUT}/README.md"
fi
[ "$failed" -eq 0 ] || echo "warning: ${failed} target(s) failed" >&2
