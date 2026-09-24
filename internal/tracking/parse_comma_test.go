package tracking

import (
	"slices"
	"testing"
)

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"Bash", []string{"Bash"}},
		{"Bash,Edit", []string{"Bash", "Edit"}},
		{" Bash , Edit\t,\r\nRead ", []string{"Bash", "Edit", "Read"}},
		{"a,,b,", []string{"a", "b"}},
		{" , \t ", nil},
	}
	for _, tt := range tests {
		if got := parseCommaSeparated(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("parseCommaSeparated(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
