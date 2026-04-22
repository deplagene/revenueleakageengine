package revenue

import (
	"errors"
	"fmt"

	revenuedomain "github.com/deplagene/revenueleakageengine/internal/domain/revenue"
	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
)

var (
	// ErrPricingRequired reports that expected revenue cannot be calculated
	// without a pricing model.
	ErrPricingRequired = errors.New("pricing is required")
	// ErrIncludedQuantityInvalid reports a negative included usage quantity.
	ErrIncludedQuantityInvalid = errors.New("included quantity cannot be negative")
	// ErrOveragePriceInvalid reports a negative usage overage price.
	ErrOveragePriceInvalid = errors.New("overage unit price cannot be negative")
	// ErrUsageRecordMismatch reports a usage record that does not belong to the
	// requested revenue calculation context.
	ErrUsageRecordMismatch = errors.New("usage record does not match calculation context")
	// ErrUsageRecordOutOfPeriod reports a usage record outside the billing period.
	ErrUsageRecordOutOfPeriod = errors.New("usage record is outside billing period")
	// ErrUsageQuantityInvalid reports a negative usage quantity.
	ErrUsageQuantityInvalid = errors.New("usage quantity cannot be negative")
)

// Calculator computes expected revenue entries from deterministic inputs.
type Calculator interface {
	CalculateExpectedRevenue(cmd CalculateExpectedRevenueCommand) (revenuedomain.ExpectedRevenueEntry, error)
}

type fixedUsageCalculator struct{}

// NewFixedUsageCalculator creates the MVP calculator for fixed recurring fee
// plus simple usage overage.
func NewFixedUsageCalculator() Calculator {
	return fixedUsageCalculator{}
}

// CalculateExpectedRevenue calculates one expected revenue ledger entry for
// fixed fee plus overage usage pricing.
func (c fixedUsageCalculator) CalculateExpectedRevenue(
	cmd CalculateExpectedRevenueCommand,
) (revenuedomain.ExpectedRevenueEntry, error) {
	if err := cmd.Validate(); err != nil {
		return revenuedomain.ExpectedRevenueEntry{}, err
	}

	usageQuantity, err := sumUsageQuantity(cmd)
	if err != nil {
		return revenuedomain.ExpectedRevenueEntry{}, err
	}

	billableQuantity := usageQuantity - cmd.Pricing.IncludedQuantity
	if billableQuantity < 0 {
		billableQuantity = 0
	}

	overageAmount := valueobject.Money{
		Currency:   cmd.Pricing.OverageUnitPrice.Currency,
		MinorUnits: billableQuantity * cmd.Pricing.OverageUnitPrice.MinorUnits,
	}

	expectedAmount, err := cmd.Pricing.BaseFee.Add(overageAmount)
	if err != nil {
		return revenuedomain.ExpectedRevenueEntry{}, fmt.Errorf("calculate expected amount: %w", err)
	}

	return revenuedomain.ExpectedRevenueEntry{
		ID:             cmd.EntryID,
		TenantID:       cmd.TenantID,
		CustomerID:     cmd.CustomerID,
		ContractID:     cmd.ContractID,
		BillableItemID: cmd.BillableItemID,
		Period:         cmd.Period,
		ExpectedAmount: expectedAmount,
		CalculationBasis: map[string]any{
			"model":                          "fixed_usage",
			"base_fee_minor_units":           cmd.Pricing.BaseFee.MinorUnits,
			"included_quantity":              cmd.Pricing.IncludedQuantity,
			"usage_quantity":                 usageQuantity,
			"billable_quantity":              billableQuantity,
			"overage_unit_price_minor_units": cmd.Pricing.OverageUnitPrice.MinorUnits,
			"overage_amount_minor_units":     overageAmount.MinorUnits,
			"currency":                       expectedAmount.Currency,
			"unit":                           cmd.Pricing.Unit,
		},
		CalculatedAt: cmd.CalculatedAt.UTC(),
		Version:      cmd.Version,
		TraceID:      cmd.TraceID,
	}, nil
}

func sumUsageQuantity(cmd CalculateExpectedRevenueCommand) (int64, error) {
	var total int64

	for _, record := range cmd.UsageRecords {
		if record.Quantity < 0 {
			return 0, fmt.Errorf("%w: usage_record_id=%s", ErrUsageQuantityInvalid, record.ID)
		}

		if !cmd.Period.Contains(record.UsageTime) {
			return 0, fmt.Errorf("%w: usage_record_id=%s", ErrUsageRecordOutOfPeriod, record.ID)
		}

		matchesContext := record.TenantID == cmd.TenantID &&
			record.CustomerID == cmd.CustomerID &&
			record.ContractID == cmd.ContractID &&
			record.BillableItemID == cmd.BillableItemID &&
			record.Unit == cmd.Pricing.Unit
		if !matchesContext {
			return 0, fmt.Errorf("%w: usage_record_id=%s", ErrUsageRecordMismatch, record.ID)
		}

		total += record.Quantity
	}

	return total, nil
}
