package datacontact

type ManualContactItem struct {
	Phone string `json:"phone" binding:"required"`
	Name  string `json:"name"`
}

type CreateManualRequest struct {
	NamaFile string              `json:"nama_file" binding:"required"`
	Contacts []ManualContactItem `json:"contacts" binding:"required"`
}

type DataContactResponse struct {
	ID            string `json:"id"`
	NamaFile      string `json:"nama_file"`
	DataTXT       string `json:"data_txt,omitempty"`
	TotalContact  int    `json:"total_contact"`
	Sumber        string `json:"sumber"`
	TanggalDibuat string `json:"tanggal_dibuat"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
