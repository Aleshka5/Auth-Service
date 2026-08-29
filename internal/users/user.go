package users

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	GoogleSub    string
	GoogleEmail  string
	Name         string
	Roles        Role
	StorageRoles ServiceRole
	ResumeRoles  ServiceRole
	LastSeenAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type GoogleProfile struct {
	Sub   string
	Email string
	Name  string
}

type RolePatch struct {
	Roles        *Role
	StorageRoles *ServiceRole
	ResumeRoles  *ServiceRole
}
