package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Athena900/go-prism/internal/evidence"
)

func TestCheckWithAdaptersMissingGoreleaseIsUnknown(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GoreleaseAdapter{}},
		fakeToolRunner{err: errors.New("not found")},
	)

	assertHasStatus(t, items, "api.gorelease.not_installed", evidence.StatusUnknown)
}

func TestCheckWithAdaptersGoreleaseNoAPIChangesPasses(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GoreleaseAdapter{}},
		fakeToolRunner{
			path: "/usr/local/bin/gorelease",
			result: ToolResult{
				Stdout: "# summary\nBase version: v1.0.0\nSuggested version: v1.0.1\n",
			},
		},
	)

	assertHasStatus(t, items, "api.gorelease.no_incompatible_changes", evidence.StatusPass)
	if got := items[0].Provenance.Extra["release_impact"]; got != "patch" {
		t.Fatalf("release_impact = %q, want patch", got)
	}
}

func TestCheckWithAdaptersGoreleaseFirstReleaseImpactUnknown(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "none", Head: "HEAD"},
		[]Adapter{GoreleaseAdapter{}},
		fakeToolRunner{
			path: "/usr/local/bin/gorelease",
			result: ToolResult{
				Stdout: "# summary\nSuggested version: v0.1.0\n",
			},
		},
	)

	assertHasStatus(t, items, "api.gorelease.no_incompatible_changes", evidence.StatusPass)
	if _, ok := items[0].Provenance.Extra["release_impact"]; ok {
		t.Fatalf("release_impact should be omitted when no base version is known: %#v", items[0].Provenance.Extra)
	}
}

func TestCheckWithAdaptersGoreleaseCompatibleChangesWarns(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "v1.0.0", Head: "HEAD"},
		[]Adapter{GoreleaseAdapter{}},
		fakeToolRunner{
			path: "/usr/local/bin/gorelease",
			result: ToolResult{
				Stdout: "# example.com/project\n## compatible changes\nThing: added\n\n# summary\nSuggested version: v1.1.0\n",
			},
		},
	)

	assertHasStatus(t, items, "api.gorelease.compatible_changes", evidence.StatusWarn)
	if got := items[0].Provenance.Extra["release_impact"]; got != "minor" {
		t.Fatalf("release_impact = %q, want minor", got)
	}
}

func TestCheckWithAdaptersGoreleaseIncompatibleChangesBlocks(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GoreleaseAdapter{}},
		fakeToolRunner{
			path: "/usr/local/bin/gorelease",
			result: ToolResult{
				Stdout:   "# example.com/project\n## incompatible changes\nThing: removed\n\n# summary\nCannot suggest a release version.\nIncompatible changes were detected.\n",
				ExitCode: 1,
				Err:      errors.New("exit status 1"),
			},
		},
	)

	assertHasStatus(t, items, "api.gorelease.incompatible_changes", evidence.StatusBlock)
	if got := items[0].Provenance.Extra["release_impact"]; got != "major" {
		t.Fatalf("release_impact = %q, want major", got)
	}
}

func TestCheckWithAdaptersGoreleaseRunFailureIsUnknown(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GoreleaseAdapter{}},
		fakeToolRunner{
			path: "/usr/local/bin/gorelease",
			result: ToolResult{
				Stderr:   "gorelease: could not find base version\n",
				ExitCode: 1,
				Err:      errors.New("exit status 1"),
			},
		},
	)

	assertHasStatus(t, items, "api.gorelease.run_failed", evidence.StatusUnknown)
}

func TestCheckWithAdaptersEmptyAdapterSetIsUnknown(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		nil,
		fakeToolRunner{},
	)

	assertHasStatus(t, items, "api.no_adapters", evidence.StatusUnknown)
}

func TestCheckWithAdaptersCanceledContextIsUnknown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	items := CheckWithAdapters(
		ctx,
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GoreleaseAdapter{}},
		fakeToolRunner{path: "/usr/local/bin/gorelease"},
	)

	assertHasStatus(t, items, "api.timeout", evidence.StatusUnknown)
}

func TestGoreleaseArgsOnlyUseGoreleaseBaseValues(t *testing.T) {
	tests := []struct {
		name string
		base string
		want []string
	}{
		{name: "PR ref is ignored", base: "origin/main", want: nil},
		{name: "semver is passed", base: "v1.2.3", want: []string{"-base=v1.2.3"}},
		{name: "latest is passed", base: "latest", want: []string{"-base=latest"}},
		{name: "none is passed", base: "none", want: []string{"-base=none"}},
		{name: "module at semver is passed", base: "example.com/mod/v2@v2.1.0", want: []string{"-base=example.com/mod/v2@v2.1.0"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := goreleaseArgs(Options{Base: tt.base})
			if len(got) != len(tt.want) {
				t.Fatalf("goreleaseArgs() = %#v, want %#v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("goreleaseArgs() = %#v, want %#v", got, tt.want)
				}
			}
		})
	}
}

func TestGoreleaseDetailsRedactsSensitiveValues(t *testing.T) {
	details := goreleaseDetails("# summary\nSuggested version: v1.0.1 token=abc123\nBearer secret-token\n")
	joined := strings.Join(details, "\n")

	if strings.Contains(joined, "abc123") || strings.Contains(joined, "secret-token") {
		t.Fatalf("details were not redacted: %q", joined)
	}
	if !strings.Contains(joined, "token=[REDACTED]") {
		t.Fatalf("details missing token redaction: %q", joined)
	}
}

type fakeToolRunner struct {
	path   string
	err    error
	result ToolResult
}

func (r fakeToolRunner) LookPath(string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.path, nil
}

func (r fakeToolRunner) Run(context.Context, ToolInvocation) ToolResult {
	return r.result
}

func assertHasStatus(t *testing.T, items []evidence.Item, id string, status evidence.Status) {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			if item.Status != status {
				t.Fatalf("%s status = %q, want %q", id, item.Status, status)
			}
			return
		}
	}
	t.Fatalf("missing evidence item %q in %#v", id, items)
}
