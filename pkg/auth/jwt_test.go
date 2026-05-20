package auth

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewJWTValidator(t *testing.T) {
	logger := zap.NewNop()
	validator := NewJWTValidator("testsecret", logger)
	if validator == nil {
		t.Error("JWTValidator должен быть создан")
	}
}

func TestJWTValidator_Validate_InvalidToken(t *testing.T) {
	logger := zap.NewNop()
	validator := NewJWTValidator("testsecret", logger)
	_, err := validator.Validate("invalid.token")
	if err == nil {
		t.Error("Ожидалась ошибка для некорректного токена")
	}
}
