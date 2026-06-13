package models

import "time"

// MenuItem represents a single dish on the menu.
//
// Monetary amounts are stored as integer cents (e.g. 18000 == $180.00) to
// avoid floating-point rounding errors. Clients send and receive cents.
type MenuItem struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       int64     `json:"price"`     // cents
	Category    string    `json:"category"`  // e.g. Main, Drink, Dessert
	Available   bool      `json:"available"` // whether currently served
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// OrderStatus represents the lifecycle status of an order.
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPreparing OrderStatus = "preparing"
	OrderStatusReady     OrderStatus = "ready"
	OrderStatusCompleted OrderStatus = "completed"
	OrderStatusCancelled OrderStatus = "cancelled"
)

// Valid reports whether the status is a recognized value.
func (s OrderStatus) Valid() bool {
	_, ok := orderStatusTransitions[s]
	return ok
}

// orderStatusTransitions maps each status to the statuses it may move to.
// completed and cancelled are terminal.
var orderStatusTransitions = map[OrderStatus][]OrderStatus{
	OrderStatusPending:   {OrderStatusPreparing, OrderStatusCancelled},
	OrderStatusPreparing: {OrderStatusReady, OrderStatusCancelled},
	OrderStatusReady:     {OrderStatusCompleted, OrderStatusCancelled},
	OrderStatusCompleted: {},
	OrderStatusCancelled: {},
}

// CanTransitionTo reports whether an order may move from s to next. Staying in
// the same status is allowed (e.g. when only the note is being updated).
func (s OrderStatus) CanTransitionTo(next OrderStatus) bool {
	if s == next {
		return true
	}
	for _, allowed := range orderStatusTransitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// OrderItem is a single line in an order (a menu item plus quantity).
// ID and OrderID are persistence-only fields and are not exposed in the API.
type OrderItem struct {
	ID         uint   `gorm:"primaryKey" json:"-"`
	OrderID    string `gorm:"index" json:"-"`
	MenuItemID string `json:"menu_item_id"`
	Name       string `json:"name"`       // snapshot of the name at order time
	UnitPrice  int64  `json:"unit_price"` // snapshot of the unit price at order time, in cents
	Quantity   int    `json:"quantity"`
	Subtotal   int64  `json:"subtotal"` // cents
}

// Order represents a customer order.
type Order struct {
	ID           string      `gorm:"primaryKey" json:"id"`
	CustomerName string      `json:"customer_name"`
	Items        []OrderItem `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE" json:"items"`
	TotalAmount  int64       `json:"total_amount"` // cents
	Status       OrderStatus `json:"status"`
	Note         string      `json:"note"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}
