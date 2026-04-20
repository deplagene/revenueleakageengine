package billing

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

type AdjustmentType string

const (
	AdjustmentTypeRefund       AdjustmentType = "refund"
	AdjustmentTypeManualCredit AdjustmentType = "manual_credit"
	AdjustmentTypeWriteOff     AdjustmentType = "write_off"
	AdjustmentTypeCompensation AdjustmentType = "compensation"
	AdjustmentTypeBonus        AdjustmentType = "bonus"
	AdjustmentTypeCorrection   AdjustmentType = "correction"
)

type Adjustment struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	CustomerID uuid.UUID
	ContractID uuid.UUID
	Type       AdjustmentType
	Amount     valueobject.Money
	ReasonCode string
	CreatedAt  time.Time
	SourceRef  string
	Metadata   map[string]any
}
