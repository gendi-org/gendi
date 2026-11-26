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
			case ServiceRefArg, TaggedArg:
				return arg.Service == nil || yield(arg.Service)
			case SpreadArg:
				return visitArgument(arg.Inner)
			case FieldAccessArg:
				return arg.FieldAccess == nil || arg.FieldAccess.Service == nil || yield(arg.FieldAccess.Service)
			default:
				return true
			}
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

	// Value based on kind
	Service     *Service     // For ServiceRef; for Tagged, the sequence of the tag after desugar
	Parameter   *Parameter   // For ParamRef
	Tag         *Tag         // For Tagged
	TagShape    TagShape     // For Tagged: the shape that the parameter type selects
	Literal     LiteralValue // For Literal
	Inner       *Argument    // For Spread (wraps another argument)
	GoRef       *GoRef       // For GoRef
	FieldAccess *FieldAccess // For FieldAccess
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
	// StringsLiteral is a []string value. Only the tag desugar phase makes
	// it, for the keys of a keyed tagged collection; YAML has no such literal.
	StringsLiteral
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
	// Entries are the entries of this tag on services, one for each tag
	// entry: a service that declares the tag twice has two entries. The
	// desugar phase links them.
	Entries []*ServiceTag
}

// ServiceTag is a tag attached to a service.
type ServiceTag struct {
	Tag *Tag
	// Service is the service that carries the entry.
	Service    *Service
	Attributes map[string]any
	// SortKey is the integer value of the attribute that Tag.SortBy names.
	// It is 0 when the attribute is absent.
	SortKey int
	// Key is the key of the service in a keyed collection of the tag: the
	// di.TagAttributeID attribute, or the service ID without it.
	Key string
	// SourceLoc is the location of the tag entry in the config. A tag that
	// moved from a decorated service keeps the location on that service. It
	// is nil for a tag that autoconfigure adds.
	SourceLoc *srcloc.Location
}
