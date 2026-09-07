package ir

import (
	"go/types"
	"testing"

	di "github.com/gendi-org/gendi"
)

func TestUnusedParamPruneKeepsParamsUsedInsideMap(t *testing.T) {
	container := NewContainer()
	param := &Parameter{Name: "api_path"}
	container.Parameters["api_path"] = param
	container.Services["router"] = &Service{
		ID: "router",
		Constructor: &Constructor{
			Args: []*Argument{{
				Kind: MapArg,
				Type: types.NewMap(types.Typ[types.String], types.Typ[types.String]),
				Entries: []MapEntry{{
					Key:   LiteralValue{Type: StringLiteral, Value: "/api"},
					Value: &Argument{Kind: ParamRefArg, Parameter: param, Type: types.Typ[types.String]},
				}},
			}},
		},
	}
	cfg := di.NewConfig()
	cfg.Parameters["api_path"] = di.Parameter{Value: di.NewStringLiteral("/v1")}

	if err := (&unusedParamPrunePhase{}).Apply(cfg, container); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, ok := container.Parameters["api_path"]; !ok {
		t.Error("parameter used inside a map argument was pruned from the IR")
	}
	if _, ok := cfg.Parameters["api_path"]; !ok {
		t.Error("parameter used inside a map argument was pruned from the config")
	}
}
