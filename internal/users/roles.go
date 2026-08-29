package users

import (
	"fmt"
	"strings"
)

type Role string

const (
	RoleAdmin    Role = "ADMIN"
	RoleStranger Role = "STRANGER"
	RoleBlocked  Role = "BLOCKED"
)

type ServiceRole string

const (
	ServiceRoleAdmin    ServiceRole = "ADMIN"
	ServiceRoleFamily   ServiceRole = "FAMILY"
	ServiceRoleStranger ServiceRole = "STRANGER"
	ServiceRoleBlocked  ServiceRole = "BLOCKED"
)

func ParseRole(s string) (Role, error) {
	switch Role(strings.ToUpper(strings.TrimSpace(s))) {
	case RoleAdmin:
		return RoleAdmin, nil
	case RoleStranger:
		return RoleStranger, nil
	case RoleBlocked:
		return RoleBlocked, nil
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
	case ServiceRoleBlocked:
		return ServiceRoleBlocked, nil
	default:
		return "", fmt.Errorf("invalid service role %q", s)
	}
}

// HasBlockedRole reports whether a whitelist projection includes BLOCKED
// for the caller's service role field.
func HasBlockedRole(fields map[string]string) bool {
	blocked := string(RoleBlocked)
	return fields["roles"] == blocked ||
		fields["storage_roles"] == blocked ||
		fields["resume_roles"] == blocked
}

// RemovesLastAdmin reports whether changing u.Roles to next would leave zero admins.
func RemovesLastAdmin(activeAdmins int, u User, next *Role) bool {
	if u.Roles != RoleAdmin || next == nil || *next == RoleAdmin {
		return false
	}
	return activeAdmins <= 1
}
