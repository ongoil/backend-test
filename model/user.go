package model

import "time"

// User is a user account. It is a plain domain struct with no database or
// transport tags; repository/mongodb maps it to a MongoDB document.
type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}
