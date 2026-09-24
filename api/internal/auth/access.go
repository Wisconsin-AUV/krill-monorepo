package auth

import (
	"slices"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

type RoleDef struct {
	Role krillv1.Role
	// Name is how the role is stored in the database.
	Name  string
	Label string
}

// Roles are ordered lowest first. Each has every permission of the roles
// before it.
var Roles = []RoleDef{
	{krillv1.Role_ROLE_LABELER, "labeler", "Labeler"},
	{krillv1.Role_ROLE_DEVELOPER, "developer", "Developer"},
	{krillv1.Role_ROLE_ADMIN, "admin", "Admin"},
}

func ParseRole(name string) krillv1.Role {
	for _, r := range Roles {
		if r.Name == name {
			return r.Role
		}
	}
	return krillv1.Role_ROLE_UNSPECIFIED
}

func RoleName(role krillv1.Role) (string, bool) {
	for _, r := range Roles {
		if r.Role == role {
			return r.Name, true
		}
	}
	return "", false
}

type PermissionDef struct {
	Permission  krillv1.Permission
	Description string
	// Role is the lowest role that has the permission. Roles are ordered, so
	// every role above it has it too.
	Role krillv1.Role
}

// Permissions says what each role can do. The web app reads this and Roles
// from GetSession, and a test fails if a Permission value is missing here.
var Permissions = []PermissionDef{
	{krillv1.Permission_PERMISSION_LABEL, "View videos, label clips, and mark frames done", krillv1.Role_ROLE_LABELER},
	{krillv1.Permission_PERMISSION_MANAGE_VIDEOS, "Upload, edit, and delete videos", krillv1.Role_ROLE_DEVELOPER},
	{krillv1.Permission_PERMISSION_MANAGE_LABEL_TYPES, "Manage label types", krillv1.Role_ROLE_DEVELOPER},
	{krillv1.Permission_PERMISSION_MANAGE_EXPORTS, "Create and download exports", krillv1.Role_ROLE_DEVELOPER},
	{krillv1.Permission_PERMISSION_MANAGE_USERS, "Manage users and their roles", krillv1.Role_ROLE_ADMIN},
}

func Can(role krillv1.Role, p krillv1.Permission) bool {
	i := slices.IndexFunc(Permissions, func(d PermissionDef) bool { return d.Permission == p })
	return i >= 0 && role >= Permissions[i].Role
}

func describe(p krillv1.Permission) string {
	for _, d := range Permissions {
		if d.Permission == p {
			return d.Description
		}
	}
	return p.String()
}

func Granted(role krillv1.Role) []krillv1.Permission {
	var out []krillv1.Permission
	for _, d := range Permissions {
		if role >= d.Role {
			out = append(out, d.Permission)
		}
	}
	return out
}

func PermissionInfos() []*krillv1.PermissionInfo {
	out := make([]*krillv1.PermissionInfo, len(Permissions))
	for i, d := range Permissions {
		out[i] = &krillv1.PermissionInfo{Permission: d.Permission, Description: d.Description, Role: d.Role}
	}
	return out
}

func RoleInfos() []*krillv1.RoleInfo {
	out := make([]*krillv1.RoleInfo, len(Roles))
	for i, r := range Roles {
		out[i] = &krillv1.RoleInfo{Role: r.Role, Label: r.Label}
	}
	return out
}
