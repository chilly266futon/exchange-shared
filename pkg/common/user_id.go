package common

import (
	"context"
)

const UserIDKey = "user_id"

func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}
