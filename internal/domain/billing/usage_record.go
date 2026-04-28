package billing

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrUsageRecordIDRequired reports that a normalized usage record is missing
	// its internal id.
	ErrUsageRecordIDRequired = errors.New("usage record id is required")
	// ErrTenantRequired reports that a billing fact is missing tenant scope.
	ErrTenantRequired = errors.New("tenant id is required")
	// ErrCustomerRequired reports that a billing fact is missing customer scope.
	ErrCustomerRequired = errors.New("customer id is required")
	// ErrContractRequired reports that a billing fact is missing contract scope.
	ErrContractRequired = errors.New("contract id is required")
	// ErrBillableItemRequired reports that a usage fact is missing its billable
	// item.
	ErrBillableItemRequired = errors.New("billable item id is required")
	// ErrExternalIDRequired reports that an ingested fact lacks the external id
	// used for idempotency.
	ErrExternalIDRequired = errors.New("external id is required")
	// ErrSourceSystemRequired reports that an ingested fact lacks its source
	// system idempotency namespace.
	ErrSourceSystemRequired = errors.New("source system is required")
	// ErrUsageTimeRequired reports that a usage fact lacks its event timestamp.
	ErrUsageTimeRequired = errors.New("usage time is required")
	// ErrUsageQuantityInvalid reports a negative usage quantity.
	ErrUsageQuantityInvalid = errors.New("usage quantity cannot be negative")
	// ErrUsageUnitRequired reports that a usage fact lacks a metered unit.
	ErrUsageUnitRequired = errors.New("usage unit is required")
)

// UsageRecord captures billable consumption imported from an external metering
// or product system.
type UsageRecord struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	CustomerID     uuid.UUID
	ContractID     uuid.UUID
	BillableItemID uuid.UUID
	ExternalID     string
	UsageTime      time.Time
	Quantity       int64
	Unit           string
	SourceSystem   string
	TraceID        string
	Metadata       map[string]any
}

// Normalize returns a copy with stable string fields, UTC timestamps, and
// non-nil metadata.
func (u UsageRecord) Normalize() UsageRecord {
	u.ExternalID = strings.TrimSpace(u.ExternalID)
	u.Unit = strings.TrimSpace(u.Unit)
	u.SourceSystem = strings.TrimSpace(u.SourceSystem)
	u.TraceID = strings.TrimSpace(u.TraceID)

	if !u.UsageTime.IsZero() {
		u.UsageTime = u.UsageTime.UTC()
	}

	if u.Metadata == nil {
		u.Metadata = map[string]any{}
	}

	return u
}

// Validate checks that the usage record is safe to persist as an idempotent
// billing fact.
func (u UsageRecord) Validate() error {
	if u.ID == uuid.Nil {
		return ErrUsageRecordIDRequired
	}

	if u.TenantID == uuid.Nil {
		return ErrTenantRequired
	}

	if u.CustomerID == uuid.Nil {
		return ErrCustomerRequired
	}

	if u.ContractID == uuid.Nil {
		return ErrContractRequired
	}

	if u.BillableItemID == uuid.Nil {
		return ErrBillableItemRequired
	}

	if strings.TrimSpace(u.ExternalID) == "" {
		return ErrExternalIDRequired
	}

	if strings.TrimSpace(u.SourceSystem) == "" {
		return ErrSourceSystemRequired
	}

	if u.UsageTime.IsZero() {
		return ErrUsageTimeRequired
	}

	if u.Quantity < 0 {
		return ErrUsageQuantityInvalid
	}

	if strings.TrimSpace(u.Unit) == "" {
		return ErrUsageUnitRequired
	}

	return nil
}
