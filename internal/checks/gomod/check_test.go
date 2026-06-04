package gomod

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Athena900/go-prism/internal/evidence"
)

func TestCheckCurrentCleanModule(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "module example.com/project\n\ngo 1.22\n")

	items := CheckCurrent(context.Background(), Options{WorkDir: dir, Base: "main", Head: "HEAD"})
	assertHasStatus(t, items, "gomod.module_path", evidence.StatusInfo)
	assertHasStatus(t, items, "gomod.go_directive", evidence.StatusInfo)
	assertHasStatus(t, items, "gomod.replace_none", evidence.StatusPass)
}

func TestCheckCurrentReplaceWarns(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "module example.com/project\n\ngo 1.22\n\nreplace example.com/old => ../old\n")

	items := CheckCurrent(context.Background(), Options{WorkDir: dir})
	assertHasStatus(t, items, "gomod.replace_present", evidence.StatusWarn)
}

func TestCheckCurrentMissingGoModBlocks(t *testing.T) {
	items := CheckCurrent(context.Background(), Options{WorkDir: t.TempDir()})
	assertHasStatus(t, items, "gomod.read_failed", evidence.StatusBlock)
}

func writeGoMod(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
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
