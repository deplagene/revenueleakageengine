package payment

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

type Allocation struct {
	ID              uuid.UUID
	PaymentID       uuid.UUID
	InvoiceID       uuid.UUID
	AllocatedAmount valueobject.Money
	AllocatedAt     time.Time
}
