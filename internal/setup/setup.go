package setup

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
)

// Settings is the operator config stored in SQLCipher after the wizard.
type Settings struct {
	AdminUser  string
	PKIDir     string
	ServerConf string
	Unit       string
	LogFile    string
	PublicHost string
	Complete   bool
}

func ParseComplete(v string) bool {
	return v == "1" || v == "true" || v == "yes"
}
