package consensus

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ldamasio/truthmetal/internal/model"
	"github.com/ldamasio/truthmetal/internal/store"
)

// Protocol defines the consensus interface.
// MVP uses SimpleQuorum (single-node, immediate promotion).
// v0.5 will add a distributed Raft-based implementation.
type Protocol interface {
	// Evaluate runs consensus for the given parameter and promotes or rejects it.
	Evaluate(ctx context.Context, p *model.Parameter) (model.ParameterState, error)
}

// SimpleQuorum is the MVP single-node implementation.
// It validates the value type and immediately promotes to CANONICAL.
type SimpleQuorum struct {
	store store.Store
}

func NewSimpleQuorum(s store.Store) *SimpleQuorum {
	return &SimpleQuorum{store: s}
}

func (q *SimpleQuorum) Evaluate(ctx context.Context, p *model.Parameter) (model.ParameterState, error) {
	// Transition to VALIDATING
	if err := q.store.UpdateParameterState(ctx, p.ID, model.StateValidating); err != nil {
		return model.StateRejected, err
	}
	if err := q.appendEvent(ctx, p, model.EventValidated, model.StateSubmitted, model.StateValidating, "system", ""); err != nil {
		return model.StateRejected, err
	}

	// Validate value type
	if err := validateValueType(p.Value, p.ValueType); err != nil {
		_ = q.store.UpdateParameterState(ctx, p.ID, model.StateRejected)
		_ = q.appendEvent(ctx, p, model.EventRejected, model.StateValidating, model.StateRejected, "system", err.Error())
		return model.StateRejected, err
	}

	// SimpleQuorum: single node → immediate consensus
	if err := q.store.UpdateParameterState(ctx, p.ID, model.StateConsensusPending); err != nil {
		return model.StateRejected, err
	}
	if err := q.appendEvent(ctx, p, model.EventConsensusReached, model.StateValidating, model.StateConsensusPending, "system", "simple-quorum"); err != nil {
		return model.StateRejected, err
	}

	if err := q.store.UpdateParameterState(ctx, p.ID, model.StateCanonical); err != nil {
		return model.StateRejected, err
	}
	if err := q.appendEvent(ctx, p, model.EventPromoted, model.StateConsensusPending, model.StateCanonical, "system", ""); err != nil {
		return model.StateRejected, err
	}

	return model.StateCanonical, nil
}

func (q *SimpleQuorum) appendEvent(
	ctx context.Context,
	p *model.Parameter,
	eventType model.LedgerEventType,
	from, to model.ParameterState,
	actor, reason string,
) error {
	return q.store.AppendEvent(ctx, &model.LedgerEvent{
		ID:          uuid.NewString(),
		ParameterID: p.ID,
		EventType:   eventType,
		FromState:   from,
		ToState:     to,
		Actor:       actor,
		Reason:      reason,
		Context:     map[string]string{"protocol": "simple-quorum"},
		OccurredAt:  time.Now().UTC(),
	})
}
