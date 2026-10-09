package entity

type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash []byte
	CreatedAt    string
	IsActive     bool
}
