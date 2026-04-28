package contract

import (
	"errors"
	"strings"

	"github.com/google/uuid"
)

var (
	// ErrBillableItemIDRequired reports that a persisted billable item is
	// missing its identifier.
	ErrBillableItemIDRequired = errors.New("billable item id is required")
	// ErrBillableItemCodeRequired reports that a billable item lacks the stable
	// business code used by pricing rules.
	ErrBillableItemCodeRequired = errors.New("billable item code is required")
	// ErrBillableItemNameRequired reports that a billable item lacks a readable
	// name.
	ErrBillableItemNameRequired = errors.New("billable item name is required")
	// ErrBillableItemUnitRequired reports that a billable item lacks the unit
	// used by usage and invoice facts.
	ErrBillableItemUnitRequired = errors.New("billable item unit is required")
	// ErrPricingModeRequired reports that a billable item lacks a pricing mode.
	ErrPricingModeRequired = errors.New("pricing mode is required")
	// ErrPricingModeInvalid reports an unknown billable item pricing mode.
	ErrPricingModeInvalid = errors.New("pricing mode is invalid")
	// ErrBillableItemStatusRequired reports that a billable item lacks status.
	ErrBillableItemStatusRequired = errors.New("billable item status is required")
	// ErrBillableItemStatusInvalid reports an unknown billable item status.
	ErrBillableItemStatusInvalid = errors.New("billable item status is invalid")
)

// PricingMode describes how a billable item should be monetized.
type PricingMode string

const (
	PricingModeFixed  PricingMode = "fixed"
	PricingModeUsage  PricingMode = "usage"
	PricingModeTiered PricingMode = "tiered"
)

// BillableItemStatus indicates whether a billable item can be used in active
// contract and billing flows.
type BillableItemStatus string

const (
	BillableItemStatusActive   BillableItemStatus = "active"
	BillableItemStatusInactive BillableItemStatus = "inactive"
)

// BillableItem identifies the product, service, or metric that can generate
// revenue for a tenant.
type BillableItem struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Code        string
	Name        string
	Category    string
	Unit        string
	PricingMode PricingMode
	Status      BillableItemStatus
}

// Normalize returns a copy with trimmed business identifiers.
func (b BillableItem) Normalize() BillableItem {
	b.Code = strings.TrimSpace(b.Code)
	b.Name = strings.TrimSpace(b.Name)
	b.Category = strings.TrimSpace(b.Category)
	b.Unit = strings.TrimSpace(b.Unit)

	return b
}

// Validate checks the invariants required before an item can participate in
// contract pricing.
func (b BillableItem) Validate() error {
	if b.ID == uuid.Nil {
		return ErrBillableItemIDRequired
	}

	if b.TenantID == uuid.Nil {
		return ErrTenantRequired
	}

	if strings.TrimSpace(b.Code) == "" {
		return ErrBillableItemCodeRequired
	}

	if strings.TrimSpace(b.Name) == "" {
		return ErrBillableItemNameRequired
	}

	if strings.TrimSpace(b.Unit) == "" {
		return ErrBillableItemUnitRequired
	}

	if err := b.PricingMode.Validate(); err != nil {
		return err
	}

	if err := b.Status.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate checks that the pricing mode is supported.
func (m PricingMode) Validate() error {
	switch m {
	case "":
		return ErrPricingModeRequired
	case PricingModeFixed, PricingModeUsage, PricingModeTiered:
		return nil
	default:
		return ErrPricingModeInvalid
	}
}

// Validate checks that the billable item status is supported.
func (s BillableItemStatus) Validate() error {
	switch s {
	case "":
		return ErrBillableItemStatusRequired
	case BillableItemStatusActive, BillableItemStatusInactive:
		return nil
	default:
		return ErrBillableItemStatusInvalid
	}
}
