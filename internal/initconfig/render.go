package initconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/Athena900/go-prism/internal/config"
)

const (
	FormatText = "text"
	FormatJSON = "json"
)

// Render serializes an init result.
func Render(result Result, format string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", FormatText:
		return renderText(result), nil
	case FormatJSON:
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(out, '\n'), nil
	default:
		return nil, fmt.Errorf("unsupported init format %q", format)
	}
}

// RenderYAML serializes generated config with stable, documented ordering.
func RenderYAML(cfg config.Config) string {
	var out bytes.Buffer
	fmt.Fprintf(&out, "module: %s\n\n", yamlScalar(cfg.Module))
	fmt.Fprintln(&out, "checks:")
	fmt.Fprintf(&out, "  gomod:\n    enabled: %t\n", cfg.Checks.GoMod.Enabled)
	fmt.Fprintf(&out, "  api:\n    enabled: %t\n", cfg.Checks.API.Enabled)
	fmt.Fprintf(&out, "  vuln:\n    enabled: %t\n", cfg.Checks.Vuln.Enabled)
	fmt.Fprintf(&out, "  downstream:\n    enabled: %t\n", cfg.Checks.Downstream.Enabled)
	if len(cfg.Checks.Downstream.Modules) == 0 {
		fmt.Fprintln(&out, "    modules: []")
	} else {
		fmt.Fprintln(&out, "    modules:")
		for _, module := range cfg.Checks.Downstream.Modules {
			fmt.Fprintf(&out, "      - name: %s\n", yamlScalar(module.Name))
			fmt.Fprintf(&out, "        path: %s\n", yamlScalar(module.Path))
			command := module.Command
			if command == "" {
				command = defaultDownstreamCommand
			}
			fmt.Fprintf(&out, "        command: %s\n", yamlScalar(command))
		}
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "policy:")
	fmt.Fprintln(&out, "  fail_on:")
	fmt.Fprintf(&out, "    gomod_parse_error: %t\n", cfg.Policy.FailOn["gomod_parse_error"])
	fmt.Fprintf(&out, "    new_replace_directive: %t\n", cfg.Policy.FailOn["new_replace_directive"])
	return out.String()
}

func renderText(result Result) []byte {
	var out bytes.Buffer
	if result.Status == StatusPreview {
		fmt.Fprintf(&out, "# go-prism init dry run: %s\n", result.Path)
		fmt.Fprint(&out, result.YAML)
		return out.Bytes()
	}

	if result.Status == StatusFailed {
		return out.Bytes()
	}

	action := "Created"
	if result.Overwritten {
		action = "Overwrote"
	}
	fmt.Fprintf(&out, "%s %s\n", action, result.Path)
	fmt.Fprintf(&out, "Module: %s\n", result.Module)
	fmt.Fprintf(&out, "Enabled checks: %s\n", strings.Join(result.EnabledChecks, ", "))
	if len(result.NextSteps) > 0 {
		fmt.Fprintln(&out)
		fmt.Fprintln(&out, "Next steps:")
		for _, step := range result.NextSteps {
			fmt.Fprintf(&out, "  - %s\n", step)
		}
	}
	return out.Bytes()
}

func yamlScalar(value string) string {
	if isPlainYAMLScalar(value) {
		return value
	}
	return strconv.Quote(value)
}

func isPlainYAMLScalar(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	lower := strings.ToLower(value)
	switch lower {
	case "true", "false", "null", "~", "yes", "no", "on", "off":
		return false
	}
	for i, r := range value {
		if r == '\n' || r == '\r' || r == '\t' || r == '#' {
			return false
		}
		if r == ':' && i+1 < len(value) && unicode.IsSpace(rune(value[i+1])) {
			return false
		}
	}
	return true
}
