package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"wakirim/model"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo *Repository
}

type claims struct {
	Sub      string `json:"sub"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Iss      string `json:"iss"`
	Aud      string `json:"aud"`
	Iat      int64  `json:"iat"`
	Exp      int64  `json:"exp"`
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Authenticate(usernameOrEmail, password string) (*model.Akun, string, int64, error) {
	akun, err := s.repo.GetAkunByUsernameOrEmail(strings.TrimSpace(usernameOrEmail))
	if err != nil {
		return nil, "", 0, errors.New("username/email atau password salah")
	}

	if akun.Password == nil || strings.TrimSpace(*akun.Password) == "" {
		return nil, "", 0, errors.New("akun belum memiliki password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*akun.Password), []byte(password)); err != nil {
		return nil, "", 0, errors.New("username/email atau password salah")
	}

	token, expiresIn, err := s.generateJWT(akun)
	if err != nil {
		return nil, "", 0, err
	}

	return akun, token, expiresIn, nil
}

func (s *Service) ValidateToken(token string) (*model.Akun, error) {
	parsed, err := s.parseAndValidateJWT(token)
	if err != nil {
		return nil, err
	}

	akun, err := s.repo.GetAkunByUUID(parsed.Sub)
	if err != nil {
		return nil, errors.New("akun tidak ditemukan")
	}

	return akun, nil
}

func (s *Service) DeleteAccountByUUID(uuid string) error {
	if strings.TrimSpace(uuid) == "" {
		return errors.New("uuid akun kosong")
	}

	return s.repo.DeleteAkunByUUID(uuid)
}

func (s *Service) BuildUserResponse(akun *model.Akun) OAuthUserResponse {
	tanggalDaftar := ""
	if akun.TanggalDaftar.Year() > 1 {
		tanggalDaftar = akun.TanggalDaftar.Format("2006-01-02")
	}
	tanggalBerakhir := ""
	if akun.TanggalBerakhir.Year() > 1 {
		tanggalBerakhir = akun.TanggalBerakhir.Format("2006-01-02")
	}

	return OAuthUserResponse{
		UUID:            akun.UUID,
		Username:        akun.Username,
		Email:           akun.Email,
		Status:          akun.Status,
		TanggalDaftar:   tanggalDaftar,
		TanggalBerakhir: tanggalBerakhir,
	}
}

func (s *Service) BuildUserResponseWithPaket(userData *AkunWithPaket) OAuthUserResponse {
	tanggalDaftar := ""
	if userData.TanggalDaftar.Year() > 1 {
		tanggalDaftar = userData.TanggalDaftar.Format("2006-01-02")
	}
	tanggalBerakhir := ""
	if userData.TanggalBerakhir.Year() > 1 {
		tanggalBerakhir = userData.TanggalBerakhir.Format("2006-01-02")
	}

	return OAuthUserResponse{
		UUID:            userData.UUID,
		Username:        userData.Username,
		Email:           userData.Email,
		Status:          userData.Status,
		TanggalDaftar:   tanggalDaftar,
		TanggalBerakhir: tanggalBerakhir,
		Paket:           userData.Paket,
	}
}

func (s *Service) generateJWT(akun *model.Akun) (string, int64, error) {
	issuedAt := time.Now().Unix()
	expiresIn := getTokenExpirySeconds()
	exp := issuedAt + expiresIn

	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}

	payload := claims{
		Sub:      akun.UUID,
		Username: akun.Username,
		Email:    akun.Email,
		Iss:      "wakirim",
		Aud:      "wakirim-client",
		Iat:      issuedAt,
		Exp:      exp,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", 0, err
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", 0, err
	}

	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)
	unsignedToken := headerEncoded + "." + payloadEncoded

	signature := signHS256(unsignedToken, []byte(getJWTSecret()))
	token := unsignedToken + "." + signature

	return token, expiresIn, nil
}

func (s *Service) parseAndValidateJWT(token string) (*claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("token tidak valid")
	}

	unsignedToken := parts[0] + "." + parts[1]
	expectedSignature := signHS256(unsignedToken, []byte(getJWTSecret()))
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSignature)) {
		return nil, errors.New("signature token tidak valid")
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("payload token tidak valid")
	}

	var parsed claims
	if err := json.Unmarshal(payloadRaw, &parsed); err != nil {
		return nil, errors.New("payload token tidak valid")
	}

	now := time.Now().Unix()
	if parsed.Exp <= now {
		return nil, errors.New("token sudah expired")
	}

	if parsed.Iss != "wakirim" || parsed.Aud != "wakirim-client" {
		return nil, errors.New("issuer/audience token tidak valid")
	}

	if strings.TrimSpace(parsed.Sub) == "" {
		return nil, errors.New("subject token kosong")
	}

	return &parsed, nil
}

func signHS256(value string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func getJWTSecret() string {
	secret := strings.TrimSpace(os.Getenv("CLIENT_JWT_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("ADMIN_PASSWORD")) + "_wakirim_client_jwt_secret"
	}
	return secret
}

func getTokenExpirySeconds() int64 {
	raw := strings.TrimSpace(os.Getenv("CLIENT_JWT_EXPIRES_SECONDS"))
	if raw == "" {
		return 60 * 60 * 24 * 30
	}

	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || val < 300 {
		return 60 * 60 * 24 * 30
	}

	return val
}

func getTokenExpiryDuration() time.Duration {
	return time.Duration(getTokenExpirySeconds()) * time.Second
}

func BuildBearerTokenResponse(token string, expiresIn int64, user OAuthUserResponse) OAuthTokenResponse {
	return OAuthTokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		User:        user,
	}
}

func ExtractBearerToken(authHeader string) (string, error) {
	if strings.TrimSpace(authHeader) == "" {
		return "", errors.New("authorization header kosong")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return "", errors.New("format authorization header salah")
	}

	if !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("authorization scheme harus Bearer")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("bearer token kosong")
	}

	return token, nil
}

func IsSecureRequest(forwardedProto string, isTLS bool) bool {
	if isTLS {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(forwardedProto), "https")
}

func CookieMaxAgeSeconds() int {
	return int(getTokenExpiryDuration().Seconds())
}

func BuildOAuthGrantError() error {
	return fmt.Errorf("grant_type tidak didukung, gunakan password")
}

func (s *Service) ChangePassword(uuid string, oldPassword string, newPassword string) error {
	akun, err := s.repo.GetAkunByUUID(uuid)
	if err != nil {
		return errors.New("akun tidak ditemukan")
	}

	if akun.Password == nil || strings.TrimSpace(*akun.Password) == "" {
		return errors.New("akun belum memiliki password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*akun.Password), []byte(oldPassword)); err != nil {
		return errors.New("password lama salah")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("gagal membuat password baru")
	}

	if err := s.repo.UpdateAkunPassword(akun.UUID, string(hashedPassword)); err != nil {
		return errors.New("gagal mengupdate password")
	}

	if err := s.sendPasswordResetEmail(akun.Email, newPassword); err != nil {
		return errors.New("password berhasil diganti, tetapi email gagal dikirim: " + err.Error())
	}

	return nil
}
