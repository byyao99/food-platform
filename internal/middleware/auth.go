package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"food-platform/internal/auth"
	"food-platform/internal/models"
)

// Gin context keys under which the authenticated identity is stored.
const (
	ContextUserIDKey   = "user_id"
	ContextUsernameKey = "username"
	ContextRoleKey     = "role"
)

// RequireAuth rejects requests without a valid bearer token. On success it
// stores the caller's identity in the Gin context for downstream handlers.
func RequireAuth(am *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := authenticate(c, am); ok {
			c.Next()
		}
	}
}

// RequireRole rejects requests whose token is missing/invalid (401) or whose
// role is not in allowed (403).
func RequireRole(am *auth.Manager, allowed ...models.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := authenticate(c, am)
		if !ok {
			return
		}
		for _, r := range allowed {
			if models.Role(claims.Role) == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
	}
}

// authenticate extracts and verifies the bearer token. On failure it writes a
// 401, aborts the chain, and returns ok=false; on success it records the
// identity in the context.
func authenticate(c *gin.Context, am *auth.Manager) (*auth.Claims, bool) {
	header := c.GetHeader("Authorization")
	token, found := strings.CutPrefix(header, "Bearer ")
	if !found || token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed authorization header"})
		return nil, false
	}
	claims, err := am.Verify(token)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return nil, false
	}
	c.Set(ContextUserIDKey, claims.Subject)
	c.Set(ContextUsernameKey, claims.Username)
	c.Set(ContextRoleKey, claims.Role)
	return claims, true
}
