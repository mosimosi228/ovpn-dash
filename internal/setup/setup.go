package setup

import "strings"

const (
	KeyAdminUser     = "admin_user"
	KeyAdminPassHash = "admin_pass_hash"
	KeyPKIDir        = "pki_dir"
	KeyServerConf    = "server_conf"
	KeyUnit          = "systemd_unit"
	KeyLogFile       = "log_file"
	KeyPublicHost    = "public_host"
	KeySetupComplete = "setup_complete"
	KeyJWTSecret     = "jwt_secret"

	KeySMTPHost = "smtp_host"
	KeySMTPPort = "smtp_port"
	KeySMTPUser = "smtp_user"
	KeySMTPPass = "smtp_pass"
	KeySMTPFrom = "smtp_from"
	KeySMTPTLS  = "smtp_tls"

	KeyTelegramBotToken = "telegram_bot_token"
	KeyTelegramBotName  = "telegram_bot_username"
	KeyTelegramOffset   = "telegram_update_offset"
)

// Live OpenVPN layout on this host (easy-rsa + openvpn-server@server).
const (
	DefaultPKIDir     = "/etc/openvpn/easy-rsa/pki"
	DefaultClientsDir = "/etc/openvpn/server/client"
	DefaultServerConf = "/etc/openvpn/server/server.conf"
	DefaultUnit       = "openvpn-server@server"
	DefaultLogFile    = "/var/log/openvpn/server.log"
)

const (
	RoleRoot  = "root"
	RoleAdmin = "admin"
	RoleUser  = "user"
)

const (
	ThemeLight = "light"
	ThemeDark  = "dark"
	MapAuto    = "auto"
	MapLight   = "light"
	MapDark    = "dark"
)

// Settings is the operator config stored in SQLCipher after the wizard.
type Settings struct {
	PKIDir     string
	ServerConf string
	Unit       string
	LogFile    string
	PublicHost string
	Complete   bool

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string
	SMTPTLS  bool

	TelegramBotToken string
	TelegramBotName  string
}

func ParseComplete(v string) bool {
	return v == "1" || v == "true" || v == "yes"
}

func ParseBool(v string) bool {
	return v == "1" || v == "true" || v == "yes"
}

// LegacyRootEmail maps the v1 admin login onto an email.
// An existing email is kept. Otherwise `{login}@{public_host}` when
// public_host is a DNS name (not an IP).
func LegacyRootEmail(user, publicHost string) string {
	user = strings.ToLower(strings.TrimSpace(user))
	if user == "" {
		user = "admin"
	}
	if strings.Contains(user, "@") {
		return user
	}
	host := strings.ToLower(strings.TrimSpace(publicHost))
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	if i := strings.IndexAny(host, "/:"); i >= 0 {
		host = host[:i]
	}
	if host == "" || !strings.Contains(host, ".") {
		return user
	}
	for _, r := range host {
		if (r >= '0' && r <= '9') || r == '.' {
			continue
		}
		return user + "@" + host
	}
	return user
}
