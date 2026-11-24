package di

import (
	"fmt"
	"maps"
	"slices"

	"github.com/gendi-org/gendi/srcloc"
)

// Pass is a compiler pass that transforms config before validation and generation.
// Passes mutate the config and return it for chaining.
type Pass interface {
	Name() string
	Process(cfg *Config) (*Config, error)
}

// ApplyPasses applies compiler passes sequentially to the config.
// Each pass receives the result of the previous pass.
func ApplyPasses(cfg *Config, passes []Pass) (*Config, error) {
	result := cfg
	for _, pass := range passes {
		transformed, err := pass.Process(result)
		if err != nil {
			return nil, fmt.Errorf("pass %q failed: %w", pass.Name(), err)
		}
		result = transformed
	}
	return result, nil
}

// ApplyInternalPasses applies mandatory internal transformation passes.
// These passes desugar high-level config constructs (like decorators) into
// simpler primitives before IR building.
func ApplyInternalPasses(cfg *Config) (*Config, error) {
	return ApplyPasses(cfg, []Pass{
		&DecoratorPass{},
	})
}

// Config is the root configuration for the DI container.
// This is a resolved configuration with no import directives.
type Config struct {
	Parameters map[string]Parameter
	Tags       map[string]Tag
	Services   map[string]Service
}

func NewConfig() *Config {
	return &Config{
		Parameters: make(map[string]Parameter),
		Tags:       make(map[string]Tag),
		Services:   make(map[string]Service),
	}
}

// Clone returns a copy of the config that can be transformed by passes
// without affecting the receiver. Slices holding per-service state
// (constructor args, tags) are cloned; their elements are treated as
// immutable by passes, which replace entries wholesale. Map argument entries
// are the exception: passes rewrite nested arguments in place, so the entries
// slice is cloned too.
func (cfg *Config) Clone() *Config {
	result := NewConfig()
	maps.Copy(result.Parameters, cfg.Parameters)
	maps.Copy(result.Tags, cfg.Tags)
	for k, v := range cfg.Services {
		v.Constructor.Args = slices.Clone(v.Constructor.Args)
		for i := range v.Constructor.Args {
			v.Constructor.Args[i].Entries = slices.Clone(v.Constructor.Args[i].Entries)
		}
		v.Tags = slices.Clone(v.Tags)
		result.Services[k] = v
	}
	return result
}

// MergeWith merges src into cfg and returns cfg.
func (cfg *Config) MergeWith(src *Config) *Config {
	if src == nil {
		return cfg
	}

	maps.Copy(cfg.Parameters, src.Parameters)
	maps.Copy(cfg.Tags, src.Tags)
	maps.Copy(cfg.Services, src.Services)
	return cfg
}

// Parameter defines a parameter default value. Its target type is
// contextual: it comes from each constructor argument the parameter is
// injected into, not from the declaration.
type Parameter struct {
	Value Literal

	// Source location (optional)
	SourceLoc *srcloc.Location
}

// Tag defines a tag declaration.
type Tag struct {
	ElementType   string
	SortBy        string
	Public        bool
	Autoconfigure bool
	Packages      []string

	// Source location (optional)
	SourceLoc *srcloc.Location
}

// ServiceTag defines a tag assigned to a service.
type ServiceTag struct {
	Name       string
	Attributes map[string]any

	// Source location (optional)
	SourceLoc *srcloc.Location
}

// Service defines a service entry.
type Service struct {
	Type               string
	Constructor        Constructor
	Shared             bool
	Public             bool
	Autoconfigure      bool
	Decorates          string
	DecorationPriority int
	Tags               []ServiceTag
	Alias              string
	Packages           []string

	// Source location (optional)
	SourceLoc *srcloc.Location
}

// Constructor defines service constructor configuration.
type Constructor struct {
	Func     string
	Method   string
	Args     []Argument
	Packages []string

	// Source locations (optional)
	SourceLoc *srcloc.Location
}

// ArgumentKind is the parsed kind of a constructor argument.
type ArgumentKind int

const (
	ArgLiteral ArgumentKind = iota
	ArgServiceRef
	ArgInner
	ArgParam
	ArgTagged
	ArgSpread
	ArgGoRef
	ArgFieldAccessService
	ArgFieldAccessGo
	ArgMap
)

// Argument represents a constructor argument.
type Argument struct {
	Kind     ArgumentKind
	Value    string
	Literal  Literal
	Packages []string

	// Entries holds the key/value pairs of a map argument (ArgMap).
	Entries []ArgEntry

	// Source location (optional)
	SourceLoc *srcloc.Location
}

// ArgEntry is one key/value pair of a map argument. The key is always a
// literal; the value is any argument form allowed inside a map, which is
// exactly one level deep.
type ArgEntry struct {
	Key       Literal
	Value     Argument
	SourceLoc *srcloc.Location
}

// Children returns pointers to the arguments nested inside a, so passes can
// both read and rewrite them. It is the single description of a composite
// argument's shape at config level.
func (a *Argument) Children() []*Argument {
	if a == nil || a.Kind != ArgMap {
		return nil
	}
	children := make([]*Argument, 0, len(a.Entries))
	for i := range a.Entries {
		children = append(children, &a.Entries[i].Value)
	}
	return children
}
