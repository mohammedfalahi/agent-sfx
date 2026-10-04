package installer_test

import (
	"testing"

	"agent-sfx/internal/installer"
)

func TestEscapeShellArg(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple path",
			input: "/usr/local/bin/agent-sfx",
			want:  "'/usr/local/bin/agent-sfx'",
		},
		{
			name:  "path with spaces",
			input: "/Users/my user/Applications/agent sfx/bin",
			want:  "'/Users/my user/Applications/agent sfx/bin'",
		},
		{
			name:  "path with double quotes",
			input: `/path/with"quotes"/bin`,
			want:  `'/path/with"quotes"/bin'`,
		},
		{
			name:  "path with single quotes",
			input: `/path/with'single'quotes/bin`,
			want:  `'/path/with'\''single'\''quotes/bin'`,
		},
		{
			name:  "path with dollar signs",
			input: `/path/with$VAR/and${ENV}/bin`,
			want:  `'/path/with$VAR/and${ENV}/bin'`,
		},
		{
			name:  "path with backticks",
			input: "/path/with`whoami`/bin",
			want:  "'/path/with`whoami`/bin'",
		},
		{
			name:  "path with backslashes",
			input: `C:\Users\name\bin\agent-sfx`,
			want:  `'C:\Users\name\bin\agent-sfx'`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := installer.EscapeShellArg(tt.input)
			if got != tt.want {
				t.Errorf("EscapeShellArg(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
