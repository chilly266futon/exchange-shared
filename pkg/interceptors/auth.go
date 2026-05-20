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

type authCtxKey string

const (
	authCtxKeyUserID      authCtxKey = "user_id"
	authCtxKeyRoles       authCtxKey = "roles"
	authCtxKeyPermissions authCtxKey = "permissions"
)

// AuthCtx агрегирует все значения авторизации из контекста.
type AuthCtx struct {
	UserID      string
	Roles       []int32
	Permissions map[string]struct{}
}

// ToAuthCtx извлекает все значения авторизации из контекста в структуру AuthCtx.
func ToAuthCtx(ctx context.Context) AuthCtx {
	userID, _ := ctx.Value(authCtxKeyUserID).(string)
	roles, _ := ctx.Value(authCtxKeyRoles).([]int32)
	perms, _ := ctx.Value(authCtxKeyPermissions).(map[string]struct{})
	return AuthCtx{
		UserID:      userID,
		Roles:       roles,
		Permissions: perms,
	}
}

// NewContextWithAuthCtx кладёт все значения из AuthCtx в context.Context.
func NewContextWithAuthCtx(ctx context.Context, a AuthCtx) context.Context {
	ctx = context.WithValue(ctx, authCtxKeyUserID, a.UserID)
	ctx = context.WithValue(ctx, authCtxKeyRoles, a.Roles)
	ctx = context.WithValue(ctx, authCtxKeyPermissions, a.Permissions)
	return ctx
}

type JWTValidator interface {
	Validate(token string) (*jwt.MapClaims, error)
}

func AuthInterceptor(
	logger *zap.Logger,
	jwtValidator JWTValidator,
	skipMethods ...string,
) grpc.UnaryServerInterceptor {
	skip := make(map[string]bool, len(skipMethods))
	for _, m := range skipMethods {
		skip[m] = true
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if skip[info.FullMethod] {
			return handler(ctx, req)
		}

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
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(token, bearerPrefix) {
			return nil, status.Errorf(codes.Unauthenticated, "invalid authorization format")
		}
		token = token[len(bearerPrefix):]

		claims, err := jwtValidator.Validate(token)
		if err != nil {
			logger.Warn("invalid token", zap.String("method", info.FullMethod), zap.Error(err))
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}

		userID, _ := (*claims)["sub"].(string)
		roles := claimIntSlice(claims, "roles")
		permissions := claimStringSet(claims, "permissions")

		authCtx := AuthCtx{
			UserID:      userID,
			Roles:       roles,
			Permissions: permissions,
		}
		ctx = NewContextWithAuthCtx(ctx, authCtx)

		logger.Info("authenticated request",
			zap.String("method", info.FullMethod),
			zap.String("user_id", userID),
		)

		return handler(ctx, req)
	}
}

// claimIntSlice извлекает []int32 из JWT-клейма (roles).
func claimIntSlice(claims *jwt.MapClaims, key string) []int32 {
	var out []int32
	switch v := (*claims)[key].(type) {
	case []interface{}:
		out = make([]int32, 0, len(v))
		for _, item := range v {
			switch n := item.(type) {
			case float64:
				out = append(out, int32(n))
			case int32:
				out = append(out, n)
			}
		}
	case []int32:
		out = v
	}
	if len(out) == 0 {
		zap.L().Info("claimIntSlice: roles is empty or default", zap.String("claim", key))
	}
	return out
}

// claimStringSet извлекает map[string]struct{} из JWT-клейма (permissions).
func claimStringSet(claims *jwt.MapClaims, key string) map[string]struct{} {
	set := make(map[string]struct{})
	switch v := (*claims)[key].(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				set[s] = struct{}{}
			}
		}
	case []string:
		for _, s := range v {
			set[s] = struct{}{}
		}
	}
	if len(set) == 0 {
		zap.L().Info("claimStringSet: permissions is empty or default", zap.String("claim", key))
	}
	return set
}
