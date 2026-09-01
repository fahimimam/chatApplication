package domain

import (
	"strings"
	"time"
)

// UserID uniquely identifies a user.
type UserID string

// String returns the textual form of a UserID.
func (id UserID) String() string { return string(id) }

// User is a person who can log in and participate in conversations.
type User struct {
	ID           UserID
	Username     string
	DisplayName  string
	PasswordHash string // bcrypt; never returned by services
	CreatedAt    time.Time
}

// Validate enforces invariants on a freshly built or updated User.
func (u *User) Validate() error {
	if strings.TrimSpace(u.Username) == "" {
		return &ValidationError{Field: "username", Message: "must not be empty"}
	}
	if len(u.Username) > 64 {
		return &ValidationError{Field: "username", Message: "must be <= 64 chars"}
	}
	if strings.ContainsAny(u.Username, " \t\n") {
		return &ValidationError{Field: "username", Message: "must not contain whitespace"}
	}
	return nil
}

// SafeView returns a copy of the user without sensitive fields.
func (u *User) SafeView() User {
	return User{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		CreatedAt:   u.CreatedAt,
	}
}
