package document

import (
	"errors"

	"github.com/google/uuid"
)

const (
	DefaultListLimit = 50
	MaxListLimit     = 100
)

var (
	ErrStoreRequired = errors.New("document store is required")
	ErrNotFound      = errors.New("document not found")
	ErrLimitInvalid  = errors.New("document list limit is invalid")
)

type ListDocumentsCommand struct {
	TenantID uuid.UUID
	Limit    int
	Offset   int
}

func (c ListDocumentsCommand) Normalize() ListDocumentsCommand {
	if c.Limit == 0 {
		c.Limit = DefaultListLimit
	}
	return c
}

func (c ListDocumentsCommand) Validate() error {
	if c.TenantID == uuid.Nil {
		return ErrTenantRequired
	}
	if c.Limit < 0 || c.Limit > MaxListLimit {
		return ErrLimitInvalid
	}
	if c.Offset < 0 {
		return ErrLimitInvalid
	}
	return nil
}

type ListDraftsCommand struct {
	TenantID   uuid.UUID
	DocumentID uuid.UUID
	Limit      int
	Offset     int
}

func (c ListDraftsCommand) Normalize() ListDraftsCommand {
	if c.Limit == 0 {
		c.Limit = DefaultListLimit
	}
	return c
}

func (c ListDraftsCommand) Validate() error {
	if c.TenantID == uuid.Nil {
		return ErrTenantRequired
	}
	if c.DocumentID == uuid.Nil {
		return ErrDocumentRequired
	}
	if c.Limit < 0 || c.Limit > MaxListLimit {
		return ErrLimitInvalid
	}
	if c.Offset < 0 {
		return ErrLimitInvalid
	}
	return nil
}
