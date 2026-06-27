package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"food-platform/internal/auth"
	"food-platform/internal/middleware"
	"food-platform/internal/models"
	"food-platform/internal/store"
)

// AuthHandler handles registration and login.
type AuthHandler struct {
	store *store.Store
	auth  *auth.Manager
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(s *store.Store, am *auth.Manager) *AuthHandler {
	return &AuthHandler{store: s, auth: am}
}

// credentialsRequest is the shared payload for register and login.
type credentialsRequest struct {
	Username string `json:"username" binding:"required,min=3,max=60"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

// Register handles POST /api/v1/auth/register. New accounts are always created
// with the customer role; staff/admin accounts are provisioned out of band
// (e.g. the startup seed) so clients cannot escalate their own privileges.
func (h *AuthHandler) Register(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		Role:         models.RoleCustomer,
	})
	if err != nil {
		if errors.Is(err, store.ErrUsernameTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
			return
		}
		respondStoreError(c, err)
		return
	}

	h.issueToken(c, user, http.StatusCreated)
}

// Login handles POST /api/v1/auth/login and returns a bearer token on success.
func (h *AuthHandler) Login(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use the same response for an unknown user and a wrong password so the
	// endpoint does not reveal which usernames exist. Run a throwaway bcrypt
	// comparison on the no-such-user path so the two cases take comparable time
	// and timing does not leak which usernames exist either.
	user, err := h.store.GetUserByUsername(normalizeUsername(req.Username))
	if err != nil {
		auth.CompareDummy(req.Password)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	h.issueToken(c, user, http.StatusOK)
}

// changePasswordRequest is the payload for a self-service password change.
type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

// ChangePassword handles PUT /api/v1/auth/password for the authenticated user.
// It verifies the current password before setting the new one. Existing tokens
// remain valid until they expire (tokens are stateless).
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validatePassword(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.store.GetUser(c.GetString(middleware.ContextUserIDKey))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	// Use 400 (not 401) for a wrong current password: the request is already
	// authenticated, and the frontend clears the session on any 401.
	if !auth.CheckPassword(user.PasswordHash, req.OldPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current password is incorrect"})
		return
	}
	if req.OldPassword == req.NewPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new password must differ from the current one"})
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if err := h.store.UpdateUserPassword(user.ID, hash); err != nil {
		respondStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// issueToken mints a token for the user and writes it with the user summary.
func (h *AuthHandler) issueToken(c *gin.Context, user models.User, status int) {
	token, err := h.auth.Issue(user.ID, user.Username, string(user.Role))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	c.JSON(status, gin.H{"data": gin.H{"token": token, "user": user}})
}
