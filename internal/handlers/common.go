package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"food-platform/internal/middleware"
	"food-platform/internal/store"
)

const (
	defaultLimit = 20
	maxLimit     = 100

	// minUsernameLen mirrors the binding tag, re-checked after normalization
	// trims surrounding whitespace.
	minUsernameLen = 3
	// maxPasswordLen is bcrypt's working limit: it only hashes the first 72
	// bytes, so anything longer gives a false sense of strength.
	maxPasswordLen = 72
)

// normalizeUsername trims surrounding whitespace and lowercases the username so
// that accounts are case- and whitespace-insensitive (Alice == alice == "alice ").
func normalizeUsername(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// validatePassword enforces the account password rules shared by registration
// and admin provisioning: at most 72 bytes (bcrypt's limit) and at least two of
// {lowercase letter, uppercase letter, digit}. It returns nil when the password
// is acceptable.
func validatePassword(pw string) error {
	if len(pw) > maxPasswordLen {
		return fmt.Errorf("password must be at most %d bytes", maxPasswordLen)
	}
	if passwordClassCount(pw) < 2 {
		return errors.New("password must contain at least two of: lowercase letter, uppercase letter, digit")
	}
	return nil
}

// passwordClassCount reports how many of these character classes appear in pw:
// lowercase letters, uppercase letters, and digits.
func passwordClassCount(pw string) int {
	var hasLower, hasUpper, hasDigit bool
	for _, r := range pw {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	count := 0
	for _, present := range []bool{hasLower, hasUpper, hasDigit} {
		if present {
			count++
		}
	}
	return count
}

// respondStoreError maps a store-layer error to an appropriate HTTP response.
// The underlying error is logged on the 500 path but never returned to clients.
func respondStoreError(c *gin.Context, err error) {
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		return
	}
	slog.Error("store error",
		slog.String(middleware.RequestIDKey, middleware.RequestIDFromContext(c)),
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.Any("error", err),
	)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

// parseListOptions reads ?limit, ?offset, ?sort and ?order into a
// store.ListOptions. limit defaults to 20 and is capped at 100; offset
// defaults to 0. sort/order are passed through and validated in the store
// against a per-resource column allowlist.
func parseListOptions(c *gin.Context) store.ListOptions {
	opts := store.ListOptions{Limit: defaultLimit, Sort: c.Query("sort"), Order: c.Query("order")}
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		opts.Limit = v
	}
	if opts.Limit > maxLimit {
		opts.Limit = maxLimit
	}
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v > 0 {
		opts.Offset = v
	}
	return opts
}

// paginationMeta builds the pagination block returned alongside list data.
func paginationMeta(opts store.ListOptions, total int64) gin.H {
	return gin.H{"limit": opts.Limit, "offset": opts.Offset, "total": total}
}
