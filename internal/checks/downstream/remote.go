package downstream

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
)

func runRemoteModule(ctx context.Context, opts Options, module Module, tools command.Runner) evidence.Item {
	target, cleanup, item := prepareRemoteModuleTarget(ctx, opts, module, tools)
	if item != nil {
		return *item
	}
	cleanupRemote := func() error {
		if cleanup == nil {
			return nil
		}
		return cleanup()
	}

	goPath, err := tools.LookPath("go")
	if err != nil {
		if cleanupErr := cleanupRemote(); cleanupErr != nil {
			return remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
		}
		return remoteConfigFailedEvidence(opts, module, fmt.Errorf("detect go: %w", err))
	}

	replaceArg := fmt.Sprintf("-replace=%s=%s", opts.ModulePath, target.HeadPath)
	editResult := tools.Run(ctx, command.Invocation{
		Path: goPath,
		Args: []string{"mod", "edit", replaceArg},
		Dir:  target.Path,
	})
	if ctx.Err() != nil {
		if cleanupErr := cleanupRemote(); cleanupErr != nil {
			return remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
		}
		return timeoutEvidence(opts, ctx.Err())
	}
	if editResult.Err != nil {
		if cleanupErr := cleanupRemote(); cleanupErr != nil {
			return remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
		}
		return replaceFailedEvidence(opts, module, target, editResult)
	}

	shellPath, err := tools.LookPath("sh")
	if err != nil {
		if cleanupErr := cleanupRemote(); cleanupErr != nil {
			return remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
		}
		return remoteConfigFailedEvidence(opts, module, fmt.Errorf("detect sh: %w", err))
	}

	result := tools.Run(ctx, command.Invocation{
		Path: shellPath,
		Args: []string{"-c", target.Command},
		Dir:  target.Path,
	})
	cleanupErr := cleanupRemote()
	if ctx.Err() != nil {
		if cleanupErr != nil {
			return remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
		}
		return timeoutEvidence(opts, ctx.Err())
	}
	if cleanupErr != nil {
		return remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
	}
	if result.Err != nil {
		return commandFailedEvidence(opts, module, target, result)
	}
	return passedEvidence(opts, module, target, result)
}

func prepareRemoteModuleTarget(ctx context.Context, opts Options, module Module, tools command.Runner) (moduleTarget, func() error, *evidence.Item) {
	target := moduleTarget{Name: module.Name}

	if err := validateRemoteModule(module); err != nil {
		item := remoteConfigFailedEvidence(opts, module, err)
		return target, nil, &item
	}

	headPath, err := resolvePath(".", defaultWorkDir(opts.WorkDir))
	if err != nil {
		item := remoteConfigFailedEvidence(opts, module, err)
		return target, nil, &item
	}
	target.HeadPath = headPath
	target.Command = remoteCommand(module)

	gitPath, err := tools.LookPath("git")
	if err != nil {
		item := remoteConfigFailedEvidence(opts, module, fmt.Errorf("detect git: %w", err))
		return target, nil, &item
	}

	tempRoot, err := os.MkdirTemp("", "go-prism-downstream-*")
	if err != nil {
		item := remoteConfigFailedEvidence(opts, module, fmt.Errorf("create temp clone dir: %w", err))
		return target, nil, &item
	}
	cleanup := func() error {
		return os.RemoveAll(tempRoot)
	}

	cloneDir := filepath.Join(tempRoot, "repo")
	target.Path = cloneDir
	cloneResult := tools.Run(ctx, command.Invocation{
		Path: gitPath,
		Args: []string{"clone", "--depth=1", strings.TrimSpace(module.Repo), cloneDir},
	})
	if ctx.Err() != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			item := remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
			return target, nil, &item
		}
		item := timeoutEvidence(opts, ctx.Err())
		return target, nil, &item
	}
	if cloneResult.Err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			item := remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
			return target, nil, &item
		}
		item := remoteCloneFailedEvidence(opts, module, target, cloneResult)
		return target, nil, &item
	}

	if strings.TrimSpace(module.Ref) != "" {
		fetchResult := tools.Run(ctx, command.Invocation{
			Path: gitPath,
			Args: []string{"-C", cloneDir, "fetch", "--depth=1", "origin", strings.TrimSpace(module.Ref)},
		})
		if ctx.Err() != nil {
			if cleanupErr := cleanup(); cleanupErr != nil {
				item := remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
				return target, nil, &item
			}
			item := timeoutEvidence(opts, ctx.Err())
			return target, nil, &item
		}
		if fetchResult.Err != nil {
			if cleanupErr := cleanup(); cleanupErr != nil {
				item := remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
				return target, nil, &item
			}
			item := remoteCheckoutFailedEvidence(opts, module, target, fetchResult)
			return target, nil, &item
		}

		checkoutResult := tools.Run(ctx, command.Invocation{
			Path: gitPath,
			Args: []string{"-C", cloneDir, "checkout", "--detach", "FETCH_HEAD"},
		})
		if ctx.Err() != nil {
			if cleanupErr := cleanup(); cleanupErr != nil {
				item := remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
				return target, nil, &item
			}
			item := timeoutEvidence(opts, ctx.Err())
			return target, nil, &item
		}
		if checkoutResult.Err != nil {
			if cleanupErr := cleanup(); cleanupErr != nil {
				item := remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
				return target, nil, &item
			}
			item := remoteCheckoutFailedEvidence(opts, module, target, checkoutResult)
			return target, nil, &item
		}
	}

	modulePath, err := remoteModulePath(cloneDir, module.Subdir)
	if err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			item := remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
			return target, nil, &item
		}
		item := remoteConfigFailedEvidence(opts, module, err)
		return target, nil, &item
	}
	target.Path = modulePath
	if _, err := os.Stat(filepath.Join(modulePath, "go.mod")); err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			item := remoteCleanupFailedEvidence(opts, module, target, cleanupErr)
			return target, nil, &item
		}
		item := remoteModuleMissingEvidence(opts, module, target, err)
		return target, nil, &item
	}

	return target, cleanup, nil
}

func validateRemoteModule(module Module) error {
	if strings.TrimSpace(module.Name) == "" {
		return errors.New("module name is required")
	}
	repo := strings.TrimSpace(module.Repo)
	if repo == "" {
		return errors.New("module repo is required")
	}
	parsed, err := url.Parse(repo)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return errors.New("remote downstream repo must use https")
	}
	if parsed.Host == "" {
		return errors.New("remote downstream repo host is required")
	}
	if parsed.User != nil {
		return errors.New("remote downstream repo must not include credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("remote downstream repo must not include query or fragment")
	}
	if err := validateRemoteModuleSubdir(module.Subdir); err != nil {
		return err
	}
	return nil
}

func validateRemoteModuleSubdir(raw string) error {
	_, err := cleanRemoteSubdir(raw)
	return err
}

func remoteModulePath(cloneDir string, rawSubdir string) (string, error) {
	subdir, err := cleanRemoteSubdir(rawSubdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(cloneDir, subdir), nil
}

func cleanRemoteSubdir(raw string) (string, error) {
	subdir := strings.TrimSpace(raw)
	if subdir == "" {
		return ".", nil
	}
	if filepath.IsAbs(subdir) {
		return "", errors.New("remote downstream subdir must be relative")
	}
	clean := filepath.Clean(subdir)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("remote downstream subdir must not escape the clone")
	}
	return clean, nil
}

func remoteCommand(module Module) string {
	commandText := strings.TrimSpace(module.Command)
	if commandText == "" {
		return defaultCommand
	}
	return commandText
}
