package goldensuite

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	manifestSchemaVersion = "1.0"
	contractSchemaVersion = "1.0"
	expectedPackageID     = "rbx-rag-public-assistant"
	expectedAuthority     = "truthmetal"
	expectedExecutor      = "verentir"
	expectedMode          = "shadow"
)

var requiredMetrics = map[string]string{
	"overall_pass_rate":                   "gte",
	"accuracy_average":                    "gte",
	"scope_adherence_average":             "gte",
	"unsupported_claim_rate":              "lte",
	"financial_boundary_violation_rate":   "lte",
	"stale_or_unauthorized_evidence_rate": "lte",
}

var allowedCategories = map[string]bool{
	"scope_refusal":             true,
	"grounded_product_fact":     true,
	"financial_boundary":        true,
	"product_boundary":          true,
	"unsupported_claim_refusal": true,
	"human_handoff":             true,
}

var evidenceOptionalCategories = map[string]bool{
	"scope_refusal": true,
	"human_handoff": true,
}

var allowedRubricDimensions = map[string]bool{
	"accuracy":        true,
	"cta":             true,
	"scope_adherence": true,
	"tone":            true,
}

type Manifest struct {
	SchemaVersion                 string            `json:"schema_version"`
	SuiteID                       string            `json:"suite_id"`
	PackageID                     string            `json:"package_id"`
	Status                        string            `json:"status"`
	Authority                     string            `json:"authority"`
	IntendedExecutor              string            `json:"intended_executor"`
	Mode                          string            `json:"mode"`
	ProposedAt                    string            `json:"proposed_at"`
	CanonicalRef                  string            `json:"canonical_ref"`
	Source                        Source            `json:"source"`
	AcceptedArtifacts             AcceptedArtifacts `json:"accepted_artifacts"`
	BlockedEvidenceDocumentIDs    []string          `json:"blocked_evidence_document_ids"`
	ExcludedCases                 []ExcludedCase    `json:"excluded_cases"`
	SigningTrustAnchorStatus      string            `json:"signing_trust_anchor_status"`
	RolloutAuthorized             bool              `json:"rollout_authorized"`
	AcceptanceEffectiveOnlyOnMain bool              `json:"acceptance_effective_only_on_main"`
}

type Source struct {
	Repository        string `json:"repository"`
	CommitSHA         string `json:"commit_sha"`
	QualityGateSHA256 string `json:"quality_gate_sha256"`
	CaseSetSHA256     string `json:"case_set_sha256"`
	CaseCount         int    `json:"case_count"`
}

type AcceptedArtifacts struct {
	QualityGateFile   string `json:"quality_gate_file"`
	QualityGateSHA256 string `json:"quality_gate_sha256"`
	CaseSetFile       string `json:"case_set_file"`
	CaseSetSHA256     string `json:"case_set_sha256"`
	CaseCount         int    `json:"case_count"`
}

type ExcludedCase struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type QualityGateContract struct {
	SchemaVersion                 string          `json:"schema_version"`
	PackageID                     string          `json:"package_id"`
	Status                        string          `json:"status"`
	Authority                     string          `json:"authority"`
	IntendedExecutor              string          `json:"intended_executor"`
	ExecutionStatus               string          `json:"execution_status"`
	Mode                          string          `json:"mode"`
	BlocksPublicRollout           bool            `json:"blocks_public_rollout"`
	PublicRolloutApprovalRequired bool            `json:"public_rollout_approval_required"`
	MinimumEvaluatedCases         int             `json:"minimum_evaluated_cases"`
	MinimumConsecutivePassingRuns int             `json:"minimum_consecutive_passing_runs"`
	Metrics                       []MetricGate    `json:"metrics"`
	VerentirMapping               VerentirMapping `json:"verentir_mapping"`
	AcceptanceRequirements        []string        `json:"acceptance_requirements"`
}

type MetricGate struct {
	Key        string  `json:"key"`
	Comparator string  `json:"comparator"`
	Threshold  float64 `json:"threshold"`
}

type VerentirMapping struct {
	ContractVersion        string                   `json:"contract_version"`
	Status                 string                   `json:"status"`
	ResultSigningPublicKey *string                  `json:"result_signing_public_key"`
	Metrics                map[string]MetricMapping `json:"metrics"`
}

type MetricMapping struct {
	Source       string `json:"source"`
	Availability string `json:"availability"`
}

type CaseSetContract struct {
	SchemaVersion    string           `json:"schema_version"`
	PackageID        string           `json:"package_id"`
	Status           string           `json:"status"`
	TruthOwner       string           `json:"truth_owner"`
	IntendedExecutor string           `json:"intended_executor"`
	ExecutionStatus  string           `json:"execution_status"`
	Cases            []CaseDefinition `json:"cases"`
}

type CaseDefinition struct {
	ID                  string   `json:"id"`
	Locale              string   `json:"locale"`
	Category            string   `json:"category"`
	Input               string   `json:"input"`
	ExpectedBehavior    string   `json:"expected_behavior"`
	ForbiddenBehaviors  []string `json:"forbidden_behaviors"`
	EvidenceDocumentIDs []string `json:"evidence_document_ids"`
	EvidenceRequired    *bool    `json:"evidence_required"`
	RubricDimensions    []string `json:"rubric_dimensions"`
	Status              string   `json:"status"`
	Owner               string   `json:"owner"`
}

type Summary struct {
	SuiteID                  string `json:"suite_id"`
	PackageID                string `json:"package_id"`
	CaseCount                int    `json:"case_count"`
	GateStatus               string `json:"gate_status"`
	CaseStatus               string `json:"case_status"`
	SigningTrustAnchorStatus string `json:"signing_trust_anchor_status"`
	RolloutAuthorized        bool   `json:"rollout_authorized"`
}

func ValidateDir(directory string) (Summary, error) {
	_, manifest, err := decodeFile[Manifest](filepath.Join(directory, "manifest.json"))
	if err != nil {
		return Summary{}, err
	}
	if manifest.AcceptedArtifacts.QualityGateFile != "quality-gates.json" ||
		manifest.AcceptedArtifacts.CaseSetFile != "cases.json" {
		return Summary{}, fmt.Errorf("accepted artifact filenames must remain pinned")
	}

	gatePath := filepath.Join(directory, manifest.AcceptedArtifacts.QualityGateFile)
	gateBytes, gates, err := decodeFile[QualityGateContract](gatePath)
	if err != nil {
		return Summary{}, err
	}
	casePath := filepath.Join(directory, manifest.AcceptedArtifacts.CaseSetFile)
	caseBytes, cases, err := decodeFile[CaseSetContract](casePath)
	if err != nil {
		return Summary{}, err
	}

	if err := validateManifest(&manifest, gateBytes, caseBytes); err != nil {
		return Summary{}, err
	}
	if err := validateQualityGates(&manifest, &gates); err != nil {
		return Summary{}, err
	}
	if err := validateCaseSet(&manifest, &gates, &cases); err != nil {
		return Summary{}, err
	}

	return Summary{
		SuiteID:                  manifest.SuiteID,
		PackageID:                manifest.PackageID,
		CaseCount:                len(cases.Cases),
		GateStatus:               gates.Status,
		CaseStatus:               cases.Status,
		SigningTrustAnchorStatus: manifest.SigningTrustAnchorStatus,
		RolloutAuthorized:        manifest.RolloutAuthorized,
	}, nil
}

func decodeFile[T any](path string) ([]byte, T, error) {
	var value T
	info, err := os.Lstat(path)
	if err != nil {
		return nil, value, fmt.Errorf("inspect %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, value, fmt.Errorf("%s must be a regular file and not a symbolic link", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, value, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, value, fmt.Errorf("inspect opened %s: %w", path, err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, value, fmt.Errorf("%s changed during validation or is not a regular file", path)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, value, fmt.Errorf("read %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return nil, value, fmt.Errorf("decode %s: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, value, fmt.Errorf("decode %s: trailing JSON value", path)
	}
	return content, value, nil
}

func validateManifest(manifest *Manifest, gateBytes, caseBytes []byte) error {
	if manifest.SchemaVersion != manifestSchemaVersion {
		return fmt.Errorf("manifest schema_version must be %q", manifestSchemaVersion)
	}
	if manifest.SuiteID == "" {
		return fmt.Errorf("manifest suite_id is required")
	}
	if manifest.PackageID != expectedPackageID {
		return fmt.Errorf("manifest package_id must be %q", expectedPackageID)
	}
	if manifest.Status != "accepted" || manifest.Authority != expectedAuthority {
		return fmt.Errorf("manifest must be accepted by TruthMetal")
	}
	if manifest.IntendedExecutor != expectedExecutor || manifest.Mode != expectedMode {
		return fmt.Errorf("manifest must target Verentir in shadow mode")
	}
	if !manifest.AcceptanceEffectiveOnlyOnMain {
		return fmt.Errorf("manifest acceptance must become effective only on protected main")
	}
	if manifest.RolloutAuthorized {
		return fmt.Errorf("TruthMetal acceptance must not authorize rollout")
	}
	if _, err := time.Parse(time.DateOnly, manifest.ProposedAt); err != nil {
		return fmt.Errorf("manifest proposed_at must be an ISO date")
	}
	if manifest.CanonicalRef != "refs/heads/main" {
		return fmt.Errorf("manifest canonical_ref must remain refs/heads/main")
	}
	if err := validateSource(&manifest.Source); err != nil {
		return err
	}
	if manifest.AcceptedArtifacts.QualityGateSHA256 != sha256Hex(gateBytes) {
		return fmt.Errorf("accepted quality-gate hash does not match quality-gates.json")
	}
	if manifest.AcceptedArtifacts.CaseSetSHA256 != sha256Hex(caseBytes) {
		return fmt.Errorf("accepted case-set hash does not match cases.json")
	}
	if manifest.AcceptedArtifacts.CaseCount <= 0 {
		return fmt.Errorf("accepted case count must be positive")
	}
	if len(manifest.BlockedEvidenceDocumentIDs) == 0 {
		return fmt.Errorf("blocked evidence document ids must record pending claim families")
	}
	if manifest.Source.CaseCount != manifest.AcceptedArtifacts.CaseCount+len(manifest.ExcludedCases) {
		return fmt.Errorf("source case count must equal accepted plus excluded cases")
	}

	blocked := map[string]bool{}
	for _, documentID := range manifest.BlockedEvidenceDocumentIDs {
		if documentID == "" || blocked[documentID] {
			return fmt.Errorf("blocked evidence document ids must be non-empty and unique")
		}
		blocked[documentID] = true
	}
	excluded := map[string]bool{}
	for _, item := range manifest.ExcludedCases {
		if item.ID == "" || item.Reason == "" || excluded[item.ID] {
			return fmt.Errorf("excluded cases must have unique ids and reasons")
		}
		excluded[item.ID] = true
	}

	if manifest.SigningTrustAnchorStatus != "pending" &&
		manifest.SigningTrustAnchorStatus != "accepted" {
		return fmt.Errorf("signing trust anchor status must be pending or accepted")
	}
	return nil
}

func validateSource(source *Source) error {
	parsed, err := url.Parse(source.Repository)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "github.com" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		parsed.ForceQuery {
		return fmt.Errorf("source repository must be an HTTPS GitHub URL")
	}
	if parsed.Path != "/rbxrobotica/rbx-rag-public-assistant" {
		return fmt.Errorf("source repository must be the public-assistant RAG package")
	}
	if !isLowerHex(source.CommitSHA, 40) {
		return fmt.Errorf("source commit_sha must be a lowercase 40-character Git SHA")
	}
	if !isLowerHex(source.QualityGateSHA256, 64) || !isLowerHex(source.CaseSetSHA256, 64) {
		return fmt.Errorf("source artifact hashes must be lowercase SHA-256 values")
	}
	if source.CaseCount <= 0 {
		return fmt.Errorf("source case_count must be positive")
	}
	return nil
}

func validateQualityGates(manifest *Manifest, gates *QualityGateContract) error {
	if gates.SchemaVersion != contractSchemaVersion || gates.PackageID != manifest.PackageID {
		return fmt.Errorf("quality-gate contract identity does not match the manifest")
	}
	if gates.Status != "accepted" || gates.Authority != expectedAuthority {
		return fmt.Errorf("quality-gate contract must be accepted by TruthMetal")
	}
	if gates.IntendedExecutor != expectedExecutor || gates.ExecutionStatus != "adapter-available" {
		return fmt.Errorf("quality-gate contract must target the available Verentir adapter")
	}
	if gates.Mode != expectedMode || !gates.BlocksPublicRollout ||
		!gates.PublicRolloutApprovalRequired {
		return fmt.Errorf("quality-gate contract must remain shadow-only and require human rollout approval")
	}
	if gates.MinimumEvaluatedCases <= 0 ||
		gates.MinimumEvaluatedCases > manifest.AcceptedArtifacts.CaseCount {
		return fmt.Errorf("minimum evaluated cases must fit the accepted suite")
	}
	if gates.MinimumConsecutivePassingRuns <= 0 {
		return fmt.Errorf("minimum consecutive passing runs must be positive")
	}
	if len(gates.AcceptanceRequirements) == 0 {
		return fmt.Errorf("remaining rollout acceptance requirements are required")
	}

	seen := map[string]bool{}
	for _, metric := range gates.Metrics {
		comparator, required := requiredMetrics[metric.Key]
		if !required || seen[metric.Key] {
			return fmt.Errorf("quality-gate metrics must match the required unique metric set")
		}
		if metric.Comparator != comparator || metric.Threshold < 0 || metric.Threshold > 1 {
			return fmt.Errorf("quality-gate metric %q has an invalid comparator or threshold", metric.Key)
		}
		seen[metric.Key] = true
	}
	if len(seen) != len(requiredMetrics) {
		return fmt.Errorf("quality-gate contract is missing required metrics")
	}
	if gates.VerentirMapping.ContractVersion != "1" ||
		gates.VerentirMapping.Status != "available" {
		return fmt.Errorf("Verentir mapping version 1 must remain available")
	}
	if len(gates.VerentirMapping.Metrics) != len(requiredMetrics) {
		return fmt.Errorf("Verentir mapping must cover every required metric")
	}
	for key := range requiredMetrics {
		mapping, ok := gates.VerentirMapping.Metrics[key]
		if !ok || mapping.Availability != "available" ||
			mapping.Source != "rag_quality_report.metrics."+key {
			return fmt.Errorf("invalid Verentir mapping for metric %q", key)
		}
	}

	key := gates.VerentirMapping.ResultSigningPublicKey
	if manifest.SigningTrustAnchorStatus == "pending" {
		if key != nil {
			return fmt.Errorf("pending signing trust anchor must not publish a key")
		}
		return nil
	}
	if key == nil {
		return fmt.Errorf("accepted signing trust anchor requires a public key")
	}
	decoded, err := base64.StdEncoding.DecodeString(*key)
	if err != nil || len(decoded) != 32 {
		return fmt.Errorf("accepted signing trust anchor must be a base64 Ed25519 public key")
	}
	return nil
}

func validateCaseSet(manifest *Manifest, gates *QualityGateContract, cases *CaseSetContract) error {
	if cases.SchemaVersion != contractSchemaVersion || cases.PackageID != manifest.PackageID {
		return fmt.Errorf("case-set contract identity does not match the manifest")
	}
	if cases.Status != "accepted" || cases.TruthOwner != expectedAuthority {
		return fmt.Errorf("case-set contract must be accepted and owned by TruthMetal")
	}
	if cases.IntendedExecutor != expectedExecutor || cases.ExecutionStatus != "adapter-available" {
		return fmt.Errorf("case-set contract must target the available Verentir adapter")
	}
	if len(cases.Cases) != manifest.AcceptedArtifacts.CaseCount ||
		len(cases.Cases) < gates.MinimumEvaluatedCases {
		return fmt.Errorf("case-set count must match the manifest and satisfy the quality gate")
	}

	blocked := map[string]bool{}
	for _, documentID := range manifest.BlockedEvidenceDocumentIDs {
		blocked[documentID] = true
	}
	excluded := map[string]bool{}
	for _, item := range manifest.ExcludedCases {
		excluded[item.ID] = true
	}
	ids := map[string]bool{}
	inputs := map[string]bool{}
	for _, item := range cases.Cases {
		if item.ID == "" || ids[item.ID] || excluded[item.ID] {
			return fmt.Errorf("accepted case ids must be unique and not excluded")
		}
		ids[item.ID] = true
		normalizedInput := strings.ToLower(strings.TrimSpace(item.Input))
		if normalizedInput == "" || inputs[normalizedInput] {
			return fmt.Errorf("accepted case inputs must be non-empty and unique")
		}
		inputs[normalizedInput] = true
		if item.Locale == "" || item.ExpectedBehavior == "" || len(item.ForbiddenBehaviors) == 0 {
			return fmt.Errorf("accepted case %q is missing required oracle content", item.ID)
		}
		if hasDuplicates(item.ForbiddenBehaviors) {
			return fmt.Errorf("accepted case %q must declare unique forbidden behaviors", item.ID)
		}
		if !allowedCategories[item.Category] {
			return fmt.Errorf("accepted case %q has an unknown category", item.ID)
		}
		if item.Status != "accepted" || item.Owner != expectedAuthority {
			return fmt.Errorf("accepted case %q must be accepted and owned by TruthMetal", item.ID)
		}
		if item.EvidenceRequired == nil {
			return fmt.Errorf("accepted case %q must declare an evidence policy", item.ID)
		}
		if !*item.EvidenceRequired && !evidenceOptionalCategories[item.Category] {
			return fmt.Errorf("accepted case %q may not omit evidence for its category", item.ID)
		}
		if *item.EvidenceRequired && len(item.EvidenceDocumentIDs) == 0 {
			return fmt.Errorf("accepted case %q requires at least one evidence document", item.ID)
		}
		if !*item.EvidenceRequired && len(item.EvidenceDocumentIDs) != 0 {
			return fmt.Errorf("accepted case %q must have an empty optional evidence allowlist", item.ID)
		}
		for _, documentID := range item.EvidenceDocumentIDs {
			if documentID == "" || blocked[documentID] {
				return fmt.Errorf("accepted case %q references blocked evidence %q", item.ID, documentID)
			}
		}
		if hasDuplicates(item.EvidenceDocumentIDs) {
			return fmt.Errorf("accepted case %q must declare unique evidence documents", item.ID)
		}
		if len(item.RubricDimensions) == 0 || hasDuplicates(item.RubricDimensions) {
			return fmt.Errorf("accepted case %q must declare unique rubric dimensions", item.ID)
		}
		for _, dimension := range item.RubricDimensions {
			if !allowedRubricDimensions[dimension] {
				return fmt.Errorf("accepted case %q has an unknown rubric dimension", item.ID)
			}
		}
	}
	return nil
}

func hasDuplicates(values []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}

func sha256Hex(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func isLowerHex(value string, length int) bool {
	if len(value) != length || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
