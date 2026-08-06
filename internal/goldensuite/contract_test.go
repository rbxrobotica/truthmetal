package goldensuite

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func checkedInSuite(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test file")
	}
	return filepath.Join(
		filepath.Dir(currentFile),
		"..",
		"..",
		"datasets",
		"rbx-rag-public-assistant",
		"v1",
	)
}

func TestCheckedInPublicAssistantSuiteIsValid(t *testing.T) {
	summary, err := ValidateDir(checkedInSuite(t))
	if err != nil {
		t.Fatalf("validate checked-in suite: %v", err)
	}
	if summary.CaseCount != 30 {
		t.Fatalf("case count = %d, want 30", summary.CaseCount)
	}
	if summary.GateStatus != "accepted" || summary.CaseStatus != "accepted" {
		t.Fatalf("suite is not accepted: %#v", summary)
	}
	if summary.SigningTrustAnchorStatus != "pending" || summary.RolloutAuthorized {
		t.Fatalf("suite relaxed pending rollout controls: %#v", summary)
	}
}

func TestArtifactHashMismatchIsRejected(t *testing.T) {
	source := checkedInSuite(t)
	destination := t.TempDir()
	copySuite(t, source, destination)
	casesPath := filepath.Join(destination, "cases.json")
	content, err := os.ReadFile(casesPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(casesPath, append(content, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = ValidateDir(destination)
	if err == nil || !strings.Contains(err.Error(), "case-set hash") {
		t.Fatalf("expected case-set hash error, got %v", err)
	}
}

func TestSymbolicLinkArtifactIsRejected(t *testing.T) {
	source := checkedInSuite(t)
	destination := t.TempDir()
	for _, name := range []string{"manifest.json", "quality-gates.json"} {
		copyFile(t, filepath.Join(source, name), filepath.Join(destination, name))
	}
	if err := os.Symlink(
		filepath.Join(source, "cases.json"),
		filepath.Join(destination, "cases.json"),
	); err != nil {
		t.Fatal(err)
	}

	_, err := ValidateDir(destination)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symbolic link error, got %v", err)
	}
}

func TestUnknownManifestFieldIsRejected(t *testing.T) {
	source := checkedInSuite(t)
	destination := t.TempDir()
	copySuite(t, source, destination)
	manifestPath := filepath.Join(destination, "manifest.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	modified := strings.Replace(
		string(content),
		`"schema_version": "1.0",`,
		`"schema_version": "1.0", "unknown_field": true,`,
		1,
	)
	if err := os.WriteFile(manifestPath, []byte(modified), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = ValidateDir(destination)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestManifestCannotAuthorizeRollout(t *testing.T) {
	directory := checkedInSuite(t)
	_, manifest, err := decodeFile[Manifest](filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	gateBytes, _, err := decodeFile[QualityGateContract](
		filepath.Join(directory, "quality-gates.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	caseBytes, _, err := decodeFile[CaseSetContract](filepath.Join(directory, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest.RolloutAuthorized = true

	err = validateManifest(&manifest, gateBytes, caseBytes)
	if err == nil || !strings.Contains(err.Error(), "must not authorize rollout") {
		t.Fatalf("expected rollout authorization error, got %v", err)
	}
}

func TestBlockedEvidenceCannotEnterAcceptedCases(t *testing.T) {
	directory := checkedInSuite(t)
	_, manifest, err := decodeFile[Manifest](filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, gates, err := decodeFile[QualityGateContract](filepath.Join(directory, "quality-gates.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, cases, err := decodeFile[CaseSetContract](filepath.Join(directory, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	cases.Cases[0].EvidenceDocumentIDs = []string{"rbx-public-strategos-pt-br-v1"}
	required := true
	cases.Cases[0].EvidenceRequired = &required

	err = validateCaseSet(&manifest, &gates, &cases)
	if err == nil || !strings.Contains(err.Error(), "blocked evidence") {
		t.Fatalf("expected blocked evidence error, got %v", err)
	}
}

func TestPendingSigningTrustAnchorCannotPublishAKey(t *testing.T) {
	directory := checkedInSuite(t)
	_, manifest, err := decodeFile[Manifest](filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, gates, err := decodeFile[QualityGateContract](filepath.Join(directory, "quality-gates.json"))
	if err != nil {
		t.Fatal(err)
	}
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	gates.VerentirMapping.ResultSigningPublicKey = &key

	err = validateQualityGates(&manifest, &gates)
	if err == nil || !strings.Contains(err.Error(), "must not publish a key") {
		t.Fatalf("expected pending signing key error, got %v", err)
	}
}

func TestSourceRepositoryURLMustBeCanonical(t *testing.T) {
	source := Source{
		Repository:        "https://user@github.com/rbxrobotica/rbx-rag-public-assistant?ref=main",
		CommitSHA:         strings.Repeat("a", 40),
		QualityGateSHA256: strings.Repeat("b", 64),
		CaseSetSHA256:     strings.Repeat("c", 64),
		CaseCount:         38,
	}
	if err := validateSource(&source); err == nil {
		t.Fatal("expected non-canonical source repository URL to be rejected")
	}
}

func copySuite(t *testing.T, source, destination string) {
	t.Helper()
	for _, name := range []string{"manifest.json", "quality-gates.json", "cases.json"} {
		copyFile(t, filepath.Join(source, name), filepath.Join(destination, name))
	}
}

func copyFile(t *testing.T, source, destination string) {
	t.Helper()
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, content, 0o600); err != nil {
		t.Fatal(err)
	}
}
