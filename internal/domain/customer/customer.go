// Package customer defines customer-account entities owned by a tenant.
package customer

import "github.com/google/uuid"

// Status represents the lifecycle state of a customer account.
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusArchived Status = "archived"
)

// Account identifies the commercial customer entity attached to contracts,
// billing facts, and leakage cases.
type Account struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	ExternalID       string
	Name             string
	Segment          string
	BillingProfileID uuid.UUID
	Status           Status
	Metadata         map[string]any
}

// Customer keeps backward compatibility with code that still refers to a
// customer instead of a customer account.
type Customer = Account
