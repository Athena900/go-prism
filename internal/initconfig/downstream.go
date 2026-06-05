package initconfig

import (
	"fmt"
	"strings"
)

const defaultDownstreamCommand = "go test ./..."

// DownstreamInput describes one downstream canary requested by init flags.
type DownstreamInput struct {
	Name string
	Path string
}

// ParseDownstream parses a repeated --downstream value in name=path form.
func ParseDownstream(value string) (DownstreamInput, error) {
	name, path, ok := strings.Cut(value, "=")
	if !ok {
		return DownstreamInput{}, newError(2, "--downstream must use name=path")
	}

	input := DownstreamInput{
		Name: strings.TrimSpace(name),
		Path: strings.TrimSpace(path),
	}
	if input.Name == "" {
		return DownstreamInput{}, newError(2, "--downstream name is required")
	}
	if input.Path == "" {
		return DownstreamInput{}, newError(2, "--downstream path is required")
	}
	return input, nil
}

// ParseDownstreams parses multiple --downstream values.
func ParseDownstreams(values []string) ([]DownstreamInput, error) {
	inputs := make([]DownstreamInput, 0, len(values))
	for _, value := range values {
		input, err := ParseDownstream(value)
		if err != nil {
			return nil, fmt.Errorf("parse downstream %q: %w", value, err)
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}
