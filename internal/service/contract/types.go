package contract

import "errors"

var (
	// ErrStoreRequired reports that contract use cases were built without a
	// persistence boundary.
	ErrStoreRequired = errors.New("contract store is required")
	// ErrContractRequired reports that a contract write was requested without a
	// contract payload.
	ErrContractRequired = errors.New("contract is required")
	// ErrBillableItemRequired reports that a billable item write was requested
	// without a billable item payload.
	ErrBillableItemRequired = errors.New("billable item is required")
	// ErrTermRequired reports that a contract term write was requested without a
	// term payload.
	ErrTermRequired = errors.New("term is required")
	// ErrContractNotFound reports that a contract lookup missed.
	ErrContractNotFound = errors.New("contract not found")
	// ErrBillableItemNotFound reports that a billable item lookup missed.
	ErrBillableItemNotFound = errors.New("billable item not found")
)
