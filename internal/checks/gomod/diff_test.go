package gomod

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Athena900/go-prism/internal/evidence"
)

func TestDiffSnapshotsNoChanges(t *testing.T) {
	content := []byte("module example.com/project\n\ngo 1.22\n")
	base := mustSnapshot(t, "base:go.mod", content)
	head := mustSnapshot(t, "head:go.mod", content)

	items := diffSnapshots(Options{Base: "main", Head: "HEAD"}, base, head)

	assertHasStatus(t, items, "gomod.diff.no_changes", evidence.StatusPass)
}

func TestDiffSnapshotsModuleChangeBlocks(t *testing.T) {
	base := mustSnapshot(t, "base:go.mod", []byte("module example.com/old\n\ngo 1.22\n"))
	head := mustSnapshot(t, "head:go.mod", []byte("module example.com/new\n\ngo 1.22\n"))

	items := diffSnapshots(Options{Base: "main", Head: "HEAD"}, base, head)

	assertHasStatus(t, items, "gomod.diff.module_changed", evidence.StatusBlock)
}

func TestDiffSnapshotsDirectiveAndRequirementChanges(t *testing.T) {
	base := mustSnapshot(t, "base:go.mod", []byte(`module example.com/project

go 1.21

require (
	github.com/acme/direct v1.0.0
	github.com/acme/indirect v1.0.0 // indirect
)
`))
	head := mustSnapshot(t, "head:go.mod", []byte(`module example.com/project

go 1.22

toolchain go1.22.1

require (
	github.com/acme/direct v1.1.0
	github.com/acme/indirect v1.0.1 // indirect
	github.com/acme/newdirect v0.1.0
)

replace github.com/acme/replaced => ../replaced

retract v0.1.0
`))

	items := diffSnapshots(Options{Base: "main", Head: "HEAD"}, base, head)

	assertHasStatus(t, items, "gomod.diff.go_directive_changed", evidence.StatusWarn)
	assertHasStatus(t, items, "gomod.diff.toolchain_changed", evidence.StatusWarn)
	assertHasStatus(t, items, "gomod.diff.direct_requirements_changed", evidence.StatusWarn)
	assertHasStatus(t, items, "gomod.diff.indirect_requirements_changed", evidence.StatusInfo)
	assertHasStatus(t, items, "gomod.diff.replace_changed", evidence.StatusWarn)
	assertHasStatus(t, items, "gomod.diff.retract_changed", evidence.StatusInfo)
}

func TestCheckDiffReadsBaseRefAndWorktreeHead(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "module example.com/project\n\ngo 1.21\n")
	runGit(t, dir, "init")
	runGit(t, dir, "add", "go.mod")
	runGit(t, dir, "-c", "user.name=go-prism", "-c", "user.email=go-prism@example.com", "commit", "-m", "base")

	writeGoMod(t, dir, "module example.com/project\n\ngo 1.22\n")

	items := CheckDiff(context.Background(), Options{
		WorkDir: dir,
		Base:    "HEAD",
		Head:    "HEAD",
	})

	assertHasStatus(t, items, "gomod.diff.go_directive_changed", evidence.StatusWarn)
}

func TestCheckDiffMissingBaseIsUnknown(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "module example.com/project\n\ngo 1.22\n")

	items := CheckDiff(context.Background(), Options{
		WorkDir: dir,
		Base:    "origin/main",
		Head:    "HEAD",
	})

	assertHasStatus(t, items, "gomod.diff.read_failed", evidence.StatusUnknown)
}

func mustSnapshot(t *testing.T, source string, content []byte) moduleSnapshot {
	t.Helper()
	snapshot, err := parseSnapshot(source, content)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = filepath.Clean(dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
