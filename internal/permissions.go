package app

import "sort"

const (
	ViewTest = "viewTest"
)

func init() {
	extendPermissions()
	for role, permissions := range permissionsByRole {
		sort.Strings(permissions)
		permissionsByRole[role] = permissions
	}
}

var permissionsByRole = map[Role][]string{
	GuestRole: {
		ViewTest,
	},
	UserRole:      {},
	DeveloperRole: {},
}

func extendPermissions() {
	permissions := []string{}
	for k, v := range permissionsByRole {
		permissions = append(permissions, v...)
		permissionsByRole[k] = permissions
	}
}

func DoesRoleHavePermission(role Role, permission string) bool {
	permissions, ok := permissionsByRole[role]
	if !ok {
		return false
	}

	i := sort.SearchStrings(permissions, permission)
	return i < len(permissions) && permissions[i] == permission
}
