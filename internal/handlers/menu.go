package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

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

// maxMenuPrice caps a menu item's price (cents) to a sane upper bound — here
// $1,000,000 — so an obvious typo cannot persist an absurd value.
const maxMenuPrice = 100_000_000

// menuItemRequest is the payload for creating/updating a menu item.
type menuItemRequest struct {
	Name        string `json:"name" binding:"required,max=120"`
	Description string `json:"description" binding:"max=500"`
	Price       int64  `json:"price" binding:"required,gt=0,lte=100000000"` // cents, max $1,000,000
	Category    string `json:"category" binding:"required,max=60"`
	Available   *bool  `json:"available"`
}

// normalize trims surrounding whitespace on the free-text fields and rejects
// values that are blank once trimmed. Keeping category trimmed makes the
// case-insensitive category filter consistent.
func (r *menuItemRequest) normalize() error {
	r.Name = strings.TrimSpace(r.Name)
	r.Category = strings.TrimSpace(r.Category)
	if r.Name == "" {
		return errors.New("name must not be blank")
	}
	if r.Category == "" {
		return errors.New("category must not be blank")
	}
	return nil
}

// List handles GET /api/v1/menu. Optional ?category (case-insensitive exact)
// and ?available (true/false) narrow the results.
func (h *MenuHandler) List(c *gin.Context) {
	opts := parseListOptions(c)
	filter := store.MenuFilter{Category: strings.TrimSpace(c.Query("category"))}
	if v := c.Query("available"); v != "" {
		available, err := strconv.ParseBool(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "available must be true or false"})
			return
		}
		filter.Available = &available
	}
	items, total, err := h.store.ListMenu(opts, filter)
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
	if err := req.normalize(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// On create, available defaults to true when the client omits it.
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
	if err := req.normalize(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Update is a full replace, so available must be explicit: defaulting an
	// omitted value to true would silently re-enable a disabled item.
	if req.Available == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "available is required when updating a menu item"})
		return
	}

	item, err := h.store.UpdateMenuItem(c.Param("id"), models.MenuItem{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		Available:   *req.Available,
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
