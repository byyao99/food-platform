package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"food-platform/internal/models"
	"food-platform/internal/store"
)

// MenuHandler handles menu-related HTTP requests.
type MenuHandler struct {
	store *store.Store
}

// NewMenuHandler creates a MenuHandler.
func NewMenuHandler(s *store.Store) *MenuHandler {
	return &MenuHandler{store: s}
}

// menuItemRequest is the payload for creating/updating a menu item.
type menuItemRequest struct {
	Name        string `json:"name" binding:"required,max=120"`
	Description string `json:"description" binding:"max=500"`
	Price       int64  `json:"price" binding:"required,gt=0"` // cents
	Category    string `json:"category" binding:"required,max=60"`
	Available   *bool  `json:"available"`
}

// List handles GET /api/v1/menu
func (h *MenuHandler) List(c *gin.Context) {
	opts := parseListOptions(c)
	items, total, err := h.store.ListMenu(opts)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":       items,
		"pagination": paginationMeta(opts, total),
	})
}

// Get handles GET /api/v1/menu/:id
func (h *MenuHandler) Get(c *gin.Context) {
	item, err := h.store.GetMenuItem(c.Param("id"))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

// Create handles POST /api/v1/menu
func (h *MenuHandler) Create(c *gin.Context) {
	var req menuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	available := true
	if req.Available != nil {
		available = *req.Available
	}

	item, err := h.store.CreateMenuItem(models.MenuItem{
		ID:          uuid.NewString(),
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		Available:   available,
	})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

// Update handles PUT /api/v1/menu/:id
func (h *MenuHandler) Update(c *gin.Context) {
	var req menuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	available := true
	if req.Available != nil {
		available = *req.Available
	}

	item, err := h.store.UpdateMenuItem(c.Param("id"), models.MenuItem{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		Available:   available,
	})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

// Delete handles DELETE /api/v1/menu/:id
func (h *MenuHandler) Delete(c *gin.Context) {
	if err := h.store.DeleteMenuItem(c.Param("id")); err != nil {
		respondStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
