package vuln

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
)

func TestCheckWithAdaptersMissingGovulncheckIsUnknown(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GovulncheckAdapter{}},
		fakeRunner{err: errors.New("not found")},
	)

	assertHasStatus(t, items, "vuln.govulncheck.not_installed", evidence.StatusUnknown)
}

func TestCheckWithAdaptersGovulncheckNoFindingsPasses(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GovulncheckAdapter{}},
		fakeRunner{
			path: "/usr/local/bin/govulncheck",
			result: command.Result{
				Stdout: `{"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck","scanner_version":"v1.3.0","db":"https://vuln.go.dev","scan_level":"symbol"}}` + "\n",
			},
		},
	)

	assertHasStatus(t, items, "vuln.govulncheck.no_findings", evidence.StatusPass)
}

func TestCheckWithAdaptersGovulncheckReachableFindingsBlock(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GovulncheckAdapter{}},
		fakeRunner{
			path: "/usr/local/bin/govulncheck",
			result: command.Result{
				Stdout: strings.Join([]string{
					`{"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck","scan_level":"symbol"}}`,
					`{"osv":{"id":"GO-0000-0001","details":"test vuln","database_specific":{"url":"https://pkg.go.dev/vuln/GO-0000-0001"}}}`,
					`{"finding":{"osv":"GO-0000-0001","fixed_version":"v1.2.3","trace":[{"module":"example.com/vuln","version":"v1.0.0","package":"example.com/vuln","function":"Bad"}]}}`,
				}, "\n"),
			},
		},
	)

	assertHasStatus(t, items, "vuln.govulncheck.reachable_findings", evidence.StatusBlock)
	if !strings.Contains(strings.Join(items[0].Details, "\n"), "symbol=Bad") {
		t.Fatalf("details missing symbol: %#v", items[0].Details)
	}
}

func TestCheckWithAdaptersGovulncheckFindsMostSpecificTraceFrame(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GovulncheckAdapter{}},
		fakeRunner{
			path: "/usr/local/bin/govulncheck",
			result: command.Result{
				Stdout: strings.Join([]string{
					`{"finding":{"osv":"GO-0000-0001","trace":[{"module":"example.com/vuln","version":"v1.0.0"},{"module":"example.com/vuln","version":"v1.0.0","package":"example.com/vuln","function":"Bad"}]}}`,
				}, "\n"),
			},
		},
	)

	assertHasStatus(t, items, "vuln.govulncheck.reachable_findings", evidence.StatusBlock)
	if !strings.Contains(strings.Join(items[0].Details, "\n"), "symbol=Bad") {
		t.Fatalf("details missing most specific symbol: %#v", items[0].Details)
	}
}

func TestCheckWithAdaptersGovulncheckPackageFindingsWarn(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GovulncheckAdapter{}},
		fakeRunner{
			path: "/usr/local/bin/govulncheck",
			result: command.Result{
				Stdout: `{"finding":{"osv":"GO-0000-0002","trace":[{"module":"example.com/vuln","version":"v1.0.0","package":"example.com/vuln"}]}}` + "\n",
			},
		},
	)

	assertHasStatus(t, items, "vuln.govulncheck.findings", evidence.StatusWarn)
}

func TestCheckWithAdaptersGovulncheckRunFailureIsUnknown(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GovulncheckAdapter{}},
		fakeRunner{
			path: "/usr/local/bin/govulncheck",
			result: command.Result{
				Stderr:   "govulncheck: pattern ./...: no packages\n",
				ExitCode: 1,
				Err:      errors.New("exit status 1"),
			},
		},
	)

	assertHasStatus(t, items, "vuln.govulncheck.run_failed", evidence.StatusUnknown)
}

func TestCheckWithAdaptersGovulncheckParseFailureIsUnknown(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GovulncheckAdapter{}},
		fakeRunner{
			path: "/usr/local/bin/govulncheck",
			result: command.Result{
				Stdout: "{not json}\n",
			},
		},
	)

	assertHasStatus(t, items, "vuln.govulncheck.parse_failed", evidence.StatusUnknown)
}

func TestCheckWithAdaptersEmptyAdapterSetIsUnknown(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		nil,
		fakeRunner{},
	)

	assertHasStatus(t, items, "vuln.no_adapters", evidence.StatusUnknown)
}

func TestCheckWithAdaptersCanceledContextIsUnknown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	items := CheckWithAdapters(
		ctx,
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GovulncheckAdapter{}},
		fakeRunner{path: "/usr/local/bin/govulncheck"},
	)

	assertHasStatus(t, items, "vuln.timeout", evidence.StatusUnknown)
}

type fakeRunner struct {
	path   string
	err    error
	result command.Result
}

func (r fakeRunner) LookPath(string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.path, nil
}

func (r fakeRunner) Run(context.Context, command.Invocation) command.Result {
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
