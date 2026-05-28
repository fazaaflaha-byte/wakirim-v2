package model

import "time"

// Auth-related model (extends Akun)
type LoginSession struct {
	Token     string    `json:"token" db:"token"`
	AkunUUID  string    `json:"akun_uuid" db:"akun_uuid"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}