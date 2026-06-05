package redact

import "regexp"

type pattern struct {
	value       *regexp.Regexp
	replacement string
}

var sensitivePatterns = []pattern{
	{
		value:       regexp.MustCompile(`(?i)(token|secret|password|passwd|api[_-]?key)=\S+`),
		replacement: "$1=[REDACTED]",
	},
	{
		value:       regexp.MustCompile(`(?i)Bearer\s+\S+`),
		replacement: "Bearer [REDACTED]",
	},
}

// Sensitive redacts common secret shapes from one line of external tool output.
func Sensitive(line string) string {
	redacted := line
	for _, pattern := range sensitivePatterns {
		redacted = pattern.value.ReplaceAllString(redacted, pattern.replacement)
	}
	return redacted
}
