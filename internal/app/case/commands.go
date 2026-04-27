package caseapp

import (
	"context"
	"errors"

	casework "github.com/deplagene/revenueleakageengine/internal/service/case"
)

// ErrCommandsServiceRequired reports that case commands were created without
// a case service dependency.
var ErrCommandsServiceRequired = errors.New("case service is required")

// UpdateCaseStatusCommand aliases the service-level lifecycle command at the
// application boundary.
type UpdateCaseStatusCommand = casework.UpdateCaseStatusCommand

// UpdateCaseStatusResult aliases the service-level lifecycle result at the
// application boundary.
type UpdateCaseStatusResult = casework.UpdateCaseStatusResult

// UpdateCaseAssigneeCommand aliases the service-level ownership command at the
// application boundary.
type UpdateCaseAssigneeCommand = casework.UpdateCaseAssigneeCommand

// UpdateCaseAssigneeResult aliases the service-level ownership result at the
// application boundary.
type UpdateCaseAssigneeResult = casework.UpdateCaseAssigneeResult

// Commands exposes case lifecycle use cases to transport layers.
type Commands struct {
	service *casework.Service
}

// NewCommands creates application-level case commands over a case service.
func NewCommands(service *casework.Service) (*Commands, error) {
	if service == nil {
		return nil, ErrCommandsServiceRequired
	}

	return &Commands{
		service: service,
	}, nil
}

// UpdateCaseStatus applies one lifecycle transition to a leakage case.
func (c *Commands) UpdateCaseStatus(
	ctx context.Context,
	cmd UpdateCaseStatusCommand,
) (UpdateCaseStatusResult, error) {
	return c.service.UpdateCaseStatus(ctx, cmd)
}

// UpdateCaseAssignee applies one ownership update to a leakage case.
func (c *Commands) UpdateCaseAssignee(
	ctx context.Context,
	cmd UpdateCaseAssigneeCommand,
) (UpdateCaseAssigneeResult, error) {
	return c.service.UpdateCaseAssignee(ctx, cmd)
}
