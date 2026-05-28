package config

import (
	"os"
	"strings"
)

type SMTPConfig struct {
	Host      string
	Port      string
	Username  string
	Password  string
	FromEmail string
	FromName  string
	LogoURL   string
	LoginURL  string
	Secure    bool
	StartTLS  bool
}

func GetSMTPConfig() SMTPConfig {
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if port == "" {
		port = "587"
	}

	fromName := strings.TrimSpace(os.Getenv("SMTP_FROM_NAME"))
	if fromName == "" {
		fromName = "Wakirim"
	}

	secure := envBool("SMTP_SECURE", port == "465")
	startTLS := envBool("SMTP_STARTTLS", port == "587" && !secure)

	return SMTPConfig{
		Host:      strings.TrimSpace(os.Getenv("SMTP_HOST")),
		Port:      port,
		Username:  strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		Password:  strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
		FromEmail: strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL")),
		FromName:  fromName,
		LogoURL:   getEmailLogoURL(),
		LoginURL:  getLoginURL(),
		Secure:    secure,
		StartTLS:  startTLS,
	}
}

func (c SMTPConfig) Enabled() bool {
	return c.Host != "" && c.Port != "" && c.Username != "" && c.Password != "" && c.FromEmail != ""
}

func (c SMTPConfig) MissingFields() []string {
	var fields []string
	if c.Host == "" {
		fields = append(fields, "SMTP_HOST")
	}
	if c.Port == "" {
		fields = append(fields, "SMTP_PORT")
	}
	if c.Username == "" {
		fields = append(fields, "SMTP_USERNAME")
	}
	if c.Password == "" {
		fields = append(fields, "SMTP_PASSWORD")
	}
	if c.FromEmail == "" {
		fields = append(fields, "SMTP_FROM_EMAIL")
	}
	return fields
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}

	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func getEmailLogoURL() string {
	logoURL := strings.TrimSpace(os.Getenv("EMAIL_LOGO_URL"))
	if logoURL != "" {
		return logoURL
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_BASE_URL")), "/")
	if baseURL == "" {
		return ""
	}

	return baseURL + "/assets/icon/logo-wakirim-site.webp"
}

func getLoginURL() string {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_BASE_URL")), "/")
	if baseURL == "" {
		return "/login"
	}
	return baseURL + "/login"
}
