package auth

func HasPermission(permissions []string, required string) bool {
	if len(permissions) == 0 {
		return false
	}
	permSet := make(map[string]struct{}, len(permissions))
	for _, p := range permissions {
		permSet[p] = struct{}{}
	}
	_, ok := permSet[required]
	return ok
}

// HasPermissionFromSet проверяет наличие разрешения в множестве (map[string]struct{}).
func HasPermissionFromSet(permissions map[string]struct{}, required string) bool {
	if len(permissions) == 0 {
		return false
	}
	_, ok := permissions[required]
	return ok
}
