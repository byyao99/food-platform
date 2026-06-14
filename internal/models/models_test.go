package models

import "testing"

func TestOrderStatusCanTransitionTo(t *testing.T) {
	tests := []struct {
		from OrderStatus
		to   OrderStatus
		want bool
	}{
		{OrderStatusPending, OrderStatusPending, true},      // same status (note-only update)
		{OrderStatusPending, OrderStatusPreparing, true},    // forward
		{OrderStatusPending, OrderStatusCancelled, true},    // cancel from any non-terminal
		{OrderStatusPending, OrderStatusReady, false},       // skipping ahead
		{OrderStatusPreparing, OrderStatusReady, true},      // forward
		{OrderStatusReady, OrderStatusCompleted, true},      // forward
		{OrderStatusReady, OrderStatusPreparing, false},     // backward
		{OrderStatusCompleted, OrderStatusCancelled, false}, // terminal
		{OrderStatusCompleted, OrderStatusCompleted, true},  // same terminal status
		{OrderStatusCancelled, OrderStatusPending, false},   // terminal
	}
	for _, tt := range tests {
		if got := tt.from.CanTransitionTo(tt.to); got != tt.want {
			t.Errorf("%s -> %s: got %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestOrderStatusValid(t *testing.T) {
	if !OrderStatusPending.Valid() {
		t.Error("pending should be valid")
	}
	if OrderStatus("bogus").Valid() {
		t.Error("bogus should be invalid")
	}
}

func TestRoleValid(t *testing.T) {
	for _, r := range []Role{RoleCustomer, RoleStaff, RoleAdmin} {
		if !r.Valid() {
			t.Errorf("%s should be valid", r)
		}
	}
	if Role("superuser").Valid() {
		t.Error("superuser should be invalid")
	}
}
