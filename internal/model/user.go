package model

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	Role         string `json:"role"`
	CreatedAt    int64  `json:"created_at"`
}

type UserConfig struct {
	Users []User `json:"users"`
}