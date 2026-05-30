package pengaduan

type CreatePengaduanRequest struct {
	Email     string `json:"email" binding:"required"`
	Judul     string `json:"judul" binding:"required"`
	Deskripsi string `json:"deskripsi" binding:"required"`
}

type TanggapiPengaduanRequest struct {
	Jawaban string `json:"jawaban" binding:"required"`
}

type PengaduanResponse struct {
	ID                string  `json:"id"`
	Username          string  `json:"username"`
	Email             string  `json:"email"`
	Judul             string  `json:"judul"`
	Deskripsi         string  `json:"deskripsi"`
	Jawaban           *string `json:"jawaban"`
	Status            string  `json:"status"`
	TanggalDibuat     string  `json:"tanggal_dibuat"`
	TanggalDitanggapi *string `json:"tanggal_ditanggapi,omitempty"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
