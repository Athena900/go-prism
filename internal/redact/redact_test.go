package redact

import (
	"strings"
	"testing"
)

func TestSensitiveRedactsCommonSecretShapes(t *testing.T) {
	out := Sensitive("token=abc123 api_key=def456 Bearer secret-token")

	for _, leaked := range []string{"abc123", "def456", "secret-token"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("Sensitive() leaked %q in %q", leaked, out)
		}
	}
	for _, want := range []string{"token=[REDACTED]", "api_key=[REDACTED]", "Bearer [REDACTED]"} {
		if !strings.Contains(out, want) {
			t.Fatalf("Sensitive() missing %q in %q", want, out)
		}
	}
}
