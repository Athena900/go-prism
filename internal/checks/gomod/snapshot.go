package gomod

import (
	"fmt"
	"sort"

	"golang.org/x/mod/modfile"
)

type moduleSnapshot struct {
	source       string
	modulePath   string
	goVersion    string
	toolchain    string
	requirements map[string]requirementSnapshot
	replacements map[string]replacementSnapshot
	retractions  map[string]string
}

type requirementSnapshot struct {
	path     string
	version  string
	indirect bool
}

type replacementSnapshot struct {
	oldPath    string
	oldVersion string
	newPath    string
	newVersion string
}

func parseSnapshot(source string, data []byte) (moduleSnapshot, error) {
	file, err := modfile.Parse(source, data, nil)
	if err != nil {
		return moduleSnapshot{}, err
	}

	snapshot := moduleSnapshot{
		source:       source,
		requirements: make(map[string]requirementSnapshot),
		replacements: make(map[string]replacementSnapshot),
		retractions:  make(map[string]string),
	}

	if file.Module != nil {
		snapshot.modulePath = file.Module.Mod.Path
	}
	if file.Go != nil {
		snapshot.goVersion = file.Go.Version
	}
	if file.Toolchain != nil {
		snapshot.toolchain = file.Toolchain.Name
	}

	for _, req := range file.Require {
		snapshot.requirements[req.Mod.Path] = requirementSnapshot{
			path:     req.Mod.Path,
			version:  req.Mod.Version,
			indirect: req.Indirect,
		}
	}

	for _, replace := range file.Replace {
		next := replacementSnapshot{
			oldPath:    replace.Old.Path,
			oldVersion: replace.Old.Version,
			newPath:    replace.New.Path,
			newVersion: replace.New.Version,
		}
		snapshot.replacements[next.key()] = next
	}

	for _, retract := range file.Retract {
		key := formatRetract(retract)
		snapshot.retractions[key] = key
	}

	return snapshot, nil
}

func (r requirementSnapshot) format() string {
	value := r.path
	if r.version != "" {
		value += "@" + r.version
	}
	if r.indirect {
		value += " (indirect)"
	}
	return value
}

func (r replacementSnapshot) key() string {
	return formatModuleVersion(r.oldPath, r.oldVersion)
}

func (r replacementSnapshot) value() string {
	return formatModuleVersion(r.newPath, r.newVersion)
}

func (r replacementSnapshot) format() string {
	return fmt.Sprintf("%s => %s", r.key(), r.value())
}

func formatModuleVersion(path string, version string) string {
	if version == "" {
		return path
	}
	return path + "@" + version
}

func formatRetract(retract *modfile.Retract) string {
	if retract.Low == retract.High {
		return retract.Low
	}
	return fmt.Sprintf("[%s, %s]", retract.Low, retract.High)
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
