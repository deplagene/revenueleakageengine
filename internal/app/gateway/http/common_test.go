package http

import (
	"context"
	"time"

	"github.com/deplagene/revenueleakageengine/internal/domain/billing"
	"github.com/deplagene/revenueleakageengine/internal/domain/contract"
	"github.com/google/uuid"
)

type noopContractQueries struct{}

func (n *noopContractQueries) GetContract(context.Context, uuid.UUID) (*contract.Contract, error) {
	return nil, nil
}

func (n *noopContractQueries) ListContractsByCustomer(context.Context, uuid.UUID, uuid.UUID) ([]*contract.Contract, error) {
	return nil, nil
}

func (n *noopContractQueries) GetEffectiveTerms(context.Context, uuid.UUID, time.Time) ([]*contract.Term, error) {
	return nil, nil
}

type noopContractCommands struct{}

func (n *noopContractCommands) UpsertContract(context.Context, *contract.Contract) error { return nil }
func (n *noopContractCommands) UpsertBillableItem(context.Context, *contract.BillableItem) error {
	return nil
}
func (n *noopContractCommands) UpsertTerm(context.Context, *contract.Term) error { return nil }

type noopIngestionCommands struct{}

func (n *noopIngestionCommands) IngestUsageRecords(ctx context.Context, records []billing.UsageRecord) error {
	return nil
}

func (n *noopIngestionCommands) IngestInvoice(
	ctx context.Context,
	invoice billing.Invoice,
	lines []billing.InvoiceLine,
) error {
	return nil
}
