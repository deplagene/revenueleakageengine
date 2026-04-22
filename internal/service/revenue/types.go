package revenue

import (
	"errors"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

var (
	// ErrEntryIDRequired reports that the expected revenue entry id is missing.
	ErrEntryIDRequired = errors.New("entry id is required")
	// ErrTenantRequired reports that a revenue calculation command has no tenant.
	ErrTenantRequired = errors.New("tenant id is required")
	// ErrCustomerRequired reports that a revenue calculation command has no customer.
	ErrCustomerRequired = errors.New("customer id is required")
	// ErrContractRequired reports that a revenue calculation command has no contract.
	ErrContractRequired = errors.New("contract id is required")
	// ErrBillableItemRequired reports that a revenue calculation command has no billable item.
	ErrBillableItemRequired = errors.New("billable item id is required")
	// ErrCalculatedAtRequired reports that a revenue calculation has no timestamp.
	ErrCalculatedAtRequired = errors.New("calculated at is required")
	// ErrUsageUnitRequired reports that a usage pricing model has no usage unit.
	ErrUsageUnitRequired = errors.New("usage unit is required")
	// ErrRecognizedAtRequired reports that actual revenue has no recognition
	// timestamp.
	ErrRecognizedAtRequired = errors.New("recognized at is required")
)

// FixedUsagePricing defines the MVP pricing model: a fixed recurring fee plus
// usage charged after an included quantity.
type FixedUsagePricing struct {
	BaseFee          valueobject.Money
	IncludedQuantity int64
	OverageUnitPrice valueobject.Money
	Unit             string
}

// Validate checks that fixed plus usage pricing is safe to use for money
// calculations.
func (p FixedUsagePricing) Validate() error {
	if p.BaseFee.Currency == "" || p.OverageUnitPrice.Currency == "" {
		return ErrPricingRequired
	}

	if !p.BaseFee.SameCurrency(p.OverageUnitPrice) {
		return valueobject.ErrCurrencyMismatch
	}

	if p.IncludedQuantity < 0 {
		return ErrIncludedQuantityInvalid
	}

	if p.OverageUnitPrice.MinorUnits < 0 {
		return ErrOveragePriceInvalid
	}

	if p.Unit == "" {
		return ErrUsageUnitRequired
	}

	return nil
}

// CalculateExpectedRevenueCommand contains all deterministic inputs required
// to produce one expected revenue ledger entry.
type CalculateExpectedRevenueCommand struct {
	EntryID        uuid.UUID
	TenantID       uuid.UUID
	CustomerID     uuid.UUID
	ContractID     uuid.UUID
	BillableItemID uuid.UUID
	Period         valueobject.BillingPeriod
	Pricing        FixedUsagePricing
	UsageRecords   []billing.UsageRecord
	CalculatedAt   time.Time
	Version        int
	TraceID        string
}

// Validate checks that an expected revenue calculation has enough context to be
// traceable and deterministic.
func (c CalculateExpectedRevenueCommand) Validate() error {
	if c.EntryID == uuid.Nil {
		return ErrEntryIDRequired
	}

	if c.TenantID == uuid.Nil {
		return ErrTenantRequired
	}

	if c.CustomerID == uuid.Nil {
		return ErrCustomerRequired
	}

	if c.ContractID == uuid.Nil {
		return ErrContractRequired
	}

	if c.BillableItemID == uuid.Nil {
		return ErrBillableItemRequired
	}

	if err := c.Period.Validate(); err != nil {
		return err
	}

	if err := c.Pricing.Validate(); err != nil {
		return err
	}

	if c.CalculatedAt.IsZero() {
		return ErrCalculatedAtRequired
	}

	return nil
}

// BuildActualRevenueCommand contains normalized invoice facts required to build
// invoice-based actual revenue entries.
type BuildActualRevenueCommand struct {
	TenantID     uuid.UUID
	CustomerID   uuid.UUID
	ContractID   uuid.UUID
	Period       valueobject.BillingPeriod
	Invoices     []billing.Invoice
	InvoiceLines []billing.InvoiceLine
	RecognizedAt time.Time
	TraceID      string
}

// Validate checks that invoice-based actual revenue construction has enough
// context to produce traceable ledger entries.
func (c BuildActualRevenueCommand) Validate() error {
	if c.TenantID == uuid.Nil {
		return ErrTenantRequired
	}

	if c.CustomerID == uuid.Nil {
		return ErrCustomerRequired
	}

	if c.ContractID == uuid.Nil {
		return ErrContractRequired
	}

	if err := c.Period.Validate(); err != nil {
		return err
	}

	if c.RecognizedAt.IsZero() {
		return ErrRecognizedAtRequired
	}

	return nil
}
