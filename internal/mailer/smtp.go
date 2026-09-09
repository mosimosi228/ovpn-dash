package mailer

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
)

// Config is optional SMTP for password recovery.
type Config struct {
	Host string
	Port int
	User string
	Pass string
	From string
	TLS  bool
}

func (c Config) Configured() bool {
	return strings.TrimSpace(c.Host) != "" && strings.TrimSpace(c.From) != ""
}

func ParsePort(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	if n <= 0 {
		return 587
	}
	return n
}

// Send delivers a plain-text message.
func Send(c Config, to, subject, body string) error {
	if !c.Configured() {
		return fmt.Errorf("smtp is not configured")
	}
	if c.Port <= 0 {
		c.Port = 587
	}
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	from := c.From
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	var auth smtp.Auth
	if c.User != "" {
		auth = smtp.PlainAuth("", c.User, c.Pass, c.Host)
	}

	if c.Port == 465 {
		return sendTLS(addr, auth, from, to, []byte(msg), c.Host)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

func sendTLS(addr string, auth smtp.Auth, from, to string, msg []byte, host string) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
