package customer

import "github.com/google/uuid"

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusArchived Status = "archived"
)

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

type Customer = Account
