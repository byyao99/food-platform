package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"food-platform/internal/models"
	"food-platform/internal/store"
)

// OrderHandler handles order-related HTTP requests.
type OrderHandler struct {
	store *store.Store
}

// NewOrderHandler creates an OrderHandler.
func NewOrderHandler(s *store.Store) *OrderHandler {
	return &OrderHandler{store: s}
}

// orderItemRequest is a single line item in an order request.
type orderItemRequest struct {
	MenuItemID string `json:"menu_item_id" binding:"required"`
	Quantity   int    `json:"quantity" binding:"required,gt=0"`
}

// createOrderRequest is the payload for creating an order.
type createOrderRequest struct {
	CustomerName string             `json:"customer_name" binding:"required,max=120"`
	Items        []orderItemRequest `json:"items" binding:"required,min=1,dive"`
	Note         string             `json:"note" binding:"max=500"`
}

// updateOrderRequest is the payload for updating an order (status and note only).
type updateOrderRequest struct {
	Status models.OrderStatus `json:"status" binding:"required"`
	Note   string             `json:"note" binding:"max=500"`
}

// List handles GET /api/v1/orders
func (h *OrderHandler) List(c *gin.Context) {
	opts := parseListOptions(c)
	orders, total, err := h.store.ListOrders(opts)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":       orders,
		"pagination": paginationMeta(opts, total),
	})
}

// Get handles GET /api/v1/orders/:id
func (h *OrderHandler) Get(c *gin.Context) {
	order, err := h.store.GetOrder(c.Param("id"))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": order})
}

// Create handles POST /api/v1/orders
func (h *OrderHandler) Create(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	items, total, err := h.buildOrderItems(req.Items)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := h.store.CreateOrder(models.Order{
		ID:           uuid.NewString(),
		CustomerName: req.CustomerName,
		Items:        items,
		TotalAmount:  total,
		Status:       models.OrderStatusPending,
		Note:         req.Note,
	})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": order})
}

// Update handles PUT /api/v1/orders/:id and updates the status and note.
func (h *OrderHandler) Update(c *gin.Context) {
	var req updateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !req.Status.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order status"})
		return
	}

	existing, err := h.store.GetOrder(c.Param("id"))
	if err != nil {
		respondStoreError(c, err)
		return
	}

	if !existing.Status.CanTransitionTo(req.Status) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("cannot change status from %s to %s", existing.Status, req.Status),
		})
		return
	}

	existing.Status = req.Status
	existing.Note = req.Note

	updated, err := h.store.UpdateOrder(c.Param("id"), existing)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": updated})
}

// Delete handles DELETE /api/v1/orders/:id
func (h *OrderHandler) Delete(c *gin.Context) {
	if err := h.store.DeleteOrder(c.Param("id")); err != nil {
		respondStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// buildOrderItems validates items against the menu, builds snapshots and computes the total.
func (h *OrderHandler) buildOrderItems(reqs []orderItemRequest) ([]models.OrderItem, int64, error) {
	items := make([]models.OrderItem, 0, len(reqs))
	var total int64

	for _, r := range reqs {
		menuItem, err := h.store.GetMenuItem(r.MenuItemID)
		if err != nil {
			return nil, 0, fmt.Errorf("menu item not found: %s", r.MenuItemID)
		}
		if !menuItem.Available {
			return nil, 0, fmt.Errorf("menu item is not available: %s", menuItem.Name)
		}

		subtotal := menuItem.Price * int64(r.Quantity)
		total += subtotal
		items = append(items, models.OrderItem{
			MenuItemID: menuItem.ID,
			Name:       menuItem.Name,
			UnitPrice:  menuItem.Price,
			Quantity:   r.Quantity,
			Subtotal:   subtotal,
		})
	}

	return items, total, nil
}
