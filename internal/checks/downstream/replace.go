package downstream

import (
	"errors"
	"os"
	"path/filepath"
)

type moduleBackup struct {
	goMod      fileBackup
	goSum      fileBackup
	modulePath string
}

type fileBackup struct {
	path   string
	data   []byte
	exists bool
	mode   os.FileMode
}

func backupModuleFiles(modulePath string) (moduleBackup, error) {
	goMod, err := backupFile(filepath.Join(modulePath, "go.mod"))
	if err != nil {
		return moduleBackup{}, err
	}
	goSum, err := backupFile(filepath.Join(modulePath, "go.sum"))
	if err != nil {
		return moduleBackup{}, err
	}
	return moduleBackup{
		goMod:      goMod,
		goSum:      goSum,
		modulePath: modulePath,
	}, nil
}

func backupFile(path string) (fileBackup, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileBackup{path: path}, nil
		}
		return fileBackup{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fileBackup{}, err
	}
	return fileBackup{
		path:   path,
		data:   data,
		exists: true,
		mode:   info.Mode().Perm(),
	}, nil
}

func (b moduleBackup) restore() error {
	if err := b.goMod.restore(); err != nil {
		return err
	}
	return b.goSum.restore()
}

func (b fileBackup) restore() error {
	if !b.exists {
		if err := os.Remove(b.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return os.WriteFile(b.path, b.data, b.mode)
}
