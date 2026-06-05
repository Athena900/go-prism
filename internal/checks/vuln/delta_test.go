package vuln

import (
	"strings"
	"testing"

	"github.com/Athena900/go-prism/internal/evidence"
)

func TestCompareGovulncheckDeltaNewReachableFindingsBlock(t *testing.T) {
	base := mustGovulncheckReport(t, "")
	head := mustGovulncheckReport(t, strings.Join([]string{
		`{"osv":{"id":"GO-0000-0001","database_specific":{"url":"https://pkg.go.dev/vuln/GO-0000-0001"}}}`,
		`{"finding":{"osv":"GO-0000-0001","fixed_version":"v1.2.3","trace":[{"module":"example.com/vuln","version":"v1.0.0","package":"example.com/vuln","function":"Bad"}]}}`,
	}, "\n"))

	items := compareGovulncheckDelta(deltaTestOptions(), "/bin/govulncheck", []string{"-format=json", "./..."}, deltaBaseTarget(), deltaHeadTarget(), base, head)

	assertHasStatus(t, items, "vuln.govulncheck.delta.new_reachable_findings", evidence.StatusBlock)
	if !strings.Contains(strings.Join(items[0].Details, "\n"), "new GO-0000-0001") {
		t.Fatalf("new finding detail missing: %#v", items[0].Details)
	}
}

func TestCompareGovulncheckDeltaNewPackageFindingsWarn(t *testing.T) {
	base := mustGovulncheckReport(t, "")
	head := mustGovulncheckReport(t, `{"finding":{"osv":"GO-0000-0002","trace":[{"module":"example.com/vuln","version":"v1.0.0","package":"example.com/vuln"}]}}`+"\n")

	items := compareGovulncheckDelta(deltaTestOptions(), "/bin/govulncheck", []string{"-format=json", "./..."}, deltaBaseTarget(), deltaHeadTarget(), base, head)

	assertHasStatus(t, items, "vuln.govulncheck.delta.new_findings", evidence.StatusWarn)
}

func TestCompareGovulncheckDeltaFixedFindingsInfo(t *testing.T) {
	base := mustGovulncheckReport(t, `{"finding":{"osv":"GO-0000-0003","trace":[{"module":"example.com/vuln","version":"v1.0.0"}]}}`+"\n")
	head := mustGovulncheckReport(t, "")

	items := compareGovulncheckDelta(deltaTestOptions(), "/bin/govulncheck", []string{"-format=json", "./..."}, deltaBaseTarget(), deltaHeadTarget(), base, head)

	assertHasStatus(t, items, "vuln.govulncheck.delta.fixed_findings", evidence.StatusInfo)
	if !strings.Contains(strings.Join(items[0].Details, "\n"), "fixed GO-0000-0003") {
		t.Fatalf("fixed finding detail missing: %#v", items[0].Details)
	}
}

func TestCompareGovulncheckDeltaNoChangesPasses(t *testing.T) {
	base := mustGovulncheckReport(t, `{"finding":{"osv":"GO-0000-0004","trace":[{"module":"example.com/vuln","version":"v1.0.0"}]}}`+"\n")
	head := mustGovulncheckReport(t, `{"finding":{"osv":"GO-0000-0004","trace":[{"module":"example.com/vuln","version":"v1.1.0"}]}}`+"\n")

	items := compareGovulncheckDelta(deltaTestOptions(), "/bin/govulncheck", []string{"-format=json", "./..."}, deltaBaseTarget(), deltaHeadTarget(), base, head)

	assertHasStatus(t, items, "vuln.govulncheck.delta.no_changes", evidence.StatusPass)
}

func TestParseGovulncheckGroupsSameOSVDifferentPackagesSeparately(t *testing.T) {
	report := mustGovulncheckReport(t, strings.Join([]string{
		`{"finding":{"osv":"GO-0000-0005","trace":[{"module":"example.com/vuln","version":"v1.0.0","package":"example.com/vuln/a"}]}}`,
		`{"finding":{"osv":"GO-0000-0005","trace":[{"module":"example.com/vuln","version":"v1.0.0","package":"example.com/vuln/b"}]}}`,
	}, "\n"))

	if len(report.Findings) != 2 {
		t.Fatalf("Findings len = %d, want 2: %#v", len(report.Findings), report.Findings)
	}
}

func mustGovulncheckReport(t *testing.T, stream string) govulncheckReport {
	t.Helper()
	report, err := parseGovulncheckJSON(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("parseGovulncheckJSON() error = %v", err)
	}
	return report
}

func deltaTestOptions() Options {
	return Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"}
}

func deltaBaseTarget() govulncheckTarget {
	return govulncheckTarget{Label: "base", Ref: "origin/main", WorkDir: "/tmp/base", Source: "origin/main"}
}

func deltaHeadTarget() govulncheckTarget {
	return govulncheckTarget{Label: "head", Ref: "HEAD", WorkDir: ".", Source: "worktree"}
}
