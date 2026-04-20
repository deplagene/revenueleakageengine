package contract

import "github.com/google/uuid"

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
