package interceptors

import (
	"context"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type JWTValidator interface {
	Validate(token string) (*jwt.MapClaims, error)
}

func AuthInterceptor(
	logger *zap.Logger,
	jwtValidator JWTValidator,
) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			logger.Warn("missing metadata in request", zap.String("method", info.FullMethod))
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		tokens := md.Get("authorization")
		if len(tokens) == 0 {
			logger.Warn("missing authorization header", zap.String("method", info.FullMethod))
			return nil, status.Errorf(codes.Unauthenticated, "missing authorization header")
		}

		token := tokens[0]
		if !strings.HasPrefix(token, "Bearer") {
			return nil, status.Errorf(codes.Unauthenticated, "invalid authorization format")
		}
		token = token[7:]

		claims, err := jwtValidator.Validate(token)
		if err != nil {
			logger.Warn("invalid token", zap.String("method", info.FullMethod), zap.Error(err))
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}

		ctx = context.WithValue(ctx, "user_id", (*claims)["sub"].(string))
		ctx = context.WithValue(ctx, "roles", (*claims)["roles"].([]string))
		ctx = context.WithValue(ctx, "permissions", (*claims)["permissions"].([]string))

		logger.Info("authenticated request",
			zap.String("method", info.FullMethod),
			zap.String("user_id", (*claims)["sub"].(string)),
		)

		return handler(ctx, req)
	}
}
