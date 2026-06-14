package store

import (
	"errors"
	"path/filepath"
	"testing"

	"food-platform/internal/models"
)

// newTestStore opens a fresh Store backed by a temporary SQLite file.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func seedMenuItem(t *testing.T, s *Store, name string, price int64, available bool) models.MenuItem {
	t.Helper()
	item, err := s.CreateMenuItem(models.MenuItem{
		ID: name + "-id", Name: name, Price: price, Category: "Main", Available: available,
	})
	if err != nil {
		t.Fatalf("CreateMenuItem: %v", err)
	}
	return item
}

func TestMenuCRUD(t *testing.T) {
	s := newTestStore(t)
	created := seedMenuItem(t, s, "Burger", 1200, true)

	got, err := s.GetMenuItem(created.ID)
	if err != nil {
		t.Fatalf("GetMenuItem: %v", err)
	}
	if got.Name != "Burger" || got.Price != 1200 {
		t.Errorf("unexpected item: %+v", got)
	}

	if _, err := s.GetMenuItem("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetMenuItem(missing): got %v, want ErrNotFound", err)
	}

	if err := s.DeleteMenuItem(created.ID); err != nil {
		t.Fatalf("DeleteMenuItem: %v", err)
	}
	if err := s.DeleteMenuItem(created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: got %v, want ErrNotFound", err)
	}
}

func TestListMenuPaginationAndTotal(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		seedMenuItem(t, s, string(rune('a'+i)), int64(100+i), true)
	}
	items, total, err := s.ListMenu(ListOptions{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("ListMenu: %v", err)
	}
	if total != 5 {
		t.Errorf("total: got %d, want 5", total)
	}
	if len(items) != 2 {
		t.Errorf("page size: got %d, want 2", len(items))
	}
}

func TestListMenuRejectsUnknownSortColumn(t *testing.T) {
	s := newTestStore(t)
	seedMenuItem(t, s, "Burger", 1200, true)
	// A would-be injection string is not in the allowlist, so it must be ignored
	// rather than reaching SQL. The query should still succeed.
	if _, _, err := s.ListMenu(ListOptions{Limit: 10, Sort: "price; DROP TABLE menu_items;--"}); err != nil {
		t.Fatalf("ListMenu with malicious sort should fall back safely, got %v", err)
	}
}

func TestCreateAndGetOrderWithItems(t *testing.T) {
	s := newTestStore(t)
	order, err := s.CreateOrder(models.Order{
		ID:           "order-1",
		CustomerName: "Bob",
		Status:       models.OrderStatusPending,
		TotalAmount:  2400,
		Items: []models.OrderItem{
			{MenuItemID: "m1", Name: "Burger", UnitPrice: 1200, Quantity: 2, Subtotal: 2400},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	got, err := s.GetOrder(order.ID)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Subtotal != 2400 {
		t.Errorf("items not preloaded correctly: %+v", got.Items)
	}
}

func TestDeleteOrderCascadesItems(t *testing.T) {
	s := newTestStore(t)
	s.CreateOrder(models.Order{
		ID: "order-1", CustomerName: "Bob", Status: models.OrderStatusPending,
		Items: []models.OrderItem{{MenuItemID: "m1", Name: "X", UnitPrice: 100, Quantity: 1, Subtotal: 100}},
	})
	if err := s.DeleteOrder("order-1"); err != nil {
		t.Fatalf("DeleteOrder: %v", err)
	}
	var leftover int64
	s.db.Model(&models.OrderItem{}).Where("order_id = ?", "order-1").Count(&leftover)
	if leftover != 0 {
		t.Errorf("expected items removed, found %d", leftover)
	}
}

func TestUserCRUD(t *testing.T) {
	s := newTestStore(t)
	user, err := s.CreateUser(models.User{ID: "u1", Username: "alice", PasswordHash: "hash", Role: models.RoleAdmin})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if got.ID != user.ID || got.Role != models.RoleAdmin {
		t.Errorf("unexpected user: %+v", got)
	}

	// Duplicate username must be rejected with the typed error.
	if _, err := s.CreateUser(models.User{ID: "u2", Username: "alice", PasswordHash: "h", Role: models.RoleCustomer}); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("duplicate username: got %v, want ErrUsernameTaken", err)
	}

	count, err := s.CountUsers()
	if err != nil || count != 1 {
		t.Errorf("CountUsers: got %d (err %v), want 1", count, err)
	}
}
