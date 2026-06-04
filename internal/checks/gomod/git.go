package gomod

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func readGoModAtRef(ctx context.Context, opts Options, ref string) ([]byte, string, error) {
	workDir := defaultWorkDir(opts.WorkDir)
	repoRoot, relGoMod, err := gitGoModPath(ctx, workDir)
	if err != nil {
		return nil, "", err
	}

	object := ref + ":" + relGoMod
	data, err := gitOutput(ctx, repoRoot, "show", object)
	if err != nil {
		return nil, "", err
	}

	return data, object, nil
}

func gitGoModPath(ctx context.Context, workDir string) (string, string, error) {
	rootData, err := gitOutput(ctx, workDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", err
	}

	repoRoot, err := canonicalPath(strings.TrimSpace(string(rootData)))
	if err != nil {
		return "", "", err
	}
	absWorkDir, err := canonicalPath(workDir)
	if err != nil {
		return "", "", err
	}

	relWorkDir, err := filepath.Rel(repoRoot, absWorkDir)
	if err != nil {
		return "", "", err
	}
	if relWorkDir == ".." || strings.HasPrefix(relWorkDir, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("workdir %q is outside git root %q", absWorkDir, repoRoot)
	}

	relGoMod := "go.mod"
	if relWorkDir != "." {
		relGoMod = filepath.Join(relWorkDir, "go.mod")
	}

	return repoRoot, filepath.ToSlash(relGoMod), nil
}

func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}

	return out, nil
}
