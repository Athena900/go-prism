package api

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Athena900/go-prism/internal/evidence"
)

func TestGoAPIDiffMissingToolIsInfo(t *testing.T) {
	item := GoAPIDiffAdapter{}.Check(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		goAPIDiffFakeRunner{missing: map[string]error{"go-apidiff": errors.New("not found")}},
	)

	if item.ID != "api.goapidiff.not_installed" {
		t.Fatalf("ID = %q", item.ID)
	}
	if item.Status != evidence.StatusInfo {
		t.Fatalf("Status = %q, want info", item.Status)
	}
}

func TestGoAPIDiffOutputClassification(t *testing.T) {
	tests := []struct {
		name          string
		result        ToolResult
		wantID        string
		wantStatus    evidence.Status
		wantImpact    string
		impactPresent bool
	}{
		{
			name:          "no changes",
			result:        ToolResult{},
			wantID:        "api.goapidiff.no_changes",
			wantStatus:    evidence.StatusPass,
			wantImpact:    "patch",
			impactPresent: true,
		},
		{
			name: "compatible changes",
			result: ToolResult{
				Stdout: "example.com/project\n  Compatible changes:\n  - ParseBytes: added\n",
			},
			wantID:        "api.goapidiff.compatible_changes",
			wantStatus:    evidence.StatusWarn,
			wantImpact:    "minor",
			impactPresent: true,
		},
		{
			name: "incompatible changes",
			result: ToolResult{
				Stdout:   "example.com/project\n  Incompatible changes:\n  - Parse: changed from func(string) to func(string, bool)\n",
				ExitCode: 1,
				Err:      errors.New("exit status 1"),
			},
			wantID:        "api.goapidiff.incompatible_changes",
			wantStatus:    evidence.StatusBlock,
			wantImpact:    "major",
			impactPresent: true,
		},
		{
			name: "incompatible wins over compatible",
			result: ToolResult{
				Stdout: "example.com/project\n  Incompatible changes:\n  - Parse: removed\n  Compatible changes:\n  - ParseBytes: added\n",
			},
			wantID:        "api.goapidiff.incompatible_changes",
			wantStatus:    evidence.StatusBlock,
			wantImpact:    "major",
			impactPresent: true,
		},
		{
			name: "run failure",
			result: ToolResult{
				Stderr:   "fatal: bad revision\n",
				ExitCode: 128,
				Err:      errors.New("exit status 128"),
			},
			wantID:     "api.goapidiff.run_failed",
			wantStatus: evidence.StatusUnknown,
		},
		{
			name: "unclassified output",
			result: ToolResult{
				Stdout: "unexpected output\n",
			},
			wantID:     "api.goapidiff.unclassified_output",
			wantStatus: evidence.StatusUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := goAPIDiffFakeRunner{
				results: map[string]ToolResult{
					"git rev-parse --show-toplevel":                                    {Stdout: "/repo\n"},
					"go-apidiff --repo-path /repo --print-compatible origin/main HEAD": tt.result,
				},
			}

			item := GoAPIDiffAdapter{}.Check(
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
			if got := item.Provenance.Extra["repo_root"]; got != "/repo" {
				t.Fatalf("repo_root = %q, want /repo", got)
			}
		})
	}
}

func TestGoAPIDiffGitContextFailureIsUnknown(t *testing.T) {
	item := GoAPIDiffAdapter{}.Check(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		goAPIDiffFakeRunner{
			results: map[string]ToolResult{
				"git rev-parse --show-toplevel": {
					Stderr:   "fatal: not a git repository\n",
					ExitCode: 128,
					Err:      errors.New("exit status 128"),
				},
			},
		},
	)

	if item.ID != "api.goapidiff.git_context_failed" {
		t.Fatalf("ID = %q", item.ID)
	}
	if item.Status != evidence.StatusUnknown {
		t.Fatalf("Status = %q, want unknown", item.Status)
	}
}

func TestGoAPIDiffDetailsRedactsAndBoundsOutput(t *testing.T) {
	result := ToolResult{
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
			"line 10",
			"line 11",
			"line 12",
			"line 13",
		}, "\n"),
	}

	details := goAPIDiffDetails(result)
	if len(details) != maxGoAPIDiffDetails {
		t.Fatalf("details len = %d, want %d: %#v", len(details), maxGoAPIDiffDetails, details)
	}
	joined := strings.Join(details, "\n")
	if strings.Contains(joined, "abc123") {
		t.Fatalf("details were not redacted: %q", joined)
	}
	if !strings.Contains(joined, "token=[REDACTED]") {
		t.Fatalf("details missing token redaction: %q", joined)
	}
}

type goAPIDiffFakeRunner struct {
	missing map[string]error
	results map[string]ToolResult
}

func (r goAPIDiffFakeRunner) LookPath(name string) (string, error) {
	if err, ok := r.missing[name]; ok {
		return "", err
	}
	return "/bin/" + name, nil
}

func (r goAPIDiffFakeRunner) Run(_ context.Context, invocation ToolInvocation) ToolResult {
	key := filepath.Base(invocation.Path)
	if len(invocation.Args) > 0 {
		key += " " + strings.Join(invocation.Args, " ")
	}
	if result, ok := r.results[key]; ok {
		return result
	}
	return ToolResult{}
}
