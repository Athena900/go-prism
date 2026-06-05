package vuln

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
	"github.com/Athena900/go-prism/internal/redact"
)

type govulncheckTarget struct {
	Label   string
	Ref     string
	WorkDir string
	Source  string
}

func prepareGovulncheckTarget(ctx context.Context, opts Options, ref string, label string, tools command.Runner) (govulncheckTarget, func(context.Context), error) {
	normalizedRef := strings.TrimSpace(ref)
	if normalizedRef == "" || normalizedRef == "HEAD" {
		return govulncheckTarget{
			Label:   label,
			Ref:     displayRef(normalizedRef),
			WorkDir: defaultWorkDir(opts.WorkDir),
			Source:  "worktree",
		}, nil, nil
	}

	gitPath, err := tools.LookPath("git")
	if err != nil {
		return govulncheckTarget{}, nil, fmt.Errorf("detect git: %w", err)
	}

	repo, err := inspectGitRepo(ctx, opts.WorkDir, gitPath, tools)
	if err != nil {
		return govulncheckTarget{}, nil, err
	}

	tempDir, err := os.MkdirTemp("", "go-prism-vuln-"+label+"-*")
	if err != nil {
		return govulncheckTarget{}, nil, err
	}

	worktreePath := filepath.Join(tempDir, "worktree")
	result := tools.Run(ctx, command.Invocation{
		Path: gitPath,
		Args: []string{"worktree", "add", "--detach", worktreePath, normalizedRef},
		Dir:  repo.Root,
	})
	if result.Err != nil {
		_ = os.RemoveAll(tempDir)
		return govulncheckTarget{}, nil, fmt.Errorf("git worktree add %s: %s", normalizedRef, commandFailure(result))
	}

	cleanup := func(cleanupCtx context.Context) {
		remove := tools.Run(cleanupCtx, command.Invocation{
			Path: gitPath,
			Args: []string{"worktree", "remove", "--force", worktreePath},
			Dir:  repo.Root,
		})
		if remove.Err != nil {
			_ = tools.Run(cleanupCtx, command.Invocation{
				Path: gitPath,
				Args: []string{"worktree", "prune"},
				Dir:  repo.Root,
			})
		}
		_ = os.RemoveAll(tempDir)
	}

	targetWorkDir := worktreePath
	if repo.RelWorkDir != "." {
		targetWorkDir = filepath.Join(worktreePath, filepath.FromSlash(repo.RelWorkDir))
	}

	return govulncheckTarget{
		Label:   label,
		Ref:     normalizedRef,
		WorkDir: targetWorkDir,
		Source:  normalizedRef,
	}, cleanup, nil
}

type gitRepo struct {
	Root       string
	RelWorkDir string
}

func inspectGitRepo(ctx context.Context, workDir string, gitPath string, tools command.Runner) (gitRepo, error) {
	rootResult := tools.Run(ctx, command.Invocation{
		Path: gitPath,
		Args: []string{"rev-parse", "--show-toplevel"},
		Dir:  defaultWorkDir(workDir),
	})
	if rootResult.Err != nil {
		return gitRepo{}, fmt.Errorf("git rev-parse --show-toplevel: %s", commandFailure(rootResult))
	}

	repoRoot, err := canonicalPath(strings.TrimSpace(rootResult.Stdout))
	if err != nil {
		return gitRepo{}, err
	}
	absWorkDir, err := canonicalPath(defaultWorkDir(workDir))
	if err != nil {
		return gitRepo{}, err
	}

	relWorkDir, err := filepath.Rel(repoRoot, absWorkDir)
	if err != nil {
		return gitRepo{}, err
	}
	if relWorkDir == ".." || strings.HasPrefix(relWorkDir, ".."+string(filepath.Separator)) {
		return gitRepo{}, fmt.Errorf("workdir %q is outside git root %q", absWorkDir, repoRoot)
	}

	return gitRepo{
		Root:       repoRoot,
		RelWorkDir: filepath.ToSlash(relWorkDir),
	}, nil
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

func targetOptions(opts Options, target govulncheckTarget) Options {
	opts.WorkDir = target.WorkDir
	return opts
}

func targetFailedEvidence(opts Options, label string, ref string, err error) evidence.Item {
	return evidence.Item{
		ID:             "vuln.govulncheck.ref_failed",
		Title:          "govulncheck ref preparation failed",
		Status:         evidence.StatusUnknown,
		Severity:       evidence.SeverityMedium,
		Category:       evidence.CategoryVuln,
		Source:         "govulncheck",
		Summary:        fmt.Sprintf("go-prism could not prepare the %s ref `%s` for govulncheck: %v.", label, displayRef(ref), err),
		Recommendation: "Confirm the ref exists locally and retry with a full fetch before trusting vulnerability delta evidence.",
		Provenance:     provenance(opts, "prepare govulncheck "+label+" ref", "git"),
	}
}

func commandFailure(result command.Result) string {
	details := boundedLines(result.Stderr+"\n"+result.Stdout, 3)
	if len(details) == 0 && result.Err != nil {
		return result.Err.Error()
	}
	for i, detail := range details {
		details[i] = redact.Sensitive(detail)
	}
	return strings.Join(details, "; ")
}

func displayRef(ref string) string {
	if strings.TrimSpace(ref) == "" {
		return "HEAD"
	}
	return strings.TrimSpace(ref)
}

func defaultWorkDir(workDir string) string {
	if workDir == "" {
		return "."
	}
	return workDir
}
