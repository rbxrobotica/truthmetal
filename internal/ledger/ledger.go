package ledger

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ldamasio/truthmetal/internal/cache"
	"github.com/ldamasio/truthmetal/internal/consensus"
	"github.com/ldamasio/truthmetal/internal/model"
	"github.com/ldamasio/truthmetal/internal/store"
)

// Service orchestrates the full parameter lifecycle.
type Service struct {
	store     store.Store
	cache     *cache.Cache
	consensus consensus.Protocol
	events    chan *model.LedgerEvent
}

func NewService(s store.Store, c *cache.Cache, p consensus.Protocol) *Service {
	return &Service{
		store:     s,
		cache:     c,
		consensus: p,
		events:    make(chan *model.LedgerEvent, 256),
	}
}

// Submit proposes a new parameter value for canonicalization.
func (svc *Service) Submit(ctx context.Context, req *model.SubmitRequest) (*model.Parameter, error) {
	// Determine version number
	existing, err := svc.store.GetParameter(ctx, req.Namespace, req.Key)
	var version int64 = 1
	var prevID string
	if err == nil {
		version = existing.Version + 1
		prevID = existing.ID
	} else if !errors.As(err, &store.ErrNotFound{}) {
		if _, ok := err.(*store.ErrNotFound); !ok {
			return nil, err
		}
	}

	now := time.Now().UTC()
	p := &model.Parameter{
		ID:                uuid.NewString(),
		Key:               req.Key,
		Value:             req.Value,
		ValueType:         req.ValueType,
		Namespace:         req.Namespace,
		Source:            req.Source,
		SourceRef:         req.SourceRef,
		Version:           version,
		PreviousVersionID: prevID,
		State:             model.StateSubmitted,
		Metadata:          req.Metadata,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if p.Metadata == nil {
		p.Metadata = map[string]string{}
	}

	if err := svc.store.CreateParameter(ctx, p); err != nil {
		return nil, err
	}

	// Record submission event
	if err := svc.store.AppendEvent(ctx, &model.LedgerEvent{
		ID:          uuid.NewString(),
		ParameterID: p.ID,
		EventType:   model.EventSubmitted,
		FromState:   "",
		ToState:     model.StateSubmitted,
		Actor:       req.Actor,
		Reason:      "new submission",
		Context:     map[string]string{"source": req.Source, "source_ref": req.SourceRef},
		OccurredAt:  now,
	}); err != nil {
		return nil, err
	}

	// If there was a previous canonical version, mark it as superseded
	if prevID != "" {
		_ = svc.store.UpdateParameterState(ctx, prevID, model.StateRevoked)
		_ = svc.store.AppendEvent(ctx, &model.LedgerEvent{
			ID:          uuid.NewString(),
			ParameterID: prevID,
			EventType:   model.EventSuperseded,
			FromState:   model.StateCanonical,
			ToState:     model.StateRevoked,
			Actor:       "system",
			Reason:      "superseded by version " + p.ID,
			Context:     map[string]string{"new_version_id": p.ID},
			OccurredAt:  now,
		})
		_ = svc.cache.InvalidateCanonical(ctx, req.Namespace, req.Key)
	}

	// Run consensus synchronously (SimpleQuorum for MVP)
	finalState, err := svc.consensus.Evaluate(ctx, p)
	if err != nil {
		return nil, err
	}
	p.State = finalState

	// Refresh from store to get updated timestamps
	fresh, err := svc.store.GetParameterByID(ctx, p.ID)
	if err != nil {
		return p, nil
	}

	if fresh.State == model.StateCanonical {
		_ = svc.cache.SetCanonical(ctx, fresh)
	}

	// Broadcast to Watch subscribers
	select {
	case svc.events <- &model.LedgerEvent{
		ParameterID: fresh.ID,
		EventType:   model.EventPromoted,
		ToState:     fresh.State,
		OccurredAt:  time.Now().UTC(),
	}:
	default:
	}

	return fresh, nil
}

// Get returns the current canonical parameter, using the cache when available.
func (svc *Service) Get(ctx context.Context, namespace, key string) (*model.Parameter, error) {
	if p, err := svc.cache.GetCanonical(ctx, namespace, key); err == nil {
		return p, nil
	}
	p, err := svc.store.GetParameter(ctx, namespace, key)
	if err != nil {
		return nil, err
	}
	_ = svc.cache.SetCanonical(ctx, p)
	return p, nil
}

// GetByID returns a specific version by UUID.
func (svc *Service) GetByID(ctx context.Context, id string) (*model.Parameter, error) {
	return svc.store.GetParameterByID(ctx, id)
}

// Revoke invalidates the canonical value.
func (svc *Service) Revoke(ctx context.Context, namespace, key, actor, reason string) (*model.Parameter, error) {
	p, err := svc.store.GetParameter(ctx, namespace, key)
	if err != nil {
		return nil, err
	}
	if err := svc.store.UpdateParameterState(ctx, p.ID, model.StateRevoked); err != nil {
		return nil, err
	}
	if err := svc.store.AppendEvent(ctx, &model.LedgerEvent{
		ID:          uuid.NewString(),
		ParameterID: p.ID,
		EventType:   model.EventRevoked,
		FromState:   model.StateCanonical,
		ToState:     model.StateRevoked,
		Actor:       actor,
		Reason:      reason,
		Context:     map[string]string{},
		OccurredAt:  time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	_ = svc.cache.InvalidateCanonical(ctx, namespace, key)
	p.State = model.StateRevoked
	return p, nil
}

// QueryAudit returns the ledger history for a parameter.
func (svc *Service) QueryAudit(ctx context.Context, namespace, key string, limit, offset int64) ([]*model.LedgerEvent, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	return svc.store.QueryAudit(ctx, namespace, key, limit, offset)
}

// Events returns the channel of live ledger events for Watch subscribers.
func (svc *Service) Events() <-chan *model.LedgerEvent {
	return svc.events
}
