package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	maxLoginAttempts       = 5
	loginCooldownDuration  = 5 * time.Minute
	blockDurationAfterFail = 24 * time.Hour
)

type loginAttemptState struct {
	FailureCount  int
	CooldownUntil time.Time
	BlockedUntil  time.Time
}

type LoginProtectionStatus struct {
	Active           bool   `json:"active"`
	Mode             string `json:"mode"`
	Reason           string `json:"reason"`
	RemainingSeconds int64  `json:"remaining_seconds"`
	Until            string `json:"until"`
}

var (
	loginAttemptMu    sync.Mutex
	loginAttemptStore = map[string]*loginAttemptState{}
)

func ClientLoginProtectionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		identity := resolveLoginIdentity(c)
		status := GetClientLoginProtectionStatus(c.ClientIP(), identity)
		if status.Active {
			c.JSON(httpStatusFromMode(status.Mode), gin.H{
				"success": false,
				"message": status.Reason,
				"data":    status,
			})
			c.Abort()
			return
		}

		c.Next()

		failedAny, _ := c.Get("auth_login_failed")
		succeededAny, _ := c.Get("auth_login_succeeded")
		failed, _ := failedAny.(bool)
		succeeded, _ := succeededAny.(bool)

		ipKey := loginAttemptKey(c.ClientIP(), "")
		identityKey := loginAttemptKey(c.ClientIP(), identity)

		if succeeded {
			resetLoginAttemptState(ipKey)
			if identityKey != ipKey {
				resetLoginAttemptState(identityKey)
			}
			return
		}

		if !failed {
			return
		}

		loginAttemptMu.Lock()
		defer loginAttemptMu.Unlock()

		now := time.Now()
		registerFailure(ipKey, now)
		if identityKey != ipKey {
			registerFailure(identityKey, now)
		}
	}
}

func GetClientLoginProtectionStatus(ip, identity string) LoginProtectionStatus {
	loginAttemptMu.Lock()
	defer loginAttemptMu.Unlock()
	return getClientLoginProtectionStatusLocked(ip, identity, time.Now())
}

func getClientLoginProtectionStatusLocked(ip, identity string, now time.Time) LoginProtectionStatus {
	ipKey := loginAttemptKey(ip, "")
	identityKey := loginAttemptKey(ip, identity)

	var longestBlock time.Time
	var longestCooldown time.Time

	for _, key := range []string{ipKey, identityKey} {
		state := getOrCreateLoginAttemptState(key)
		if state.BlockedUntil.After(now) && state.BlockedUntil.After(longestBlock) {
			longestBlock = state.BlockedUntil
		}
		if state.CooldownUntil.After(now) && state.CooldownUntil.After(longestCooldown) {
			longestCooldown = state.CooldownUntil
		}
	}

	if !longestBlock.IsZero() {
		return LoginProtectionStatus{
			Active:           true,
			Mode:             "blocked",
			Reason:           "Login diblokir sementara karena percobaan gagal berulang setelah masa tunggu.",
			RemainingSeconds: int64(time.Until(longestBlock).Seconds()),
			Until:            longestBlock.Format(time.RFC3339),
		}
	}

	if !longestCooldown.IsZero() {
		return LoginProtectionStatus{
			Active:           true,
			Mode:             "cooldown",
			Reason:           "Percobaan login gagal lebih dari 5 kali. Harap tunggu sebelum mencoba lagi.",
			RemainingSeconds: int64(time.Until(longestCooldown).Seconds()),
			Until:            longestCooldown.Format(time.RFC3339),
		}
	}

	return LoginProtectionStatus{
		Active:           false,
		Mode:             "none",
		Reason:           "",
		RemainingSeconds: 0,
		Until:            "",
	}
}

func registerFailure(key string, now time.Time) {
	state := getOrCreateLoginAttemptState(key)

	if !state.CooldownUntil.IsZero() && now.After(state.CooldownUntil) {
		state.BlockedUntil = now.Add(blockDurationAfterFail)
		state.CooldownUntil = time.Time{}
		state.FailureCount = 0
		return
	}

	state.FailureCount++
	if state.FailureCount > maxLoginAttempts {
		state.CooldownUntil = now.Add(loginCooldownDuration)
		state.FailureCount = 0
	}
}

func resolveLoginIdentity(c *gin.Context) string {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(c.GetHeader("Content-Type"), ";")[0]))

	if contentType == "application/x-www-form-urlencoded" || contentType == "multipart/form-data" {
		username := strings.TrimSpace(c.PostForm("username"))
		if username != "" {
			return username
		}
	}

	if contentType == "application/json" {
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return ""
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))

		var payload map[string]interface{}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return ""
		}

		username, _ := payload["username"].(string)
		return strings.TrimSpace(username)
	}

	return strings.TrimSpace(c.PostForm("username"))
}

func loginAttemptKey(ip, identity string) string {
	identity = strings.ToLower(strings.TrimSpace(identity))
	if identity == "" {
		return "ip:" + ip
	}
	return "ip:" + ip + "|id:" + identity
}

func getOrCreateLoginAttemptState(key string) *loginAttemptState {
	state, ok := loginAttemptStore[key]
	if !ok {
		state = &loginAttemptState{}
		loginAttemptStore[key] = state
	}
	return state
}

func resetLoginAttemptState(key string) {
	loginAttemptMu.Lock()
	defer loginAttemptMu.Unlock()
	delete(loginAttemptStore, key)
}

func httpStatusFromMode(mode string) int {
	switch mode {
	case "blocked":
		return http.StatusForbidden
	case "cooldown":
		return http.StatusTooManyRequests
	default:
		return http.StatusUnauthorized
	}
}

func FormatRemainingDuration(seconds int64) string {
	if seconds <= 0 {
		return "0 detik"
	}
	d := time.Duration(seconds) * time.Second
	minutes := int64(d / time.Minute)
	secs := int64((d % time.Minute) / time.Second)
	if minutes > 0 {
		return fmt.Sprintf("%d menit %d detik", minutes, secs)
	}
	return fmt.Sprintf("%d detik", secs)
}
