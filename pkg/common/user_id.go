package common

import (
	"context"

	"google.golang.org/grpc/metadata"
)

const UserIDKey = "user_id"

func GetUserID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(UserIDKey)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
