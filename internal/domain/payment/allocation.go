package payment

import (
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/valueobject"
	"github.com/google/uuid"
)

// Allocation links a payment to a specific invoice amount.
type Allocation struct {
	ID              uuid.UUID
	PaymentID       uuid.UUID
	InvoiceID       uuid.UUID
	AllocatedAmount valueobject.Money
	AllocatedAt     time.Time
}
