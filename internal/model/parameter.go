package model

import "time"

// ParameterState is the lifecycle state of a parameter.
type ParameterState string

const (
	StateSubmitted       ParameterState = "SUBMITTED"
	StateValidating      ParameterState = "VALIDATING"
	StateConsensusPending ParameterState = "CONSENSUS_PENDING"
	StateCanonical       ParameterState = "CANONICAL"
	StateRejected        ParameterState = "REJECTED"
	StateRevoked         ParameterState = "REVOKED"
)

// ValueType constrains the allowed types for parameter values.
type ValueType string

const (
	TypeString    ValueType = "string"
	TypeNumber    ValueType = "number"
	TypeBool      ValueType = "bool"
	TypeJSON      ValueType = "json"
	TypeSemver    ValueType = "semver"
	TypeTimestamp ValueType = "timestamp"
)

// Parameter is an immutable, versioned, namespaced key-value fact.
// Once a parameter reaches CANONICAL state, it is never mutated;
// updates create a new version that supersedes the previous one.
type Parameter struct {
	ID                string            `db:"id"`
	Key               string            `db:"key"`
	Value             string            `db:"value"`
	ValueType         ValueType         `db:"value_type"`
	Namespace         string            `db:"namespace"`
	Source            string            `db:"source"`
	SourceRef         string            `db:"source_ref"`
	Version           int64             `db:"version"`
	PreviousVersionID string            `db:"previous_version_id"`
	State             ParameterState    `db:"state"`
	Metadata          map[string]string `db:"metadata"`
	CreatedAt         time.Time         `db:"created_at"`
	UpdatedAt         time.Time         `db:"updated_at"`
}

// LedgerEventType describes what happened in a state transition.
type LedgerEventType string

const (
	EventSubmitted        LedgerEventType = "SUBMITTED"
	EventValidated        LedgerEventType = "VALIDATED"
	EventConsensusReached LedgerEventType = "CONSENSUS_REACHED"
	EventPromoted         LedgerEventType = "PROMOTED"
	EventRejected         LedgerEventType = "REJECTED"
	EventRevoked          LedgerEventType = "REVOKED"
	EventSuperseded       LedgerEventType = "SUPERSEDED"
)

// LedgerEvent is an immutable record of a single state transition.
// The ledger is append-only and serves as the audit trail.
type LedgerEvent struct {
	ID          string            `db:"id"`
	ParameterID string            `db:"parameter_id"`
	EventType   LedgerEventType   `db:"event_type"`
	FromState   ParameterState    `db:"from_state"`
	ToState     ParameterState    `db:"to_state"`
	Actor       string            `db:"actor"`
	Reason      string            `db:"reason"`
	Context     map[string]string `db:"context"`
	OccurredAt  time.Time         `db:"occurred_at"`
}

// SubmitRequest carries the input for proposing a new parameter.
type SubmitRequest struct {
	Key       string
	Value     string
	ValueType ValueType
	Namespace string
	Source    string
	SourceRef string
	Metadata  map[string]string
	Actor     string
}
