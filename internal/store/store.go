package store

import (
	"context"

	"github.com/ldamasio/truthmetal/internal/model"
)

// Store is the persistence interface for parameters and ledger events.
// All methods operate on a single logical unit; callers must not
// assume any cross-method transactionality unless explicitly noted.
type Store interface {
	// Parameter operations

	// CreateParameter persists a new parameter in SUBMITTED state.
	CreateParameter(ctx context.Context, p *model.Parameter) error

	// GetParameter returns the current canonical parameter for the given namespace/key.
	// Returns ErrNotFound if no canonical version exists.
	GetParameter(ctx context.Context, namespace, key string) (*model.Parameter, error)

	// GetParameterByID returns a specific parameter version by its UUID.
	GetParameterByID(ctx context.Context, id string) (*model.Parameter, error)

	// UpdateParameterState transitions a parameter to a new state.
	// This is the only mutating operation on parameters.
	UpdateParameterState(ctx context.Context, id string, state model.ParameterState) error

	// ListParameterVersions returns all versions of a key in chronological order.
	ListParameterVersions(ctx context.Context, namespace, key string) ([]*model.Parameter, error)

	// Ledger operations

	// AppendEvent records a state transition to the immutable ledger.
	AppendEvent(ctx context.Context, e *model.LedgerEvent) error

	// QueryAudit returns ledger events for a parameter, newest first.
	QueryAudit(ctx context.Context, namespace, key string, limit, offset int64) ([]*model.LedgerEvent, int64, error)
}

// ErrNotFound is returned when a requested resource does not exist.
type ErrNotFound struct {
	Resource string
}

func (e *ErrNotFound) Error() string {
	return e.Resource + " not found"
}
