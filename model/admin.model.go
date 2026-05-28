package model

import "time"

// Table Akun
type Akun struct {
	UUID            string    `json:"uuid" db:"uuid"`
	Email           string    `json:"email" db:"email"`
	Username        string    `json:"username" db:"username"`
	Password        *string   `json:"password,omitempty" db:"password"`
	TanggalDaftar   time.Time `json:"tanggal_daftar" db:"tanggal_daftar"`
	TanggalBerakhir time.Time `json:"tanggal_berakhir" db:"tanggal_berakhir"`
	Status          string    `json:"status" db:"status"` // "berjalan" | "akan_habis" | "habis"
}

// Table Payment
type Payment struct {
	ID              string    `json:"id" db:"id"`
	Username        string    `json:"username" db:"username"`
	Email           string    `json:"email" db:"email"`
	Paket           string    `json:"paket" db:"paket"`
	TotalHarga      int64     `json:"total_harga" db:"total_harga"`
	BuktiPembayaran string    `json:"bukti_pembayaran" db:"bukti_pembayaran"`
	TanggalBayar    time.Time `json:"tanggal_bayar" db:"tanggal_bayar"`
	Catatan         string    `json:"catatan" db:"catatan"`
	Status          string    `json:"status" db:"status"` // "pending" | "verified"
}

// Table Paket
type Paket struct {
	ID          string `json:"id" db:"id"`
	Nama        string `json:"nama" db:"nama"`
	Harga       int64  `json:"harga" db:"harga"`
	DurasiBulan int    `json:"durasi_bulan" db:"durasi_bulan"`
	Aktif       bool   `json:"aktif" db:"aktif"`
}
