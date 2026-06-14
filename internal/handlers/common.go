package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"food-platform/internal/middleware"
	"food-platform/internal/store"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

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
