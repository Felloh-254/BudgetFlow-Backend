package models

import "time"

type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
}

// PublicUser is what we ever send back over the API — never the hash.
// Having a dedicated type (rather than trusting the `json:"-"` tag alone)
// makes it impossible to accidentally leak the hash if the struct changes.
type PublicUser struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) Public() PublicUser {
	return PublicUser{ID: u.ID, Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt}
}
