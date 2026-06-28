package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"food-platform/internal/auth"
	"food-platform/internal/models"
	"food-platform/internal/router"
	"food-platform/internal/store"
)

// testEnv bundles the wired router and its dependencies for a single test.
type testEnv struct {
	r  *gin.Engine
	s  *store.Store
	am *auth.Manager
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	am := auth.NewManager([]byte("test-secret"), time.Hour)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &testEnv{r: router.New(s, am, log), s: s, am: am}
}

// token creates a user with the given role and returns a bearer token for it.
func (e *testEnv) token(t *testing.T, username string, role models.Role) string {
	t.Helper()
	hash, _ := auth.HashPassword("password123")
	user, err := e.s.CreateUser(models.User{
		ID: uuid.NewString(), Username: username, PasswordHash: hash, Role: role,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	tok, err := e.am.Issue(user.ID, user.Username, string(user.Role))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return tok
}

// seedMenuItem inserts an available menu item directly into the store.
func (e *testEnv) seedMenuItem(t *testing.T, name string, price int64, available bool) models.MenuItem {
	t.Helper()
	item, err := e.s.CreateMenuItem(models.MenuItem{
		ID: uuid.NewString(), Name: name, Price: price, Category: "Main", Available: available,
	})
	if err != nil {
		t.Fatalf("CreateMenuItem: %v", err)
	}
	return item
}

// do issues an HTTP request against the router. body may be nil; token may be "".
func (e *testEnv) do(t *testing.T, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.r.ServeHTTP(rec, req)
	return rec
}

// decodeData unmarshals the "data" field of a JSON response into v.
func decodeData(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v (body: %s)", err, rec.Body.String())
	}
	if err := json.Unmarshal(envelope.Data, v); err != nil {
		t.Fatalf("decode data: %v", err)
	}
}

func TestMenuListIsPublic(t *testing.T) {
	e := setup(t)
	if rec := e.do(t, http.MethodGet, "/api/v1/menu", nil, ""); rec.Code != http.StatusOK {
		t.Errorf("GET /menu without token: got %d, want 200", rec.Code)
	}
}

func TestMenuCreateRequiresAdmin(t *testing.T) {
	e := setup(t)
	payload := map[string]any{"name": "Burger", "price": 1500, "category": "Main"}

	if rec := e.do(t, http.MethodPost, "/api/v1/menu", payload, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: got %d, want 401", rec.Code)
	}
	customer := e.token(t, "cust", models.RoleCustomer)
	if rec := e.do(t, http.MethodPost, "/api/v1/menu", payload, customer); rec.Code != http.StatusForbidden {
		t.Errorf("customer token: got %d, want 403", rec.Code)
	}
	admin := e.token(t, "admin", models.RoleAdmin)
	if rec := e.do(t, http.MethodPost, "/api/v1/menu", payload, admin); rec.Code != http.StatusCreated {
		t.Errorf("admin token: got %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestMenuListFiltering(t *testing.T) {
	e := setup(t)
	// seedMenuItem always uses category "Main"; vary only name and availability.
	e.seedMenuItem(t, "Burger", 1500, true)
	e.seedMenuItem(t, "Cola", 300, true)
	e.seedMenuItem(t, "Sold Out", 900, false)

	total := func(t *testing.T, path string) int {
		t.Helper()
		rec := e.do(t, http.MethodGet, path, nil, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: got %d, want 200 (body: %s)", path, rec.Code, rec.Body.String())
		}
		var body struct {
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body.Pagination.Total
	}

	// seedMenuItem uses category "Main"; all three share it (case-insensitive).
	if got := total(t, "/api/v1/menu?category=main"); got != 3 {
		t.Errorf("category=main total: got %d, want 3", got)
	}
	// Only the two available items show when filtering available=true.
	if got := total(t, "/api/v1/menu?available=true"); got != 2 {
		t.Errorf("available=true total: got %d, want 2", got)
	}
	if got := total(t, "/api/v1/menu?available=false"); got != 1 {
		t.Errorf("available=false total: got %d, want 1", got)
	}
	// A non-boolean available value is rejected.
	if rec := e.do(t, http.MethodGet, "/api/v1/menu?available=maybe", nil, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("available=maybe: got %d, want 400", rec.Code)
	}
}

func TestMenuUpdateRequiresAvailable(t *testing.T) {
	e := setup(t)
	admin := e.token(t, "admin", models.RoleAdmin)
	item := e.seedMenuItem(t, "Burger", 1500, false) // starts unavailable

	// Omitting "available" on update is rejected (would otherwise re-enable it).
	noAvail := map[string]any{"name": "Burger", "price": 1500, "category": "Main"}
	if rec := e.do(t, http.MethodPut, "/api/v1/menu/"+item.ID, noAvail, admin); rec.Code != http.StatusBadRequest {
		t.Errorf("update without available: got %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}

	// Explicit available is honored.
	withAvail := map[string]any{"name": "Burger", "price": 1500, "category": "Main", "available": false}
	rec := e.do(t, http.MethodPut, "/api/v1/menu/"+item.ID, withAvail, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("update with available: got %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var updated models.MenuItem
	decodeData(t, rec, &updated)
	if updated.Available {
		t.Error("item should remain unavailable")
	}
}

func TestMenuCreateRejectsExcessivePrice(t *testing.T) {
	e := setup(t)
	admin := e.token(t, "admin", models.RoleAdmin)
	payload := map[string]any{"name": "Gold Burger", "price": 100000001, "category": "Main"}
	if rec := e.do(t, http.MethodPost, "/api/v1/menu", payload, admin); rec.Code != http.StatusBadRequest {
		t.Errorf("excessive price: got %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestOrderCreatePricingIsServerAuthoritative(t *testing.T) {
	e := setup(t)
	item := e.seedMenuItem(t, "Burger", 1500, true)
	customer := e.token(t, "cust", models.RoleCustomer)

	// Client sends only id + quantity (and a bogus price that must be ignored).
	payload := map[string]any{
		"items": []map[string]any{{"menu_item_id": item.ID, "quantity": 2, "unit_price": 1}},
	}
	rec := e.do(t, http.MethodPost, "/api/v1/orders", payload, customer)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create order: got %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var order models.Order
	decodeData(t, rec, &order)
	if order.TotalAmount != 3000 {
		t.Errorf("total: got %d, want 3000", order.TotalAmount)
	}
	if len(order.Items) != 1 || order.Items[0].UnitPrice != 1500 {
		t.Errorf("unit price should come from the menu, got %+v", order.Items)
	}
	if order.Status != models.OrderStatusPending {
		t.Errorf("new order status: got %s, want pending", order.Status)
	}
}

func TestOrderCreateRejectsUnavailableItem(t *testing.T) {
	e := setup(t)
	item := e.seedMenuItem(t, "Sold Out", 1500, false)
	customer := e.token(t, "cust", models.RoleCustomer)
	payload := map[string]any{
		"items": []map[string]any{{"menu_item_id": item.ID, "quantity": 1}},
	}
	if rec := e.do(t, http.MethodPost, "/api/v1/orders", payload, customer); rec.Code != http.StatusBadRequest {
		t.Errorf("unavailable item: got %d, want 400", rec.Code)
	}
}

func TestOrderStatusTransitionEnforced(t *testing.T) {
	e := setup(t)
	item := e.seedMenuItem(t, "Burger", 1500, true)
	customer := e.token(t, "cust", models.RoleCustomer)
	staff := e.token(t, "staff", models.RoleStaff)

	rec := e.do(t, http.MethodPost, "/api/v1/orders", map[string]any{
		"items": []map[string]any{{"menu_item_id": item.ID, "quantity": 1}},
	}, customer)
	var order models.Order
	decodeData(t, rec, &order)

	// pending -> ready skips a step and must be rejected.
	illegal := e.do(t, http.MethodPut, "/api/v1/orders/"+order.ID, map[string]any{"status": "ready"}, staff)
	if illegal.Code != http.StatusBadRequest {
		t.Errorf("illegal transition: got %d, want 400", illegal.Code)
	}
	// pending -> preparing is allowed.
	legal := e.do(t, http.MethodPut, "/api/v1/orders/"+order.ID, map[string]any{"status": "preparing"}, staff)
	if legal.Code != http.StatusOK {
		t.Errorf("legal transition: got %d, want 200 (body: %s)", legal.Code, legal.Body.String())
	}
}

func TestOrderListRequiresStaff(t *testing.T) {
	e := setup(t)
	customer := e.token(t, "cust", models.RoleCustomer)
	staff := e.token(t, "staff", models.RoleStaff)

	if rec := e.do(t, http.MethodGet, "/api/v1/orders", nil, customer); rec.Code != http.StatusForbidden {
		t.Errorf("customer listing orders: got %d, want 403", rec.Code)
	}
	if rec := e.do(t, http.MethodGet, "/api/v1/orders", nil, staff); rec.Code != http.StatusOK {
		t.Errorf("staff listing orders: got %d, want 200", rec.Code)
	}
}

// placeOrder creates a one-item order as the given token and returns it.
func (e *testEnv) placeOrder(t *testing.T, item models.MenuItem, token string) models.Order {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/api/v1/orders", map[string]any{
		"items": []map[string]any{{"menu_item_id": item.ID, "quantity": 1}},
	}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("place order: got %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var order models.Order
	decodeData(t, rec, &order)
	return order
}

func TestOrderCreateUsesAccountIdentity(t *testing.T) {
	e := setup(t)
	item := e.seedMenuItem(t, "Burger", 1500, true)
	customer := e.token(t, "alice", models.RoleCustomer)
	alice, _ := e.s.GetUserByUsername("alice")

	// A client-supplied customer_name must be ignored in favor of the account.
	rec := e.do(t, http.MethodPost, "/api/v1/orders", map[string]any{
		"customer_name": "Impersonated",
		"items":         []map[string]any{{"menu_item_id": item.ID, "quantity": 1}},
	}, customer)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var order models.Order
	decodeData(t, rec, &order)
	if order.UserID != alice.ID {
		t.Errorf("user_id: got %q, want %q", order.UserID, alice.ID)
	}
	if order.CustomerName != "alice" {
		t.Errorf("customer_name: got %q, want %q (client value must be ignored)", order.CustomerName, "alice")
	}
}

func TestOrderGetOwnership(t *testing.T) {
	e := setup(t)
	item := e.seedMenuItem(t, "Burger", 1500, true)
	alice := e.token(t, "alice", models.RoleCustomer)
	bob := e.token(t, "bob", models.RoleCustomer)
	staff := e.token(t, "staff", models.RoleStaff)
	order := e.placeOrder(t, item, alice)
	path := "/api/v1/orders/" + order.ID

	if rec := e.do(t, http.MethodGet, path, nil, alice); rec.Code != http.StatusOK {
		t.Errorf("owner reading own order: got %d, want 200", rec.Code)
	}
	// A different customer must not be able to read it; 404 hides its existence.
	if rec := e.do(t, http.MethodGet, path, nil, bob); rec.Code != http.StatusNotFound {
		t.Errorf("non-owner reading order: got %d, want 404", rec.Code)
	}
	if rec := e.do(t, http.MethodGet, path, nil, staff); rec.Code != http.StatusOK {
		t.Errorf("staff reading any order: got %d, want 200", rec.Code)
	}
}

func TestOrderMineScoping(t *testing.T) {
	e := setup(t)
	item := e.seedMenuItem(t, "Burger", 1500, true)
	alice := e.token(t, "alice", models.RoleCustomer)
	bob := e.token(t, "bob", models.RoleCustomer)
	e.placeOrder(t, item, alice)
	e.placeOrder(t, item, bob)
	e.placeOrder(t, item, bob)

	mineTotal := func(t *testing.T, token string) int {
		t.Helper()
		rec := e.do(t, http.MethodGet, "/api/v1/orders/mine", nil, token)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /orders/mine: got %d, want 200 (body: %s)", rec.Code, rec.Body.String())
		}
		var body struct {
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body.Pagination.Total
	}

	if got := mineTotal(t, alice); got != 1 {
		t.Errorf("alice's orders: got %d, want 1", got)
	}
	if got := mineTotal(t, bob); got != 2 {
		t.Errorf("bob's orders: got %d, want 2", got)
	}
}

func TestRegisterAndLogin(t *testing.T) {
	e := setup(t)
	creds := map[string]any{"username": "alice", "password": "password123"}

	reg := e.do(t, http.MethodPost, "/api/v1/auth/register", creds, "")
	if reg.Code != http.StatusCreated {
		t.Fatalf("register: got %d, want 201 (body: %s)", reg.Code, reg.Body.String())
	}
	// Registration must not let a client choose an elevated role.
	var regData struct {
		Token string      `json:"token"`
		User  models.User `json:"user"`
	}
	decodeData(t, reg, &regData)
	if regData.Token == "" {
		t.Error("register did not return a token")
	}
	if regData.User.Role != models.RoleCustomer {
		t.Errorf("new account role: got %s, want customer", regData.User.Role)
	}

	// Duplicate username -> 409.
	if dup := e.do(t, http.MethodPost, "/api/v1/auth/register", creds, ""); dup.Code != http.StatusConflict {
		t.Errorf("duplicate register: got %d, want 409", dup.Code)
	}

	// Correct login -> 200; wrong password -> 401.
	if ok := e.do(t, http.MethodPost, "/api/v1/auth/login", creds, ""); ok.Code != http.StatusOK {
		t.Errorf("login: got %d, want 200", ok.Code)
	}
	bad := map[string]any{"username": "alice", "password": "wrong-password"}
	if rec := e.do(t, http.MethodPost, "/api/v1/auth/login", bad, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: got %d, want 401", rec.Code)
	}
}

func TestRegisterPasswordValidation(t *testing.T) {
	e := setup(t)

	// Only digits -> one character class -> 400.
	weak := map[string]any{"username": "bob", "password": "12345678"}
	if rec := e.do(t, http.MethodPost, "/api/v1/auth/register", weak, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("weak password: got %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}

	// Longer than bcrypt's 72-byte limit -> 400.
	long := map[string]any{"username": "bob", "password": strings.Repeat("Ab1", 25)} // 75 bytes
	if rec := e.do(t, http.MethodPost, "/api/v1/auth/register", long, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("over-long password: got %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}

	// Two classes within the limit -> 201.
	ok := map[string]any{"username": "bob", "password": "password1"}
	if rec := e.do(t, http.MethodPost, "/api/v1/auth/register", ok, ""); rec.Code != http.StatusCreated {
		t.Errorf("valid password: got %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestUsernameNormalization(t *testing.T) {
	e := setup(t)
	creds := map[string]any{"username": "Alice", "password": "password1"}
	if rec := e.do(t, http.MethodPost, "/api/v1/auth/register", creds, ""); rec.Code != http.StatusCreated {
		t.Fatalf("register Alice: got %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	// A differently-cased, whitespace-padded variant is the same account -> 409.
	dup := map[string]any{"username": " alice ", "password": "password1"}
	if rec := e.do(t, http.MethodPost, "/api/v1/auth/register", dup, ""); rec.Code != http.StatusConflict {
		t.Errorf("duplicate variant: got %d, want 409 (body: %s)", rec.Code, rec.Body.String())
	}

	// Login is case-insensitive too.
	login := map[string]any{"username": "ALICE", "password": "password1"}
	if rec := e.do(t, http.MethodPost, "/api/v1/auth/login", login, ""); rec.Code != http.StatusOK {
		t.Errorf("case-insensitive login: got %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestAdminCreatesStaffAccount(t *testing.T) {
	e := setup(t)
	admin := e.token(t, "admin", models.RoleAdmin)
	customer := e.token(t, "cust", models.RoleCustomer)
	newStaff := map[string]any{"username": "kitchen", "password": "password123", "role": "staff"}

	// Only admins may provision accounts.
	if rec := e.do(t, http.MethodPost, "/api/v1/users", newStaff, customer); rec.Code != http.StatusForbidden {
		t.Errorf("customer creating user: got %d, want 403", rec.Code)
	}

	// An unknown role is rejected.
	bogus := map[string]any{"username": "x", "password": "password123", "role": "superuser"}
	if rec := e.do(t, http.MethodPost, "/api/v1/users", bogus, admin); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid role: got %d, want 400", rec.Code)
	}

	rec := e.do(t, http.MethodPost, "/api/v1/users", newStaff, admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin creating staff: got %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var created models.User
	decodeData(t, rec, &created)
	if created.Role != models.RoleStaff {
		t.Errorf("created role: got %s, want staff", created.Role)
	}

	// The new staff account can log in and is recognized as staff.
	login := map[string]any{"username": "kitchen", "password": "password123"}
	if ok := e.do(t, http.MethodPost, "/api/v1/auth/login", login, ""); ok.Code != http.StatusOK {
		t.Errorf("new staff login: got %d, want 200", ok.Code)
	}
}

func TestUserListRoleUpdateAndDelete(t *testing.T) {
	e := setup(t)
	admin := e.token(t, "admin", models.RoleAdmin)
	customer := e.token(t, "cust", models.RoleCustomer)

	// Listing is admin-only.
	if rec := e.do(t, http.MethodGet, "/api/v1/users", nil, customer); rec.Code != http.StatusForbidden {
		t.Errorf("customer listing users: got %d, want 403", rec.Code)
	}
	listRec := e.do(t, http.MethodGet, "/api/v1/users", nil, admin)
	if listRec.Code != http.StatusOK {
		t.Fatalf("admin listing users: got %d, want 200", listRec.Code)
	}
	var listBody struct {
		Data       []models.User `json:"data"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listBody.Pagination.Total != 2 {
		t.Errorf("user total: got %d, want 2", listBody.Pagination.Total)
	}

	// Promote the customer to staff.
	target := listBody.Data[0]
	if target.Role == models.RoleAdmin { // ensure we target the non-admin
		target = listBody.Data[1]
	}
	roleRec := e.do(t, http.MethodPut, "/api/v1/users/"+target.ID+"/role", map[string]any{"role": "staff"}, admin)
	if roleRec.Code != http.StatusOK {
		t.Fatalf("update role: got %d, want 200 (body: %s)", roleRec.Code, roleRec.Body.String())
	}
	var updated models.User
	decodeData(t, roleRec, &updated)
	if updated.Role != models.RoleStaff {
		t.Errorf("updated role: got %s, want staff", updated.Role)
	}

	// Delete it, then a second delete should 404.
	if rec := e.do(t, http.MethodDelete, "/api/v1/users/"+target.ID, nil, admin); rec.Code != http.StatusNoContent {
		t.Errorf("delete user: got %d, want 204", rec.Code)
	}
	if rec := e.do(t, http.MethodDelete, "/api/v1/users/"+target.ID, nil, admin); rec.Code != http.StatusNotFound {
		t.Errorf("re-delete user: got %d, want 404", rec.Code)
	}
}

func TestUserListFiltering(t *testing.T) {
	e := setup(t)
	admin := e.token(t, "admin", models.RoleAdmin)
	e.token(t, "kitchen1", models.RoleStaff)
	e.token(t, "kitchen2", models.RoleStaff)
	e.token(t, "alice", models.RoleCustomer)

	total := func(t *testing.T, path string) int {
		t.Helper()
		rec := e.do(t, http.MethodGet, path, nil, admin)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: got %d, want 200 (body: %s)", path, rec.Code, rec.Body.String())
		}
		var body struct {
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body.Pagination.Total
	}

	if got := total(t, "/api/v1/users?role=staff"); got != 2 {
		t.Errorf("role=staff total: got %d, want 2", got)
	}
	if got := total(t, "/api/v1/users?q=alic"); got != 1 {
		t.Errorf("q=alic total: got %d, want 1", got)
	}
	// An unknown role is rejected.
	if rec := e.do(t, http.MethodGet, "/api/v1/users?role=superuser", nil, admin); rec.Code != http.StatusBadRequest {
		t.Errorf("role=superuser: got %d, want 400", rec.Code)
	}
}

func TestUpdateRoleToSameRoleIsNoOp(t *testing.T) {
	e := setup(t)
	admin := e.token(t, "admin", models.RoleAdmin)
	e.token(t, "kitchen", models.RoleStaff)
	staff, err := e.s.GetUserByUsername("kitchen")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}

	// Setting the role it already has succeeds and leaves it unchanged.
	rec := e.do(t, http.MethodPut, "/api/v1/users/"+staff.ID+"/role", map[string]any{"role": "staff"}, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("same-role update: got %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var updated models.User
	decodeData(t, rec, &updated)
	if updated.Role != models.RoleStaff {
		t.Errorf("role after no-op: got %s, want staff", updated.Role)
	}
}

func TestSelfServiceChangePassword(t *testing.T) {
	e := setup(t)
	// token() seeds the account with password "password123".
	tok := e.token(t, "alice", models.RoleCustomer)

	// Unauthenticated -> 401.
	body := map[string]any{"old_password": "password123", "new_password": "newpass123"}
	if rec := e.do(t, http.MethodPut, "/api/v1/auth/password", body, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: got %d, want 401", rec.Code)
	}

	// Wrong current password -> 400 (deliberately not 401, which would log out).
	wrong := map[string]any{"old_password": "nope12345", "new_password": "newpass123"}
	if rec := e.do(t, http.MethodPut, "/api/v1/auth/password", wrong, tok); rec.Code != http.StatusBadRequest {
		t.Errorf("wrong old password: got %d, want 400", rec.Code)
	}

	// Weak new password -> 400.
	weak := map[string]any{"old_password": "password123", "new_password": "12345678"}
	if rec := e.do(t, http.MethodPut, "/api/v1/auth/password", weak, tok); rec.Code != http.StatusBadRequest {
		t.Errorf("weak new password: got %d, want 400", rec.Code)
	}

	// Valid change -> 204.
	if rec := e.do(t, http.MethodPut, "/api/v1/auth/password", body, tok); rec.Code != http.StatusNoContent {
		t.Fatalf("change password: got %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	// New password logs in; old one no longer works.
	if rec := e.do(t, http.MethodPost, "/api/v1/auth/login", map[string]any{"username": "alice", "password": "newpass123"}, ""); rec.Code != http.StatusOK {
		t.Errorf("login with new password: got %d, want 200", rec.Code)
	}
	if rec := e.do(t, http.MethodPost, "/api/v1/auth/login", map[string]any{"username": "alice", "password": "password123"}, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("login with old password: got %d, want 401", rec.Code)
	}
}

func TestAdminResetPassword(t *testing.T) {
	e := setup(t)
	admin := e.token(t, "admin", models.RoleAdmin)
	customer := e.token(t, "cust", models.RoleCustomer)
	target, err := e.s.GetUserByUsername("cust")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	body := map[string]any{"password": "resetpass1"}

	// Non-admin -> 403.
	if rec := e.do(t, http.MethodPut, "/api/v1/users/"+target.ID+"/password", body, customer); rec.Code != http.StatusForbidden {
		t.Errorf("customer resetting: got %d, want 403", rec.Code)
	}
	// Unknown id -> 404.
	if rec := e.do(t, http.MethodPut, "/api/v1/users/does-not-exist/password", body, admin); rec.Code != http.StatusNotFound {
		t.Errorf("unknown id: got %d, want 404", rec.Code)
	}
	// Weak password -> 400.
	if rec := e.do(t, http.MethodPut, "/api/v1/users/"+target.ID+"/password", map[string]any{"password": "12345678"}, admin); rec.Code != http.StatusBadRequest {
		t.Errorf("weak password: got %d, want 400", rec.Code)
	}
	// Valid reset -> 204, and the target can log in with the new password.
	if rec := e.do(t, http.MethodPut, "/api/v1/users/"+target.ID+"/password", body, admin); rec.Code != http.StatusNoContent {
		t.Fatalf("reset password: got %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
	if rec := e.do(t, http.MethodPost, "/api/v1/auth/login", map[string]any{"username": "cust", "password": "resetpass1"}, ""); rec.Code != http.StatusOK {
		t.Errorf("login after reset: got %d, want 200", rec.Code)
	}
}

func TestAdminCannotActOnSelf(t *testing.T) {
	e := setup(t)
	admin := e.token(t, "admin", models.RoleAdmin)
	self, err := e.s.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}

	if rec := e.do(t, http.MethodDelete, "/api/v1/users/"+self.ID, nil, admin); rec.Code != http.StatusForbidden {
		t.Errorf("admin deleting self: got %d, want 403", rec.Code)
	}
	demote := map[string]any{"role": "customer"}
	if rec := e.do(t, http.MethodPut, "/api/v1/users/"+self.ID+"/role", demote, admin); rec.Code != http.StatusForbidden {
		t.Errorf("admin demoting self: got %d, want 403", rec.Code)
	}
}

func TestRequestIDHeaderEchoed(t *testing.T) {
	e := setup(t)
	rec := e.do(t, http.MethodGet, "/health", nil, "")
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID response header to be set")
	}
}
