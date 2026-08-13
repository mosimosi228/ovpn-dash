package main

import (
	"fmt"
	"os"

	"github.com/mosimosi228/ovpn-dash/internal/app"
	"github.com/mosimosi228/ovpn-dash/internal/install"
	"github.com/spf13/cobra"
)

// Set via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "ovpn-dash",
		Short: "OpenVPN panel — manage an existing OpenVPN server",
	}
	root.AddCommand(
		newServeCmd(),
		newInstallCmd(),
		newUninstallCmd(),
		&cobra.Command{
			Use:   "version",
			Short: "Print version",
			Run:   func(cmd *cobra.Command, args []string) { fmt.Println(version) },
		},
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newServeCmd() *cobra.Command {
	var dir, listen string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start ovpn-dash (setup wizard on /dashboard)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.Serve(app.ServeOptions{Dir: dir, Listen: listen})
		},
	}
	cmd.Flags().StringVar(&dir, "dir", envOr("OVPNDASH_DIR", "./ovpn-dash-data"), "data directory")
	cmd.Flags().StringVar(&listen, "listen", envOr("OVPNDASH_LISTEN", "127.0.0.1:7474"), "listen address")
	return cmd
}

func newInstallCmd() *cobra.Command {
	var noStart bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install ovpn-dash as a systemd service (root)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.Execute(install.Options{NoStart: noStart})
		},
	}
	cmd.Flags().BoolVar(&noStart, "no-start", false, "enable unit but do not start the service")
	return cmd
}

func newUninstallCmd() *cobra.Command {
	var purge bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove ovpn-dash systemd service (root)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.Uninstall(install.UninstallOptions{Purge: purge})
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "also remove /etc/ovpn-dash")
	return cmd
}
