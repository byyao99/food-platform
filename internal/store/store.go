package store

import (
	"errors"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"food-platform/internal/models"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("resource not found")

// ListOptions controls pagination and sorting for list queries.
// Sort is a column name; it is validated against a per-resource allowlist
// before reaching SQL, so it is never a SQL-injection vector. Order is
// "asc" or "desc". Zero values fall back to each list method's defaults.
type ListOptions struct {
	Offset int
	Limit  int
	Sort   string
	Order  string
}

// orderClause builds a safe "<column> <direction>" ORDER BY fragment.
// opts.Sort is honored only if present in allowed; otherwise defaultSort is
// used. Direction is "asc"/"desc", defaulting to defaultOrder.
func orderClause(opts ListOptions, allowed map[string]bool, defaultSort, defaultOrder string) string {
	col := defaultSort
	if opts.Sort != "" && allowed[opts.Sort] {
		col = opts.Sort
	}
	dir := defaultOrder
	switch strings.ToLower(opts.Order) {
	case "asc":
		dir = "asc"
	case "desc":
		dir = "desc"
	}
	return col + " " + dir
}

// menuSortColumns are the columns clients may sort the menu by.
var menuSortColumns = map[string]bool{
	"name": true, "price": true, "category": true,
	"available": true, "created_at": true, "updated_at": true,
}

// orderSortColumns are the columns clients may sort orders by.
var orderSortColumns = map[string]bool{
	"customer_name": true, "total_amount": true, "status": true,
	"created_at": true, "updated_at": true,
}

// userSortColumns are the columns admins may sort the user list by.
var userSortColumns = map[string]bool{
	"username": true, "role": true, "created_at": true, "updated_at": true,
}

// Store persists menu items and orders in a SQLite database via GORM.
type Store struct {
	db *gorm.DB
}

// Open opens the SQLite database at dsn, runs migrations, and returns a Store.
// dsn is typically a file path such as "food-platform.db".
func Open(dsn string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Warn),
		TranslateError: true, // surface gorm.ErrDuplicatedKey etc. as typed errors
	})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&models.MenuItem{}, &models.Order{}, &models.OrderItem{}, &models.User{}); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

// Ping verifies the database connection is alive.
func (s *Store) Ping() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Ping()
}

// translate maps GORM's not-found error to the package sentinel.
func translate(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// ---------- Menu ----------

// MenuFilter narrows a ListMenu query. Zero-value fields are ignored. Category
// matches case-insensitively (exact); Available, when set, filters by serving
// status.
type MenuFilter struct {
	Category  string
	Available *bool
}

// ListMenu returns a page of menu items matching filter, plus the total count of
// matches. Defaults to oldest-first by creation time.
func (s *Store) ListMenu(opts ListOptions, filter MenuFilter) ([]models.MenuItem, int64, error) {
	apply := func(q *gorm.DB) *gorm.DB {
		if filter.Category != "" {
			q = q.Where("LOWER(category) = LOWER(?)", filter.Category)
		}
		if filter.Available != nil {
			q = q.Where("available = ?", *filter.Available)
		}
		return q
	}

	var total int64
	if err := apply(s.db.Model(&models.MenuItem{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := []models.MenuItem{}
	q := apply(s.db.Model(&models.MenuItem{})).
		Order(orderClause(opts, menuSortColumns, "created_at", "asc")).
		Offset(opts.Offset)
	if opts.Limit > 0 {
		q = q.Limit(opts.Limit)
	}
	if err := q.Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetMenuItem returns a menu item by ID.
func (s *Store) GetMenuItem(id string) (models.MenuItem, error) {
	var item models.MenuItem
	if err := s.db.First(&item, "id = ?", id).Error; err != nil {
		return models.MenuItem{}, translate(err)
	}
	return item, nil
}

// CreateMenuItem persists a menu item. The caller supplies the ID; timestamps are set by GORM.
func (s *Store) CreateMenuItem(item models.MenuItem) (models.MenuItem, error) {
	if err := s.db.Create(&item).Error; err != nil {
		return models.MenuItem{}, err
	}
	return item, nil
}

// UpdateMenuItem updates an existing menu item, preserving its ID and CreatedAt.
func (s *Store) UpdateMenuItem(id string, item models.MenuItem) (models.MenuItem, error) {
	var existing models.MenuItem
	if err := s.db.First(&existing, "id = ?", id).Error; err != nil {
		return models.MenuItem{}, translate(err)
	}
	item.ID = existing.ID
	item.CreatedAt = existing.CreatedAt
	if err := s.db.Save(&item).Error; err != nil {
		return models.MenuItem{}, err
	}
	return item, nil
}

// DeleteMenuItem removes a menu item.
func (s *Store) DeleteMenuItem(id string) error {
	res := s.db.Delete(&models.MenuItem{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- Order ----------

// OrderFilter narrows a ListOrders query. A non-empty UserID restricts results
// to orders placed by that account.
type OrderFilter struct {
	UserID string
}

// ListOrders returns a page of orders (with their items) matching filter, plus
// the total count of matches. Defaults to newest-first by creation time.
func (s *Store) ListOrders(opts ListOptions, filter OrderFilter) ([]models.Order, int64, error) {
	apply := func(q *gorm.DB) *gorm.DB {
		if filter.UserID != "" {
			q = q.Where("user_id = ?", filter.UserID)
		}
		return q
	}

	var total int64
	if err := apply(s.db.Model(&models.Order{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	orders := []models.Order{}
	q := apply(s.db.Preload("Items")).
		Order(orderClause(opts, orderSortColumns, "created_at", "desc")).
		Offset(opts.Offset)
	if opts.Limit > 0 {
		q = q.Limit(opts.Limit)
	}
	if err := q.Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// GetOrder returns an order, with its items, by ID.
func (s *Store) GetOrder(id string) (models.Order, error) {
	var order models.Order
	if err := s.db.Preload("Items").First(&order, "id = ?", id).Error; err != nil {
		return models.Order{}, translate(err)
	}
	return order, nil
}

// CreateOrder inserts a new order and its items in one transaction.
// The caller supplies the order ID.
func (s *Store) CreateOrder(order models.Order) (models.Order, error) {
	if err := s.db.Create(&order).Error; err != nil {
		return models.Order{}, err
	}
	return order, nil
}

// UpdateOrder updates an order's status and note only; line items are untouched.
func (s *Store) UpdateOrder(id string, order models.Order) (models.Order, error) {
	res := s.db.Model(&models.Order{}).Where("id = ?", id).Updates(map[string]any{
		"status": order.Status,
		"note":   order.Note,
	})
	if res.Error != nil {
		return models.Order{}, res.Error
	}
	if res.RowsAffected == 0 {
		return models.Order{}, ErrNotFound
	}
	return s.GetOrder(id)
}

// DeleteOrder removes an order and its items in one transaction.
func (s *Store) DeleteOrder(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("order_id = ?", id).Delete(&models.OrderItem{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&models.Order{}, "id = ?", id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}
