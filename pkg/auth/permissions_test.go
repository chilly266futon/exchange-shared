package auth

import "testing"

func TestHasPermission(t *testing.T) {
	perms := []string{"read", "write"}
	if !HasPermission(perms, "read") {
		t.Error("Должен возвращать true для существующего permission")
	}
	if HasPermission(perms, "delete") {
		t.Error("Должен возвращать false для отсутствующего permission")
	}
}
