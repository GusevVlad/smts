package server

import (
	"context"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	userInfoKey contextKey = "userInfo"
	userGroupsKey contextKey = "userGroups"
)

// contextWithUserInfo adds user information to the context
func contextWithUserInfo(ctx context.Context, userInfo map[string]string, groups []string) context.Context {
	ctx = context.WithValue(ctx, userInfoKey, userInfo)
	ctx = context.WithValue(ctx, userGroupsKey, groups)
	return ctx
}

// userInfoFromContext extracts user information from the context
func userInfoFromContext(ctx context.Context) (map[string]string, []string) {
	userInfo, _ := ctx.Value(userInfoKey).(map[string]string)
	groups, _ := ctx.Value(userGroupsKey).([]string)
	return userInfo, groups
}