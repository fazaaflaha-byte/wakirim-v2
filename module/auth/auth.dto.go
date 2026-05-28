package auth

type TokenRequest struct {
	GrantType string `json:"grant_type" form:"grant_type"`
	Username  string `json:"username" form:"username"`
	Password  string `json:"password" form:"password"`
}

type OAuthUserResponse struct {
	UUID            string `json:"uuid"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	Status          string `json:"status"`
	TanggalDaftar   string `json:"tanggal_daftar"`
	TanggalBerakhir string `json:"tanggal_berakhir"`
}

type OAuthTokenResponse struct {
	AccessToken string            `json:"access_token"`
	TokenType   string            `json:"token_type"`
	ExpiresIn   int64             `json:"expires_in"`
	User        OAuthUserResponse `json:"user"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
