package store

import (
	"errors"

	"gorm.io/gorm"

	"food-platform/internal/models"
)

// ErrUsernameTaken is returned by CreateUser when the username already exists.
var ErrUsernameTaken = errors.New("username already taken")

// CreateUser persists a new user. The caller supplies the ID and password hash.
// It returns ErrUsernameTaken if the username is already in use.
func (s *Store) CreateUser(user models.User) (models.User, error) {
	if err := s.db.Create(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return models.User{}, ErrUsernameTaken
		}
		return models.User{}, err
	}
	return user, nil
}

// GetUserByUsername returns the user with the given username, or ErrNotFound.
func (s *Store) GetUserByUsername(username string) (models.User, error) {
	var user models.User
	if err := s.db.First(&user, "username = ?", username).Error; err != nil {
		return models.User{}, translate(err)
	}
	return user, nil
}

// CountUsers returns the total number of users; used to decide whether to seed
// the initial admin account on startup.
func (s *Store) CountUsers() (int64, error) {
	var n int64
	if err := s.db.Model(&models.User{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// UserFilter narrows a ListUsers query. Zero-value fields are ignored. Role
// matches exactly; Username matches a case-insensitive substring (usernames are
// stored normalized to lowercase).
type UserFilter struct {
	Role     models.Role
	Username string
}

// ListUsers returns a page of users matching filter, plus the total count of
// matches. Defaults to oldest-first by creation time.
func (s *Store) ListUsers(opts ListOptions, filter UserFilter) ([]models.User, int64, error) {
	apply := func(q *gorm.DB) *gorm.DB {
		if filter.Role != "" {
			q = q.Where("role = ?", filter.Role)
		}
		if filter.Username != "" {
			q = q.Where("username LIKE ?", "%"+filter.Username+"%")
		}
		return q
	}

	var total int64
	if err := apply(s.db.Model(&models.User{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	users := []models.User{}
	q := apply(s.db.Model(&models.User{})).
		Order(orderClause(opts, userSortColumns, "created_at", "asc")).
		Offset(opts.Offset)
	if opts.Limit > 0 {
		q = q.Limit(opts.Limit)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// GetUser returns a user by ID, or ErrNotFound.
func (s *Store) GetUser(id string) (models.User, error) {
	var user models.User
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		return models.User{}, translate(err)
	}
	return user, nil
}

// UpdateUserRole changes a user's role, returning the updated user and its
// previous role. When the requested role equals the current one it is a no-op
// (no write) and the returned old role equals the new role, so callers can tell
// nothing changed. It returns ErrNotFound if no such user exists.
func (s *Store) UpdateUserRole(id string, role models.Role) (user models.User, oldRole models.Role, err error) {
	user, err = s.GetUser(id)
	if err != nil {
		return models.User{}, "", err
	}
	oldRole = user.Role
	if oldRole == role {
		return user, oldRole, nil
	}
	if err = s.db.Model(&user).Update("role", role).Error; err != nil {
		return models.User{}, "", err
	}
	user.Role = role
	return user, oldRole, nil
}

// UpdateUserPassword sets a user's bcrypt password hash. It returns ErrNotFound
// if no such user exists.
func (s *Store) UpdateUserPassword(id, hash string) error {
	res := s.db.Model(&models.User{}).Where("id = ?", id).Update("password_hash", hash)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser removes a user, returning ErrNotFound if it does not exist.
func (s *Store) DeleteUser(id string) error {
	res := s.db.Delete(&models.User{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
