package api

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/ldamasio/truthmetal/internal/ledger"
	"github.com/ldamasio/truthmetal/internal/model"
	"github.com/ldamasio/truthmetal/internal/store"
	truthmetalv1 "github.com/ldamasio/truthmetal/gen/truthmetal/v1"
	"github.com/ldamasio/truthmetal/gen/truthmetal/v1/truthmetalv1connect"
)

// Server implements the TruthMetalService via connect-go.
type Server struct {
	svc *ledger.Service
}

var _ truthmetalv1connect.TruthMetalServiceHandler = (*Server)(nil)

func NewServer(svc *ledger.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) Submit(ctx context.Context, req *connect.Request[truthmetalv1.SubmitRequest]) (*connect.Response[truthmetalv1.SubmitResponse], error) {
	r := req.Msg
	if r.Key == "" || r.Namespace == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key and namespace are required"))
	}
	if r.Value == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("value is required"))
	}

	vt := model.ValueType(r.ValueType)
	if vt == "" {
		vt = model.TypeString
	}

	p, err := s.svc.Submit(ctx, &model.SubmitRequest{
		Key:       r.Key,
		Value:     r.Value,
		ValueType: vt,
		Namespace: r.Namespace,
		Source:    r.Source,
		SourceRef: r.SourceRef,
		Metadata:  r.Metadata,
		Actor:     r.Actor,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&truthmetalv1.SubmitResponse{Parameter: toProto(p)}), nil
}

func (s *Server) Get(ctx context.Context, req *connect.Request[truthmetalv1.GetRequest]) (*connect.Response[truthmetalv1.GetResponse], error) {
	r := req.Msg
	if r.Namespace == "" || r.Key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("namespace and key are required"))
	}

	p, err := s.svc.Get(ctx, r.Namespace, r.Key)
	if err != nil {
		if isNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&truthmetalv1.GetResponse{Parameter: toProto(p)}), nil
}

func (s *Server) GetVersion(ctx context.Context, req *connect.Request[truthmetalv1.GetVersionRequest]) (*connect.Response[truthmetalv1.GetVersionResponse], error) {
	r := req.Msg
	if r.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	p, err := s.svc.GetByID(ctx, r.Id)
	if err != nil {
		if isNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&truthmetalv1.GetVersionResponse{Parameter: toProto(p)}), nil
}

func (s *Server) Revoke(ctx context.Context, req *connect.Request[truthmetalv1.RevokeRequest]) (*connect.Response[truthmetalv1.RevokeResponse], error) {
	r := req.Msg
	if r.Namespace == "" || r.Key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("namespace and key are required"))
	}

	p, err := s.svc.Revoke(ctx, r.Namespace, r.Key, r.Actor, r.Reason)
	if err != nil {
		if isNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&truthmetalv1.RevokeResponse{Parameter: toProto(p)}), nil
}

func (s *Server) QueryAudit(ctx context.Context, req *connect.Request[truthmetalv1.QueryAuditRequest]) (*connect.Response[truthmetalv1.QueryAuditResponse], error) {
	r := req.Msg
	if r.Namespace == "" || r.Key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("namespace and key are required"))
	}

	events, total, err := s.svc.QueryAudit(ctx, r.Namespace, r.Key, r.Limit, r.Offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	pbEvents := make([]*truthmetalv1.LedgerEvent, len(events))
	for i, e := range events {
		pbEvents[i] = eventToProto(e)
	}

	return connect.NewResponse(&truthmetalv1.QueryAuditResponse{Events: pbEvents, Total: total}), nil
}

func (s *Server) Watch(ctx context.Context, req *connect.Request[truthmetalv1.WatchRequest], stream *connect.ServerStream[truthmetalv1.LedgerEvent]) error {
	ch := s.svc.Events()
	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(eventToProto(e)); err != nil {
				return err
			}
		}
	}
}

func toProto(p *model.Parameter) *truthmetalv1.Parameter {
	return &truthmetalv1.Parameter{
		Id:                p.ID,
		Key:               p.Key,
		Value:             p.Value,
		ValueType:         string(p.ValueType),
		Namespace:         p.Namespace,
		Source:            p.Source,
		SourceRef:         p.SourceRef,
		Version:           p.Version,
		PreviousVersionId: p.PreviousVersionID,
		State:             stateToProto(p.State),
		Metadata:          p.Metadata,
		CreatedAt:         timestamppb.New(p.CreatedAt),
		UpdatedAt:         timestamppb.New(p.UpdatedAt),
	}
}

func eventToProto(e *model.LedgerEvent) *truthmetalv1.LedgerEvent {
	t := e.OccurredAt
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return &truthmetalv1.LedgerEvent{
		Id:          e.ID,
		ParameterId: e.ParameterID,
		EventType:   string(e.EventType),
		FromState:   string(e.FromState),
		ToState:     string(e.ToState),
		Actor:       e.Actor,
		Reason:      e.Reason,
		Context:     e.Context,
		OccurredAt:  timestamppb.New(t),
	}
}

func stateToProto(s model.ParameterState) truthmetalv1.ParameterState {
	switch s {
	case model.StateSubmitted:
		return truthmetalv1.ParameterState_PARAMETER_STATE_SUBMITTED
	case model.StateValidating:
		return truthmetalv1.ParameterState_PARAMETER_STATE_VALIDATING
	case model.StateConsensusPending:
		return truthmetalv1.ParameterState_PARAMETER_STATE_CONSENSUS_PENDING
	case model.StateCanonical:
		return truthmetalv1.ParameterState_PARAMETER_STATE_CANONICAL
	case model.StateRejected:
		return truthmetalv1.ParameterState_PARAMETER_STATE_REJECTED
	case model.StateRevoked:
		return truthmetalv1.ParameterState_PARAMETER_STATE_REVOKED
	default:
		return truthmetalv1.ParameterState_PARAMETER_STATE_UNSPECIFIED
	}
}

func isNotFound(err error) bool {
	var nf *store.ErrNotFound
	return errors.As(err, &nf)
}
