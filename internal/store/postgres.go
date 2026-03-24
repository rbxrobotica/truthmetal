package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ldamasio/truthmetal/internal/model"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) CreateParameter(ctx context.Context, p *model.Parameter) error {
	meta, err := json.Marshal(p.Metadata)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO parameters
			(id, key, value, value_type, namespace, source, source_ref, version,
			 previous_version_id, state, metadata, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		p.ID, p.Key, p.Value, string(p.ValueType), p.Namespace,
		p.Source, p.SourceRef, p.Version, nilIfEmpty(p.PreviousVersionID),
		string(p.State), meta, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *PostgresStore) GetParameter(ctx context.Context, namespace, key string) (*model.Parameter, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, key, value, value_type, namespace, source, source_ref, version,
		       COALESCE(previous_version_id,''), state, metadata, created_at, updated_at
		FROM parameters
		WHERE namespace=$1 AND key=$2 AND state='CANONICAL'
		ORDER BY version DESC
		LIMIT 1`,
		namespace, key,
	)
	return scanParameter(row)
}

func (s *PostgresStore) GetParameterByID(ctx context.Context, id string) (*model.Parameter, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, key, value, value_type, namespace, source, source_ref, version,
		       COALESCE(previous_version_id,''), state, metadata, created_at, updated_at
		FROM parameters WHERE id=$1`,
		id,
	)
	return scanParameter(row)
}

func (s *PostgresStore) UpdateParameterState(ctx context.Context, id string, state model.ParameterState) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE parameters SET state=$1, updated_at=NOW() WHERE id=$2`,
		string(state), id,
	)
	return err
}

func (s *PostgresStore) ListParameterVersions(ctx context.Context, namespace, key string) ([]*model.Parameter, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, key, value, value_type, namespace, source, source_ref, version,
		       COALESCE(previous_version_id,''), state, metadata, created_at, updated_at
		FROM parameters
		WHERE namespace=$1 AND key=$2
		ORDER BY version ASC`,
		namespace, key,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var params []*model.Parameter
	for rows.Next() {
		p, err := scanParameterFromRows(rows)
		if err != nil {
			return nil, err
		}
		params = append(params, p)
	}
	return params, rows.Err()
}

func (s *PostgresStore) AppendEvent(ctx context.Context, e *model.LedgerEvent) error {
	ctx2, err := json.Marshal(e.Context)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO ledger_events
			(id, parameter_id, event_type, from_state, to_state, actor, reason, context, occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		e.ID, e.ParameterID, string(e.EventType),
		string(e.FromState), string(e.ToState),
		e.Actor, e.Reason, ctx2, e.OccurredAt,
	)
	return err
}

func (s *PostgresStore) QueryAudit(ctx context.Context, namespace, key string, limit, offset int64) ([]*model.LedgerEvent, int64, error) {
	var total int64
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ledger_events le
		JOIN parameters p ON p.id = le.parameter_id
		WHERE p.namespace=$1 AND p.key=$2`,
		namespace, key,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT le.id, le.parameter_id, le.event_type, le.from_state, le.to_state,
		       le.actor, le.reason, le.context, le.occurred_at
		FROM ledger_events le
		JOIN parameters p ON p.id = le.parameter_id
		WHERE p.namespace=$1 AND p.key=$2
		ORDER BY le.occurred_at DESC
		LIMIT $3 OFFSET $4`,
		namespace, key, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []*model.LedgerEvent
	for rows.Next() {
		e := &model.LedgerEvent{}
		var ctx2 []byte
		err := rows.Scan(
			&e.ID, &e.ParameterID, &e.EventType, &e.FromState, &e.ToState,
			&e.Actor, &e.Reason, &ctx2, &e.OccurredAt,
		)
		if err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(ctx2, &e.Context); err != nil {
			e.Context = map[string]string{}
		}
		events = append(events, e)
	}
	return events, total, rows.Err()
}

func scanParameter(row pgx.Row) (*model.Parameter, error) {
	p := &model.Parameter{}
	var meta []byte
	err := row.Scan(
		&p.ID, &p.Key, &p.Value, &p.ValueType, &p.Namespace,
		&p.Source, &p.SourceRef, &p.Version, &p.PreviousVersionID,
		&p.State, &meta, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &ErrNotFound{Resource: "parameter"}
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(meta, &p.Metadata); err != nil {
		p.Metadata = map[string]string{}
	}
	return p, nil
}

func scanParameterFromRows(rows pgx.Rows) (*model.Parameter, error) {
	p := &model.Parameter{}
	var meta []byte
	err := rows.Scan(
		&p.ID, &p.Key, &p.Value, &p.ValueType, &p.Namespace,
		&p.Source, &p.SourceRef, &p.Version, &p.PreviousVersionID,
		&p.State, &meta, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(meta, &p.Metadata); err != nil {
		p.Metadata = map[string]string{}
	}
	return p, nil
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
