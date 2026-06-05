package api

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Athena900/go-prism/internal/evidence"
)

func TestModverMissingToolIsInfo(t *testing.T) {
	item := ModverAdapter{}.Check(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		modverFakeRunner{missing: map[string]error{"modver": errors.New("not found")}},
	)

	if item.ID != "api.modver.not_installed" {
		t.Fatalf("ID = %q", item.ID)
	}
	if item.Status != evidence.StatusInfo {
		t.Fatalf("Status = %q, want info", item.Status)
	}
}

func TestModverExitCodeMapping(t *testing.T) {
	tests := []struct {
		name          string
		exitCode      int
		wantID        string
		wantStatus    evidence.Status
		wantImpact    string
		impactPresent bool
	}{
		{name: "none", exitCode: 0, wantID: "api.modver.none_required", wantStatus: evidence.StatusPass},
		{name: "patch", exitCode: 1, wantID: "api.modver.patch_required", wantStatus: evidence.StatusPass, wantImpact: "patch", impactPresent: true},
		{name: "minor", exitCode: 2, wantID: "api.modver.minor_required", wantStatus: evidence.StatusWarn, wantImpact: "minor", impactPresent: true},
		{name: "major", exitCode: 3, wantID: "api.modver.major_required", wantStatus: evidence.StatusBlock, wantImpact: "major", impactPresent: true},
		{name: "error", exitCode: 4, wantID: "api.modver.run_failed", wantStatus: evidence.StatusUnknown},
		{name: "unexpected", exitCode: 99, wantID: "api.modver.unclassified_output", wantStatus: evidence.StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := modverFakeRunner{
				results: map[string]ToolResult{
					"git rev-parse --git-dir": {Stdout: ".git\n"},
					"modver -q -git " + filepath.Clean("/repo/.git") + " origin/main HEAD": {
						ExitCode: tt.exitCode,
						Err:      exitErrForCode(tt.exitCode),
					},
				},
			}

			item := ModverAdapter{}.Check(
				context.Background(),
				Options{WorkDir: "/repo", Base: "origin/main", Head: "HEAD"},
				runner,
			)

			if item.ID != tt.wantID {
				t.Fatalf("ID = %q, want %q", item.ID, tt.wantID)
			}
			if item.Status != tt.wantStatus {
				t.Fatalf("Status = %q, want %q", item.Status, tt.wantStatus)
			}
			gotImpact, ok := item.Provenance.Extra["release_impact"]
			if ok != tt.impactPresent {
				t.Fatalf("release_impact present = %v, want %v; extra=%#v", ok, tt.impactPresent, item.Provenance.Extra)
			}
			if gotImpact != tt.wantImpact {
				t.Fatalf("release_impact = %q, want %q", gotImpact, tt.wantImpact)
			}
			if got := item.Provenance.Extra["exit_code"]; got != strconv.Itoa(tt.exitCode) {
				t.Fatalf("exit_code = %q, want %d", got, tt.exitCode)
			}
		})
	}
}

func TestModverGitContextFailureIsUnknown(t *testing.T) {
	item := ModverAdapter{}.Check(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		modverFakeRunner{
			results: map[string]ToolResult{
				"git rev-parse --git-dir": {
					Stderr:   "fatal: not a git repository\n",
					ExitCode: 128,
					Err:      errors.New("exit status 128"),
				},
			},
		},
	)

	if item.ID != "api.modver.git_context_failed" {
		t.Fatalf("ID = %q", item.ID)
	}
	if item.Status != evidence.StatusUnknown {
		t.Fatalf("Status = %q, want unknown", item.Status)
	}
}

func TestModverDetailsRedactsAndBoundsOutput(t *testing.T) {
	result := ToolResult{
		ExitCode: 2,
		Stdout: strings.Join([]string{
			"line 1 token=abc123",
			"line 2",
			"line 3",
			"line 4",
			"line 5",
			"line 6",
			"line 7",
			"line 8",
			"line 9",
		}, "\n"),
	}

	details := modverDetails(result, "Minor")
	if len(details) != maxModverDetails {
		t.Fatalf("details len = %d, want %d: %#v", len(details), maxModverDetails, details)
	}
	joined := strings.Join(details, "\n")
	if strings.Contains(joined, "abc123") {
		t.Fatalf("details were not redacted: %q", joined)
	}
	if !strings.Contains(joined, "token=[REDACTED]") {
		t.Fatalf("details missing token redaction: %q", joined)
	}
}

func TestDefaultAdaptersIncludeModver(t *testing.T) {
	adapters := defaultAdapters()

	var hasGorelease bool
	var hasModver bool
	for _, adapter := range adapters {
		switch adapter.(type) {
		case GoreleaseAdapter:
			hasGorelease = true
		case ModverAdapter:
			hasModver = true
		}
	}
	if !hasGorelease {
		t.Fatal("default adapters missing GoreleaseAdapter")
	}
	if !hasModver {
		t.Fatal("default adapters missing ModverAdapter")
	}
}

func exitErrForCode(code int) error {
	if code == 0 {
		return nil
	}
	return errors.New("exit status " + strconv.Itoa(code))
}

type modverFakeRunner struct {
	missing map[string]error
	results map[string]ToolResult
	runs    []ToolInvocation
}

func (r modverFakeRunner) LookPath(name string) (string, error) {
	if err, ok := r.missing[name]; ok {
		return "", err
	}
	return "/bin/" + name, nil
}

func (r modverFakeRunner) Run(_ context.Context, invocation ToolInvocation) ToolResult {
	key := filepath.Base(invocation.Path)
	if len(invocation.Args) > 0 {
		key += " " + strings.Join(invocation.Args, " ")
	}
	if result, ok := r.results[key]; ok {
		return result
	}
	return ToolResult{}
}
