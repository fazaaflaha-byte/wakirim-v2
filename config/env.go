package config

import (
	"bufio"
	"log"
	"os"
	"strings"
)

func LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}

		if current, exists := os.LookupEnv(key); !exists || strings.TrimSpace(current) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// Log prints a message with a prefix
func Log(msg string) {
	log.Println(msg)
}

// GetAdminPassword returns the admin password from environment
func GetAdminPassword() string {
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		return "admin123"
	}
	return password
}

// GetAdminUsername returns the admin username from environment
func GetAdminUsername() string {
	username := os.Getenv("ADMIN_USERNAME")
	if username == "" {
		return "admin"
	}
	return username
}
