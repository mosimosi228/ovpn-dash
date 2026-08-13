# ovpn-dash

[English](README.md) · [Русский](README.ru.md)

Web panel for an **already installed** OpenVPN server. It does not install OpenVPN.

Linux amd64 / arm64. Listens on `127.0.0.1:7474` only — put Caddy or nginx in front.

## 1. OpenVPN

You need a working server with a CA (`ca.crt` + `ca.key`) and `server.conf`. If you do not have one yet:

```bash
sudo apt update
sudo apt install -y openvpn easy-rsa
```

Then set up PKI and `server.conf` as usual (easy-rsa, `openvpn-install.sh`, etc.). The panel does not do that.

For the client map, add to `server.conf`:

```
status /var/log/openvpn/status.log
```

For revoke to take effect, add `crl-verify` pointing at the CRL. For a TLS key in `.ovpn`, use `tls-crypt` or `tls-auth`.

## 2. Panel

```bash
curl -fsSL https://github.com/mosimosi228/ovpn-dash/releases/latest/download/install.sh | sudo sh
sudo systemctl start ovpn-dash
```

Open `http://127.0.0.1:7474/dashboard/` **from this machine**. First run: admin login and host paths. Later the same fields are on the **Settings** tab.

| Field | Example |
| --- | --- |
| PKI directory | `/etc/openvpn/easy-rsa/pki` |
| server.conf | `/etc/openvpn/server/server.conf` |
| Systemd unit | `openvpn-server@server` |
| Log file | `/var/log/openvpn/server.log` |
| Public address for .ovpn | `vpn.example.com` |

Other common paths: `server.conf` → `/etc/openvpn/server.conf`, unit → `openvpn@server`, CA key → `pki/private/ca.key` or `pki/ca.key`.

If the wizard is not opened from localhost, pass the token from `/etc/ovpn-dash/setup.token`:

`http://127.0.0.1:7474/dashboard/?setup_token=…`

## 3. Proxy

Do not expose port `7474` to the internet. Caddy example:

```
vpn.example.com {
    reverse_proxy 127.0.0.1:7474
}
```

Samples: `deployments/caddy/Caddyfile`, `deployments/nginx/ovpn-dash.conf`.

## Commands

```bash
sudo systemctl status ovpn-dash
sudo journalctl -u ovpn-dash -f
sudo ovpn-dash uninstall          # remove the unit
```

Panel data: `/etc/ovpn-dash`.
