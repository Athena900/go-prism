package downstream

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Athena900/go-prism/internal/command"
	"github.com/Athena900/go-prism/internal/evidence"
)

const defaultCommand = "go test ./..."

func runModule(ctx context.Context, opts Options, module Module, tools command.Runner) evidence.Item {
	target, err := prepareModuleTarget(opts, module)
	if err != nil {
		return configFailedEvidence(opts, module, err)
	}

	backup, err := backupModuleFiles(target.Path)
	if err != nil {
		return configFailedEvidence(opts, module, err)
	}

	replaced := false
	restore := func() error {
		if !replaced {
			return nil
		}
		return backup.restore()
	}

	goPath, err := tools.LookPath("go")
	if err != nil {
		return configFailedEvidence(opts, module, fmt.Errorf("detect go: %w", err))
	}

	replaceArg := fmt.Sprintf("-replace=%s=%s", opts.ModulePath, target.HeadPath)
	editResult := tools.Run(ctx, command.Invocation{
		Path: goPath,
		Args: []string{"mod", "edit", replaceArg},
		Dir:  target.Path,
	})
	if ctx.Err() != nil {
		_ = restore()
		return timeoutEvidence(opts, ctx.Err())
	}
	if editResult.Err != nil {
		_ = restore()
		return replaceFailedEvidence(opts, module, target, editResult)
	}
	replaced = true

	shellPath, err := tools.LookPath("sh")
	if err != nil {
		if restoreErr := restore(); restoreErr != nil {
			return restoreFailedEvidence(opts, module, target, restoreErr)
		}
		return configFailedEvidence(opts, module, fmt.Errorf("detect sh: %w", err))
	}

	commandText := target.Command
	result := tools.Run(ctx, command.Invocation{
		Path: shellPath,
		Args: []string{"-c", commandText},
		Dir:  target.Path,
	})
	restoreErr := restore()
	if ctx.Err() != nil {
		if restoreErr != nil {
			return restoreFailedEvidence(opts, module, target, restoreErr)
		}
		return timeoutEvidence(opts, ctx.Err())
	}
	if restoreErr != nil {
		return restoreFailedEvidence(opts, module, target, restoreErr)
	}
	if result.Err != nil {
		return commandFailedEvidence(opts, module, target, result)
	}
	return passedEvidence(opts, module, target, result)
}

type moduleTarget struct {
	Name     string
	Path     string
	Command  string
	HeadPath string
}

func prepareModuleTarget(opts Options, module Module) (moduleTarget, error) {
	if strings.TrimSpace(module.Name) == "" {
		return moduleTarget{}, fmt.Errorf("module name is required")
	}
	if strings.TrimSpace(module.Path) == "" {
		return moduleTarget{}, fmt.Errorf("module path is required")
	}

	targetPath, err := resolvePath(defaultWorkDir(opts.WorkDir), module.Path)
	if err != nil {
		return moduleTarget{}, err
	}
	if info, err := os.Stat(targetPath); err != nil {
		return moduleTarget{}, err
	} else if !info.IsDir() {
		return moduleTarget{}, fmt.Errorf("%s is not a directory", targetPath)
	}
	if _, err := os.Stat(filepath.Join(targetPath, "go.mod")); err != nil {
		return moduleTarget{}, fmt.Errorf("read downstream go.mod: %w", err)
	}

	headPath, err := resolvePath(".", defaultWorkDir(opts.WorkDir))
	if err != nil {
		return moduleTarget{}, err
	}

	commandText := strings.TrimSpace(module.Command)
	if commandText == "" {
		commandText = defaultCommand
	}

	return moduleTarget{
		Name:     module.Name,
		Path:     targetPath,
		Command:  commandText,
		HeadPath: headPath,
	}, nil
}

func resolvePath(base string, path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Abs(path)
	}
	return filepath.Abs(filepath.Join(base, path))
}

func defaultWorkDir(workDir string) string {
	if workDir == "" {
		return "."
	}
	return workDir
}
