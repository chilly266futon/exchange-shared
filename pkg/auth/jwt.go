package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type JWTValidator struct {
	secret []byte
	logger *zap.Logger
}

func NewJWTValidator(secret string, logger *zap.Logger) *JWTValidator {
	return &JWTValidator{
		secret: []byte(secret),
		logger: logger,
	}
}

func (v *JWTValidator) Validate(tokenString string) (*jwt.MapClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		return v.secret, nil
	})
	if err != nil {
		if v.logger != nil {
			v.logger.Warn("JWT validation failed", zap.Error(err))
		}
		return nil, err
	}

	claims, ok := token.Claims.(*jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}
