// Package contract defines commercial truth such as contracts, billable items,
// and contract terms used to calculate expected revenue.
package contract

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrContractIDRequired reports that a contract operation is missing the
	// contract identifier needed for an update or persisted entity.
	ErrContractIDRequired = errors.New("contract id is required")
	// ErrTenantRequired reports that a contract-scoped operation is missing its
	// tenant boundary.
	ErrTenantRequired = errors.New("tenant id is required")
	// ErrCustomerRequired reports that a contract is missing the customer it
	// commercializes.
	ErrCustomerRequired = errors.New("customer id is required")
	// ErrStatusRequired reports that a contract lifecycle status is missing.
	ErrStatusRequired = errors.New("contract status is required")
	// ErrStatusInvalid reports an unknown contract lifecycle status.
	ErrStatusInvalid = errors.New("contract status is invalid")
	// ErrStartDateRequired reports that a contract lacks its effective start
	// date.
	ErrStartDateRequired = errors.New("contract start date is required")
	// ErrContractDateRangeInvalid reports an end date that does not follow the
	// start date.
	ErrContractDateRangeInvalid = errors.New("contract end date must be after start date")
	// ErrCurrencyRequired reports that a contract is missing its settlement
	// currency.
	ErrCurrencyRequired = errors.New("contract currency is required")
	// ErrVersionInvalid reports a non-positive contract version.
	ErrVersionInvalid = errors.New("contract version must be greater than zero")
	// ErrBillingModelRequired reports that a contract is missing its billing
	// model.
	ErrBillingModelRequired = errors.New("billing model is required")
	// ErrBillingModelInvalid reports an unknown billing model.
	ErrBillingModelInvalid = errors.New("billing model is invalid")
)

// Status represents the lifecycle state of a contract.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusActive    Status = "active"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

// BillingModel identifies the pricing pattern used by a contract.
type BillingModel string

const (
	BillingModelFixedRecurring BillingModel = "fixed_recurring"
	BillingModelUsageBased     BillingModel = "usage_based"
)

// Contract stores the commercial agreement that defines how revenue should be
// earned from a customer account.
type Contract struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	CustomerID   uuid.UUID
	ExternalID   string
	Status       Status
	StartDate    time.Time
	EndDate      *time.Time
	Currency     string
	Version      int
	BillingModel BillingModel
	SignedAt     *time.Time
	Metadata     map[string]any
}

// Normalize returns a copy with stable casing and non-nil collection fields.
func (c Contract) Normalize() Contract {
	c.ExternalID = strings.TrimSpace(c.ExternalID)
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))

	if c.StartDate.Location() != time.UTC {
		c.StartDate = c.StartDate.UTC()
	}

	if c.EndDate != nil {
		endDate := c.EndDate.UTC()
		c.EndDate = &endDate
	}

	if c.SignedAt != nil {
		signedAt := c.SignedAt.UTC()
		c.SignedAt = &signedAt
	}

	if c.Metadata == nil {
		c.Metadata = map[string]any{}
	}

	return c
}

// Validate checks the business invariants required before a contract can be
// used as the commercial source of truth.
func (c Contract) Validate() error {
	if c.ID == uuid.Nil {
		return ErrContractIDRequired
	}

	if c.TenantID == uuid.Nil {
		return ErrTenantRequired
	}

	if c.CustomerID == uuid.Nil {
		return ErrCustomerRequired
	}

	if err := c.Status.Validate(); err != nil {
		return err
	}

	if c.StartDate.IsZero() {
		return ErrStartDateRequired
	}

	if c.EndDate != nil && !c.EndDate.After(c.StartDate) {
		return ErrContractDateRangeInvalid
	}

	if strings.TrimSpace(c.Currency) == "" {
		return ErrCurrencyRequired
	}

	if c.Version <= 0 {
		return ErrVersionInvalid
	}

	if err := c.BillingModel.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate checks that the lifecycle status is one of the supported states.
func (s Status) Validate() error {
	switch s {
	case "":
		return ErrStatusRequired
	case StatusDraft, StatusActive, StatusExpired, StatusCancelled:
		return nil
	default:
		return ErrStatusInvalid
	}
}

// Validate checks that the billing model is one of the supported models.
func (m BillingModel) Validate() error {
	switch m {
	case "":
		return ErrBillingModelRequired
	case BillingModelFixedRecurring, BillingModelUsageBased:
		return nil
	default:
		return ErrBillingModelInvalid
	}
}
