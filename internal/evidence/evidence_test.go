package evidence

import "testing"

func TestDecide(t *testing.T) {
	tests := []struct {
		name  string
		items []Item
		want  Status
	}{
		{name: "empty is pass", want: StatusPass},
		{
			name:  "warn beats unknown",
			items: []Item{{Status: StatusUnknown}, {Status: StatusWarn}},
			want:  StatusWarn,
		},
		{
			name:  "block wins",
			items: []Item{{Status: StatusWarn}, {Status: StatusBlock}},
			want:  StatusBlock,
		},
		{
			name:  "unknown when no warn or block",
			items: []Item{{Status: StatusPass}, {Status: StatusUnknown}},
			want:  StatusUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Decide(tt.items); got != tt.want {
				t.Fatalf("Decide() = %q, want %q", got, tt.want)
			}
		})
	}
}
