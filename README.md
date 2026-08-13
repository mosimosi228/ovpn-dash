# ovpn-dash

Web panel for an **already installed** OpenVPN server. One Go binary embeds the Vue dashboard.

## Run

```bash
make init
make run          # 127.0.0.1:7474
```

Open `http://127.0.0.1:7474/dashboard/`. First run: localhost or `?setup_token=` from `setup.token` in the data dir.

Does not install OpenVPN. Wizard asks for PKI dir, `server.conf`, systemd unit, public host.

## Install

`install.sh` lives in git (`scripts/install.sh`) and is published as a GitHub Release asset. Always take it from **latest**:

```bash
curl -fsSL https://github.com/mosimosi228/ovpn-dash/releases/latest/download/install.sh | sudo sh
sudo systemctl start ovpn-dash
```

The script downloads the matching `ovpn-dash_<tag>_linux_<arch>.tar.gz` from the same release (binary + `README.md` + `install.sh` from that git tag).

Put Caddy/nginx in front. See `deployments/`. Do not expose `:7474` to the internet.

## Release

Commit `README.md` and `scripts/install.sh`, tag that commit, then build and publish once:

```bash
git tag v0.1.0
git push origin v0.1.0
GH_PUBLISH=1 make release VERSION=v0.1.0
```

That uploads, from git:

- `install.sh` → https://github.com/mosimosi228/ovpn-dash/releases/latest/download/install.sh
- `README.md` → …/download/README.md
- `ovpn-dash_<tag>_linux_amd64.tar.gz` / `…_arm64.tar.gz` (binary + README.md + install.sh)
- `SHA256SUMS`

Linux amd64/arm64 only. SQLCipher requires CGO. Do not mark the release as prerelease, or `latest` will not point at it.
