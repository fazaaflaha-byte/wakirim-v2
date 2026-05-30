package auth

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"

	"wakirim/config"

	"github.com/google/uuid"
)

func (s *Service) sendPasswordResetEmail(email, newPassword string) error {
	cfg := config.GetSMTPConfig()
	if !cfg.Enabled() {
		config.Log("[Email] SMTP belum lengkap (" + strings.Join(cfg.MissingFields(), ", ") + "), email reset password dilewati untuk " + email)
		return nil
	}

	subject := "Password Wakirim Anda Telah Diganti"
	plainBody := fmt.Sprintf(
		"Halo,\n\nPassword akun Wakirim Anda telah berhasil diganti.\n\nPassword baru: %s\n\nSilakan login menggunakan password baru Anda.\n\nTerima kasih.\nWakirim",
		newPassword,
	)
	htmlBody := buildPasswordResetEmailHTML(newPassword, cfg.LogoURL)
	return sendSMTPEmail(cfg, []string{email}, subject, plainBody, htmlBody)
}

func buildPasswordResetEmailHTML(newPassword, logoURL string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
    @media only screen and (max-width: 620px) {
      .email-shell { padding: 18px 10px !important; }
      .email-card { width: 100%% !important; border-radius: 12px !important; }
      .email-pad { padding-left: 18px !important; padding-right: 18px !important; }
    }
  </style>
</head>
<body style="margin:0;background:#f3f4f6;font-family:Arial,Helvetica,sans-serif;color:#111111;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" class="email-shell" style="background:#f3f4f6;padding:34px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" class="email-card" style="max-width:560px;background:#ffffff;border:1px solid #e5e7eb;border-radius:18px;overflow:hidden;box-shadow:0 18px 45px rgba(17,24,39,.08);">
          <tr>
            <td class="email-pad" style="padding:28px;text-align:center;background:#111111;">
              %s
              <h1 style="margin:18px 0 0;font-size:22px;line-height:1.3;color:#ffffff;">Password Wakirim Anda Telah Diganti</h1>
              <p style="margin:10px 0 0;font-size:14px;line-height:1.7;color:#d4d4d4;">Gunakan password baru berikut untuk login ke akun Anda.</p>
            </td>
          </tr>
          <tr>
            <td class="email-pad" style="padding:26px 28px;">
              <div style="border:1px solid #e5e7eb;border-radius:12px;background:#fafafa;padding:18px;">
                <p style="margin:0 0 8px;font-size:13px;font-weight:700;color:#737373;text-transform:uppercase;letter-spacing:.08em;">Password Baru</p>
                <p style="margin:0;font-size:18px;font-weight:700;color:#111111;">%s</p>
              </div>
              <p style="margin:22px 0 0;font-size:14px;line-height:1.7;color:#525252;">Silakan login dan pastikan Anda dapat masuk dengan lancar.</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, emailLogoHTML(logoURL), html.EscapeString(newPassword))
}

func emailLogoHTML(logoURL string) string {
	if strings.TrimSpace(logoURL) == "" {
		return `<div style="display:inline-block;border:1px solid rgba(255,255,255,.18);border-radius:12px;padding:12px 18px;color:#ffffff;font-size:20px;font-weight:800;letter-spacing:.02em;">Wakirim</div>`
	}

	return fmt.Sprintf(
		`<img src="%s" width="156" alt="Wakirim" style="display:inline-block;max-width:156px;width:156px;height:auto;margin:0 auto;border:0;outline:none;text-decoration:none;">`,
		html.EscapeString(logoURL),
	)
}

func sendSMTPEmail(cfg config.SMTPConfig, recipients []string, subject, plainBody, htmlBody string) error {
	message, err := buildEmailMessage(cfg, recipients, subject, plainBody, htmlBody)
	if err != nil {
		return err
	}

	address := net.JoinHostPort(cfg.Host, cfg.Port)
	var client *smtp.Client

	if cfg.Secure {
		conn, err := tls.Dial("tcp", address, &tls.Config{ServerName: cfg.Host})
		if err != nil {
			return err
		}

		client, err = smtp.NewClient(conn, cfg.Host)
		if err != nil {
			conn.Close()
			return err
		}
	} else {
		client, err = smtp.Dial(address)
		if err != nil {
			return err
		}

		if cfg.StartTLS {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
				client.Close()
				return err
			}
		}
	}
	defer client.Close()

	if cfg.Username != "" || cfg.Password != "" {
		if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return err
		}
	}

	if err := client.Mail(cfg.FromEmail); err != nil {
		return err
	}

	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}

	if _, err := writer.Write(message); err != nil {
		writer.Close()
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	return client.Quit()
}

func buildEmailMessage(cfg config.SMTPConfig, recipients []string, subject, plainBody, htmlBody string) ([]byte, error) {
	alternativeBoundary := "wakirim_alt_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	from := mail.Address{Name: cfg.FromName, Address: cfg.FromEmail}

	var msg bytes.Buffer
	msg.WriteString("From: " + from.String() + "\r\n")
	msg.WriteString("To: " + strings.Join(recipients, ", ") + "\r\n")
	msg.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: multipart/alternative; boundary=\"" + alternativeBoundary + "\"\r\n")
	msg.WriteString("\r\n")

	msg.WriteString("--" + alternativeBoundary + "\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(quotedPrintable(plainBody))
	msg.WriteString("\r\n")

	msg.WriteString("--" + alternativeBoundary + "\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(quotedPrintable(htmlBody))
	msg.WriteString("\r\n")
	msg.WriteString("--" + alternativeBoundary + "--\r\n")

	return msg.Bytes(), nil
}

func quotedPrintable(value string) string {
	var buf bytes.Buffer
	writer := quotedprintable.NewWriter(&buf)
	_, _ = writer.Write([]byte(value))
	_ = writer.Close()
	return buf.String()
}
