# ovpn-dash

[English](README.md) · [Русский](README.ru.md)

Веб-панель для **уже установленного** сервера OpenVPN. Сама OpenVPN не ставит.

Linux amd64 / arm64. Слушает только `127.0.0.1:7474` — снаружи ставьте Caddy или nginx.

## 1. OpenVPN

Нужен рабочий сервер с CA (`ca.crt` + `ca.key`) и `server.conf`. Если его ещё нет:

```bash
sudo apt update
sudo apt install -y openvpn easy-rsa
```

Дальше обычная настройка PKI и `server.conf` (easy-rsa, `openvpn-install.sh` и т.п.). Панель этим не занимается.

Для карты клиентов в `server.conf` добавьте:

```
status /var/log/openvpn/status.log
```

Для отзыва сертификатов — `crl-verify` на CRL. Для TLS-ключа в `.ovpn` — `tls-crypt` или `tls-auth`.

## 2. Панель

```bash
curl -fsSL https://github.com/mosimosi228/ovpn-dash/releases/latest/download/install.sh | sudo sh
sudo systemctl start ovpn-dash
```

Откройте `http://127.0.0.1:7474/dashboard/` **с этой машины**. Первый запуск: логин администратора и пути. Потом те же поля — во вкладке **Настройки**.

| Поле | Пример |
| --- | --- |
| Каталог PKI | `/etc/openvpn/easy-rsa/pki` |
| server.conf | `/etc/openvpn/server/server.conf` |
| Systemd unit | `openvpn-server@server` |
| Файл лога | `/var/log/openvpn/server.log` |
| Публичный адрес для .ovpn | `vpn.example.com` |

Другие частые пути: `server.conf` → `/etc/openvpn/server.conf`, unit → `openvpn@server`, ключ CA → `pki/private/ca.key` или `pki/ca.key`.

Если мастер открываете не с localhost, передайте токен из `/etc/ovpn-dash/setup.token`:

`http://127.0.0.1:7474/dashboard/?setup_token=…`

## 3. Прокси

Порт `7474` в интернет не публикуйте. Пример Caddy:

```
vpn.example.com {
    reverse_proxy 127.0.0.1:7474
}
```

Примеры: `deployments/caddy/Caddyfile`, `deployments/nginx/ovpn-dash.conf`.

## Команды

```bash
sudo systemctl status ovpn-dash
sudo journalctl -u ovpn-dash -f
sudo ovpn-dash uninstall          # снять unit
```

Данные панели: `/etc/ovpn-dash`.
