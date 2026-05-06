// Package event defines application event contracts used by outbox and Kafka
// delivery adapters.
package event

// Kafka topic names for Sprint 7 v1 integrations.
const (
	TopicUsageRecordsV1               = "usage.records.v1"
	TopicBillingInvoicesV1            = "billing.invoices.v1"
	TopicReconciliationRunRequestedV1 = "reconciliation.run.requested.v1"
	TopicReconciliationRunCompletedV1 = "reconciliation.run.completed.v1"
	TopicLeakageCaseCreatedV1         = "leakage.case.created.v1"
)

// Event type names carried inside the JSON envelope.
const (
	TypeUsageRecordReceived           = "usage.record.received.v1"
	TypeBillingInvoiceReceived        = "billing.invoice.received.v1"
	TypeReconciliationRunRequested    = "reconciliation.run.requested.v1"
	TypeReconciliationRunCompleted    = "reconciliation.run.completed.v1"
	TypeLeakageCaseCreated            = "leakage.case.created.v1"
	TypeUsageRecordsIngested          = "usage.records.ingested.v1"
	TypeBillingInvoiceIngested        = "billing.invoice.ingested.v1"
	TypeReconciliationRunCompletedOut = "reconciliation.run.completed.outbox.v1"
	TypeLeakageCaseCreatedOut         = "leakage.case.created.outbox.v1"
)
