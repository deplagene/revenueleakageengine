package contract

import "github.com/google/uuid"

type PricingMode string

const (
	PricingModeFixed  PricingMode = "fixed"
	PricingModeUsage  PricingMode = "usage"
	PricingModeTiered PricingMode = "tiered"
)

type BillableItemStatus string

const (
	BillableItemStatusActive   BillableItemStatus = "active"
	BillableItemStatusInactive BillableItemStatus = "inactive"
)

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
