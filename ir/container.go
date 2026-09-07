package ir

import (
	"go/types"
	"iter"
	"slices"

	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/xmaps"
)

// Container is the fully resolved intermediate representation of a DI container.
type Container struct {
	Services   map[string]*Service
	Parameters map[string]*Parameter
	tags       map[string]*Tag
}

func NewContainer() *Container {
	return &Container{
		Services:   make(map[string]*Service),
		Parameters: make(map[string]*Parameter),
		tags:       make(map[string]*Tag),
	}
}

func (c *Container) ServiceIDsPostOrder() []string {
	result := make([]string, 0, len(c.Services))
	for svc := range c.ServicesPostOrder() {
		result = append(result, svc.ID)
	}
	return result
}

// ServicesPostOrder returns an iterator that yields services in post-order
// (dependencies before dependents). This is useful for operations that need
// to process dependencies before their dependents.
func (c *Container) ServicesPostOrder() iter.Seq[*Service] {
	return func(yield func(*Service) bool) {
		visited := make(map[string]bool)
		var visit func(*Service) bool
		visit = func(svc *Service) bool {
			if svc == nil || visited[svc.ID] {
				return true
			}
			visited[svc.ID] = true

			// Visit dependencies first
			for _, dep := range svc.Dependencies {
				if !visit(dep) {
					return false
				}
			}

			// Yield this service after its dependencies
			return yield(svc)
		}

		for _, id := range xmaps.OrderedKeys(c.Services) {
			if !visit(c.Services[id]) {
				return
			}
		}
	}
}

// Service is a fully resolved service definition.
type Service struct {
	ID   string
	Type types.Type

	// Construction
	Constructor *Constructor
	Alias       *Service // If this is an alias, points to target service

	// Lifecycle
	Shared        bool
	Public        bool
	Autoconfigure bool

	// Tags
	Tags []*ServiceTag

	// Computed
	Dependencies []*Service // Direct dependencies (resolved)
}

// IsAlias returns true if this service is an alias.
func (s *Service) IsAlias() bool {
	return s.Alias != nil
}

func (s *Service) Clone() *Service {
	result := *s
	if s.Constructor != nil {
		result.Constructor = s.Constructor.Clone()
	}
	result.Tags = slices.Clone(s.Tags)
	result.Dependencies = slices.Clone(s.Dependencies)

	return &result
}

// dependencyRefs returns every service reference used to construct the service.
// Unlike Dependencies, repeated references are yielded repeatedly.
func (s *Service) dependencyRefs() iter.Seq[*Service] {
	return func(yield func(*Service) bool) {
		if s.IsAlias() {
			yield(s.Alias)
			return
		}
		if s.Constructor == nil {
			// Dependencies may be populated directly by tests or other IR producers.
			for _, dependency := range s.Dependencies {
				if dependency != nil && !yield(dependency) {
					return
				}
			}
			return
		}

		if s.Constructor.Kind == MethodConstructor && s.Constructor.Receiver != nil {
			if !yield(s.Constructor.Receiver) {
				return
			}
		}

		var visitArgument func(*Argument) bool
		visitArgument = func(arg *Argument) bool {
			if arg == nil {
				return true
			}
			switch arg.Kind {
			case ServiceRefArg:
				if arg.Service != nil && !yield(arg.Service) {
					return false
				}
			case FieldAccessArg:
				if arg.FieldAccess != nil && arg.FieldAccess.Service != nil && !yield(arg.FieldAccess.Service) {
					return false
				}
			}
			for _, child := range arg.Children() {
				if !visitArgument(child) {
					return false
				}
			}
			return true
		}

		for _, arg := range s.Constructor.Args {
			if !visitArgument(arg) {
				return
			}
		}
	}
}

func (s *Service) dependencyRefCount(dependencyID string) int {
	count := 0
	for dependency := range s.dependencyRefs() {
		if dependency.ID == dependencyID {
			count++
		}
	}
	return count
}

// Constructor defines how a service is constructed.
type Constructor struct {
	Kind ConstructorKind
	Func *types.Func // For FuncConstructor
	Args []*Argument

	// For method constructors
	Receiver *Service // The service whose method is called

	// For generic constructors
	TypeArgs []types.Type // Resolved type arguments for generic functions

	// Signature info
	Params       []types.Type
	ResultType   types.Type
	ReturnsError bool
	Variadic     bool // True if function/method has variadic parameters
}

func (c *Constructor) Clone() *Constructor {
	result := *c

	if len(c.Args) > 0 {
		result.Args = make([]*Argument, len(c.Args))
		for i, arg := range c.Args {
			if arg == nil {
				continue
			}

			argClone := *arg
			if len(arg.Entries) > 0 {
				argClone.Entries = make([]MapEntry, len(arg.Entries))
				for j, entry := range arg.Entries {
					entryClone := entry
					valueClone := *entry.Value
					entryClone.Value = &valueClone
					argClone.Entries[j] = entryClone
				}
			}
			result.Args[i] = &argClone
		}
	}

	result.Params = slices.Clone(c.Params)
	result.TypeArgs = slices.Clone(c.TypeArgs)

	return &result
}

// ConstructorKind indicates the type of constructor.
type ConstructorKind int

const (
	FuncConstructor ConstructorKind = iota
	MethodConstructor
)

// Argument is a resolved constructor argument.
type Argument struct {
	Kind ArgumentKind
	Type types.Type // Expected parameter type

	// SourceLoc is the location of the di.Argument this was resolved from, so
	// a validation error caught after resolution (e.g. a type mismatch found
	// while walking Children()) can still report where the offending YAML
	// node was, rather than the location of whatever it references.
	SourceLoc *srcloc.Location

	// Value based on kind
	Service     *Service     // For ServiceRef
	Parameter   *Parameter   // For ParamRef
	Tag         *Tag         // For Tagged
	Literal     LiteralValue // For Literal
	Inner       *Argument    // For Spread (wraps another argument)
	GoRef       *GoRef       // For GoRef
	FieldAccess *FieldAccess // For FieldAccess
	Entries     []MapEntry   // For Map
}

// MapEntry is one resolved key/value pair of a map argument.
type MapEntry struct {
	Key   LiteralValue
	Value *Argument
}

// Children returns the arguments nested inside a. It is the single place that
// describes the shape of a composite argument: every walker that has to see
// nested arguments goes through it instead of switching on Kind itself.
func (a *Argument) Children() []*Argument {
	if a == nil {
		return nil
	}
	switch a.Kind {
	case SpreadArg:
		if a.Inner != nil {
			return []*Argument{a.Inner}
		}
	case MapArg:
		children := make([]*Argument, 0, len(a.Entries))
		for _, entry := range a.Entries {
			children = append(children, entry.Value)
		}
		return children
	}
	return nil
}

// GoRef holds a reference to a package-level variable or constant.
type GoRef struct {
	Object types.Object // *types.Var or *types.Const
}

// FieldAccess holds a field access expression on a service or Go symbol.
type FieldAccess struct {
	Service    *Service   // Non-nil for @service targets
	GoRef      *GoRef     // Non-nil for !go: targets
	FieldNames []string   // Field chain, e.g. ["Database", "DSN"]
	ResultType types.Type // Type of the final field
}

// ArgumentKind indicates the type of argument.
type ArgumentKind int

const (
	LiteralArg ArgumentKind = iota
	ServiceRefArg
	ParamRefArg
	TaggedArg
	SpreadArg
	GoRefArg
	FieldAccessArg
	MapArg
)

// LiteralValue holds a typed literal value.
type LiteralValue struct {
	Type  LiteralType
	Value any // string, int64, float64, bool, or nil
}

// LiteralType indicates the type of literal.
type LiteralType int

const (
	StringLiteral LiteralType = iota
	IntLiteral
	FloatLiteral
	BoolLiteral
	NullLiteral
	DurationLiteral
)

// Parameter is a parameter referenced by at least one constructor argument.
// Its conversion target is contextual (per injection site), so the IR keeps
// only the name.
type Parameter struct {
	Name string
}

// Tag is a resolved tag definition.
type Tag struct {
	Name          string
	ElementType   types.Type
	SortBy        string
	Public        bool
	Autoconfigure bool
	Services      []*Service // Services with this tag (sorted by priority)
}

// ServiceTag is a tag attached to a service.
type ServiceTag struct {
	Tag        *Tag
	Attributes map[string]any
}
