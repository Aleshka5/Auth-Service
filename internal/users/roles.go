package users

import (
	"fmt"
	"strings"
)

type Role string

const (
	RoleAdmin    Role = "ADMIN"
	RoleStranger Role = "STRANGER"
)

type ServiceRole string

const (
	ServiceRoleAdmin    ServiceRole = "ADMIN"
	ServiceRoleFamily   ServiceRole = "FAMILY"
	ServiceRoleStranger ServiceRole = "STRANGER"
)

func ParseRole(s string) (Role, error) {
	switch Role(strings.ToUpper(strings.TrimSpace(s))) {
	case RoleAdmin:
		return RoleAdmin, nil
	case RoleStranger:
		return RoleStranger, nil
	default:
		return "", fmt.Errorf("invalid roles value %q", s)
	}
}

func ParseServiceRole(s string) (ServiceRole, error) {
	switch ServiceRole(strings.ToUpper(strings.TrimSpace(s))) {
	case ServiceRoleAdmin:
		return ServiceRoleAdmin, nil
	case ServiceRoleFamily:
		return ServiceRoleFamily, nil
	case ServiceRoleStranger:
		return ServiceRoleStranger, nil
	default:
		return "", fmt.Errorf("invalid service role %q", s)
	}
}

// RemovesLastAdmin reports whether demoting or blocking u would leave zero active admins.
func RemovesLastAdmin(activeAdmins int, u User, demote, block bool) bool {
	if u.Roles != RoleAdmin || u.Blocked {
		return false
	}
	if !demote && !block {
		return false
	}
	return activeAdmins <= 1
}
