package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"food-platform/internal/auth"
	"food-platform/internal/middleware"
	"food-platform/internal/models"
	"food-platform/internal/store"
)

// UserHandler handles admin-only user management.
type UserHandler struct {
	store *store.Store
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(s *store.Store) *UserHandler {
	return &UserHandler{store: s}
}

// createUserRequest is the payload for provisioning an account with an explicit
// role. Unlike self-registration, this path may create staff and admins, so it
// is gated to admins at the route level.
type createUserRequest struct {
	Username string      `json:"username" binding:"required,min=3,max=60"`
	Password string      `json:"password" binding:"required,min=8,max=72"`
	Role     models.Role `json:"role" binding:"required"`
}

// Create handles POST /api/v1/users (admin only).
func (h *UserHandler) Create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !req.Role.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}
	req.Username = normalizeUsername(req.Username)
	if len(req.Username) < minUsernameLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username must be at least 3 characters"})
		return
	}
	if err := validatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondStoreError(c, err)
		return
	}

	user, err := h.store.CreateUser(models.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		PasswordHash: hash,
		Role:         req.Role,
	})
	if err != nil {
		if errors.Is(err, store.ErrUsernameTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
			return
		}
		respondStoreError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": user})
}

// List handles GET /api/v1/users (admin only). Optional ?role and ?q filter by
// exact role and case-insensitive username substring respectively.
func (h *UserHandler) List(c *gin.Context) {
	opts := parseListOptions(c)
	filter := store.UserFilter{Username: normalizeUsername(c.Query("q"))}
	if role := c.Query("role"); role != "" {
		r := models.Role(role)
		if !r.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
		filter.Role = r
	}
	users, total, err := h.store.ListUsers(opts, filter)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":       users,
		"pagination": paginationMeta(opts, total),
	})
}

// updateRoleRequest is the payload for changing a user's role.
type updateRoleRequest struct {
	Role models.Role `json:"role" binding:"required"`
}

// UpdateRole handles PUT /api/v1/users/:id/role (admin only). An admin may not
// change their own role, which keeps them from removing their own access.
func (h *UserHandler) UpdateRole(c *gin.Context) {
	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !req.Role.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
		return
	}
	if c.Param("id") == c.GetString(middleware.ContextUserIDKey) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you cannot change your own role"})
		return
	}

	user, oldRole, err := h.store.UpdateUserRole(c.Param("id"), req.Role)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if oldRole != user.Role {
		slog.Info("user role changed",
			slog.String(middleware.RequestIDKey, middleware.RequestIDFromContext(c)),
			slog.String("actor_id", c.GetString(middleware.ContextUserIDKey)),
			slog.String("target_id", user.ID),
			slog.String("old_role", string(oldRole)),
			slog.String("new_role", string(user.Role)),
		)
	}
	c.JSON(http.StatusOK, gin.H{"data": user})
}

// Delete handles DELETE /api/v1/users/:id (admin only). An admin may not delete
// their own account.
func (h *UserHandler) Delete(c *gin.Context) {
	if c.Param("id") == c.GetString(middleware.ContextUserIDKey) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you cannot delete your own account"})
		return
	}
	if err := h.store.DeleteUser(c.Param("id")); err != nil {
		respondStoreError(c, err)
		return
	}
	slog.Info("user deleted",
		slog.String(middleware.RequestIDKey, middleware.RequestIDFromContext(c)),
		slog.String("actor_id", c.GetString(middleware.ContextUserIDKey)),
		slog.String("target_id", c.Param("id")),
	)
	c.Status(http.StatusNoContent)
}

// resetPasswordRequest is the payload for an admin password reset.
type resetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// ResetPassword handles PUT /api/v1/users/:id/password (admin only). Unlike the
// self-service change, no current password is required, so an admin can recover
// a locked-out account.
func (h *UserHandler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if err := h.store.UpdateUserPassword(c.Param("id"), hash); err != nil {
		respondStoreError(c, err)
		return
	}
	slog.Info("user password reset",
		slog.String(middleware.RequestIDKey, middleware.RequestIDFromContext(c)),
		slog.String("actor_id", c.GetString(middleware.ContextUserIDKey)),
		slog.String("target_id", c.Param("id")),
	)
	c.Status(http.StatusNoContent)
}
