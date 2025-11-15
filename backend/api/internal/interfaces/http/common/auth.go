package common

import "context"

type contextKey string

const authUserContextKey contextKey = "authUser"

// AuthenticatedUser represents the JWT-derived principal.
type AuthenticatedUser struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Picture  string `json:"picture,omitempty"`
}

// ContextWithUser stores the authenticated user into context.
func ContextWithUser(ctx context.Context, user AuthenticatedUser) context.Context {
	return context.WithValue(ctx, authUserContextKey, user)
}

// UserFromContext extracts the authenticated user from context.
func UserFromContext(ctx context.Context) (AuthenticatedUser, bool) {
	user, ok := ctx.Value(authUserContextKey).(AuthenticatedUser)
	return user, ok
}
