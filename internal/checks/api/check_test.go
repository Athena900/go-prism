package api

import (
	"context"
	"errors"
	"testing"

	"github.com/Athena900/go-prism/internal/evidence"
)

func TestCheckWithAdaptersMissingGoreleaseIsUnknown(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GoreleaseAdapter{}},
		fakeResolver{err: errors.New("not found")},
	)

	assertHasStatus(t, items, "api.gorelease.not_installed", evidence.StatusUnknown)
}

func TestCheckWithAdaptersFoundGoreleaseStillUnknownUntilRunnerExists(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		[]Adapter{GoreleaseAdapter{}},
		fakeResolver{path: "/usr/local/bin/gorelease"},
	)

	assertHasStatus(t, items, "api.gorelease.execution_pending", evidence.StatusUnknown)
}

func TestCheckWithAdaptersEmptyAdapterSetIsUnknown(t *testing.T) {
	items := CheckWithAdapters(
		context.Background(),
		Options{WorkDir: ".", Base: "origin/main", Head: "HEAD"},
		nil,
		fakeResolver{},
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
		fakeResolver{path: "/usr/local/bin/gorelease"},
	)

	assertHasStatus(t, items, "api.timeout", evidence.StatusUnknown)
}

type fakeResolver struct {
	path string
	err  error
}

func (r fakeResolver) LookPath(string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.path, nil
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
