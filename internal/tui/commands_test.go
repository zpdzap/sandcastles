package tui

import "testing"

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantArgs []string
		wantNil  bool
	}{
		{"/start foo", "/start", []string{"foo"}, false},
		{"/start foo fix the login", "/start", []string{"foo", "fix", "the", "login"}, false},
		{"/stop foo", "/stop", []string{"foo"}, false},
		{"/connect foo", "/connect", []string{"foo"}, false},
		{"/quit", "/quit", nil, false},
		{"/diff foo", "/diff", []string{"foo"}, false},
		{"not a command", "", nil, true},
		{"", "", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd := ParseCommand(tt.input)
			if tt.wantNil {
				if cmd != nil {
					t.Errorf("expected nil, got %+v", cmd)
				}
				return
			}
			if cmd == nil {
				t.Fatal("expected command, got nil")
			}
			if cmd.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", cmd.Name, tt.wantName)
			}
			if len(cmd.Args) == 0 && len(tt.wantArgs) == 0 {
				return
			}
			if len(cmd.Args) != len(tt.wantArgs) {
				t.Errorf("Args = %v, want %v", cmd.Args, tt.wantArgs)
			}
		})
	}
}
