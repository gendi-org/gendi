package stdlib

import (
	"reflect"
	"testing"

	di "github.com/gendi-org/gendi"
)

func TestSLogPassIsDeterministic(t *testing.T) {
	// "a" adds the service "a.logger", which also exists in the input with
	// its own slog tag. The result must not depend on which of the two
	// services the pass visits first.
	newConfig := func() *di.Config {
		taggedService := func(channel string) di.Service {
			return di.Service{
				Constructor: di.Constructor{
					Func: "app.New",
					Args: []di.Argument{{Kind: di.ArgServiceRef, Value: "logger"}},
				},
				Tags: []di.ServiceTag{{Name: "slog", Attributes: map[string]any{"channel": channel}}},
			}
		}
		return &di.Config{
			Services: map[string]di.Service{
				"a":        taggedService("first"),
				"a.logger": taggedService("second"),
				"b":        taggedService("third"),
			},
		}
	}

	want, err := (&SLogPass{}).Process(newConfig())
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	for range 50 {
		got, err := (&SLogPass{}).Process(newConfig())
		if err != nil {
			t.Fatalf("process: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("result differs between runs:\n got: %#v\nwant: %#v", got.Services, want.Services)
		}
	}
}
