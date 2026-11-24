package ir

import "testing"

func TestValidatorPhaseCycleErrorIsDeterministic(t *testing.T) {
	tests := []struct {
		name    string
		deps    map[string][]string
		wantErr string
	}{
		{
			name: "one cycle starts at the lowest ID",
			deps: map[string][]string{
				"aaa": {"ccc"},
				"bbb": {"aaa"},
				"ccc": {"bbb"},
			},
			wantErr: "circular dependency: aaa -> ccc -> bbb -> aaa",
		},
		{
			name: "the cycle with the lowest ID is reported first",
			deps: map[string][]string{
				"xxx": {"yyy"},
				"yyy": {"xxx"},
				"mmm": {"nnn"},
				"nnn": {"mmm"},
			},
			wantErr: "circular dependency: mmm -> nnn -> mmm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := NewContainer()
			for id := range tt.deps {
				container.Services[id] = &Service{ID: id}
			}
			for id, deps := range tt.deps {
				for _, dep := range deps {
					svc := container.Services[id]
					svc.Dependencies = append(svc.Dependencies, container.Services[dep])
				}
			}

			// Go randomizes the map order on each range loop, so one run
			// cannot show that the order does not reach the message.
			for range 50 {
				err := (&validatorPhase{}).detectCycles(container)
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}
