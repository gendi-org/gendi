// Package yaml provides YAML parsing for DI configuration files.
package yaml

import (
	"fmt"
	"math"
	"strings"

	"github.com/goccy/go-yaml/ast"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/typeres"
)

// Parser converts raw YAML structures to di.Config.
type Parser struct{}

// NewParser creates a new YAML parser.
func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) ConvertConfigWithDirAndFile(raw *RawConfig, configDir string, filePath string) (*di.Config, error) {
	cfg := &di.Config{
		Parameters: make(map[string]di.Parameter),
		Tags:       make(map[string]di.Tag),
		Services:   make(map[string]di.Service),
	}

	// Resolve $this package path from config file directory
	var thisPackage string
	if configDir != "" {
		if pkg, err := resolvePackagePath(configDir); err == nil {
			thisPackage = pkg
		}
	}

	// Convert parameters
	for name, param := range raw.Parameters {
		if _, ok := param.Value.(*ast.MappingNode); ok {
			// A parameter's target type is contextual, so its default must be
			// a plain scalar; a mapping has no meaning here.
			return nil, srcloc.Errorf(newLocation(filePath, param.Node),
				"parameter %q: value must be a plain scalar, got a mapping", name)
		}
		if param.Value == nil {
			return nil, srcloc.Errorf(newLocation(filePath, param.Node), "parameter %q: null value is not supported", name)
		}
		lit, err := p.convertLiteral(param.Value, filePath)
		if err != nil {
			return nil, srcloc.AddContext(err, "parameter %q", name)
		}
		if lit.IsNull() {
			return nil, srcloc.Errorf(newLocation(filePath, param.Node), "parameter %q: null value is not supported", name)
		}
		cfg.Parameters[name] = di.Parameter{
			Value:     lit,
			SourceLoc: newLocation(filePath, param.Node),
		}
	}

	// Convert tags
	for name, tag := range raw.Tags {
		elementType := tag.ElementType
		// Substitute $this with the resolved package path
		if thisPackage != "" && strings.Contains(elementType, "$this.") {
			elementType = strings.ReplaceAll(elementType, "$this.", thisPackage+".")
		}
		cfg.Tags[name] = di.Tag{
			ElementType:   elementType,
			SortBy:        tag.SortBy,
			Public:        tag.Public,
			Autoconfigure: tag.Autoconfigure,
			Packages:      typeres.CollectTypePackages(elementType),
			SourceLoc:     newLocation(filePath, tag.Node),
		}
	}

	// Extract and parse _default if present
	var defaults *ServiceDefaults
	if defaultSvc, ok := raw.Services["_default"]; ok {
		defaults = &ServiceDefaults{
			Shared:        defaultSvc.Shared,
			Public:        defaultSvc.Public,
			Autoconfigure: defaultSvc.Autoconfigure,
		}
		// Validate that _default only contains allowed fields
		if err := p.validateDefaults(defaultSvc); err != nil {
			return nil, srcloc.AddContext(err, "_default")
		}
	}

	// Convert services
	for name, svc := range raw.Services {
		if name == "_default" {
			continue // Skip _default itself
		}
		converted, err := p.convertServiceWithPackageAndFile(svc, defaults, thisPackage, filePath)
		if err != nil {
			return nil, srcloc.AddContext(err, "service %q", name)
		}
		cfg.Services[name] = converted
	}

	return cfg, nil
}

func (p *Parser) convertServiceWithPackageAndFile(raw *RawService, defaults *ServiceDefaults, thisPackage string, filePath string) (di.Service, error) {
	aliasTarget := raw.Alias
	if IsServiceAlias(aliasTarget) {
		aliasTarget = ParseServiceAlias(aliasTarget)
	}
	if raw.Alias != "" && raw.Shared != nil {
		return di.Service{}, srcloc.Errorf(
			newLocation(filePath, raw.Node),
			"alias cannot define shared; lifecycle is inherited from target %q",
			aliasTarget,
		)
	}

	// Apply defaults if not explicitly set
	defaultShared := true
	if defaults != nil && defaults.Shared != nil {
		defaultShared = *defaults.Shared
	}

	shared := defaultShared
	if raw.Shared != nil {
		shared = *raw.Shared
	}

	defaultAutoconfigure := true
	if defaults != nil && defaults.Autoconfigure != nil {
		defaultAutoconfigure = *defaults.Autoconfigure
	}

	autoconfigure := defaultAutoconfigure
	if raw.Autoconfigure != nil {
		autoconfigure = *raw.Autoconfigure
	}

	public := raw.Public
	if public == nil && defaults != nil && defaults.Public != nil {
		public = defaults.Public
	}

	// Convert to non-pointer for di.Service
	publicBool := false
	if public != nil {
		publicBool = *public
	}

	svc := di.Service{
		Type:               raw.Type,
		Shared:             shared,
		Public:             publicBool,
		Autoconfigure:      autoconfigure,
		Decorates:          raw.Decorates,
		DecorationPriority: raw.DecorationPriority,
		SourceLoc:          newLocation(filePath, raw.Node),
	}

	if raw.Alias != "" {
		// Aliases have no lifecycle of their own and delegate to their target.
		svc.Shared = false
		svc.Alias = aliasTarget
	}

	// Convert tags
	svc.Tags = make([]di.ServiceTag, len(raw.Tags))
	for i, tag := range raw.Tags {
		svc.Tags[i] = di.ServiceTag{
			Name:       tag.Name,
			Attributes: tag.Attributes,
			SourceLoc:  newLocation(filePath, tag.Node),
		}
	}

	// Convert constructor
	svc.Constructor = di.Constructor{
		Func:      raw.Constructor.Func,
		Method:    raw.Constructor.Method,
		SourceLoc: newLocation(filePath, raw.Constructor.Node),
	}

	// Substitute $this with the resolved package path
	if thisPackage != "" {
		// Substitute in type field (can appear anywhere due to type prefixes like *, [], etc.)
		if strings.Contains(svc.Type, "$this.") {
			svc.Type = strings.ReplaceAll(svc.Type, "$this.", thisPackage+".")
		}
		// Substitute in the constructor func (at the start of the path and
		// anywhere within generic type arguments). The method field takes no
		// $this: it addresses a service (@svc.Method), never a package.
		svc.Constructor.Func = substituteThisInFuncRef(svc.Constructor.Func, thisPackage)
	}

	// Populate Packages after $this substitution
	svc.Packages = typeres.CollectTypePackages(svc.Type)
	svc.Constructor.Packages = typeres.CollectFuncPackages(svc.Constructor.Func)

	if len(raw.Constructor.Args) > 0 {
		svc.Constructor.Args = make([]di.Argument, len(raw.Constructor.Args))
		for i, arg := range raw.Constructor.Args {
			converted, err := p.convertArgumentWithFile(&arg, filePath)
			if err != nil {
				return di.Service{}, srcloc.AddContext(err, "arg[%d]", i)
			}
			// Substitute $this in !go: argument values
			if thisPackage != "" && converted.Kind == di.ArgGoRef && strings.Contains(converted.Value, "$this.") {
				converted.Value = strings.Replace(converted.Value, "$this.", thisPackage+".", 1)
			}
			// Substitute $this in !field:!go: argument values
			if thisPackage != "" && converted.Kind == di.ArgFieldAccessGo && strings.Contains(converted.Value, "$this.") {
				converted.Value = strings.Replace(converted.Value, "$this.", thisPackage+".", 1)
			}
			// Populate Packages after $this substitution
			switch converted.Kind {
			case di.ArgGoRef:
				converted.Packages = typeres.CollectGoRefPackages(converted.Value)
			case di.ArgFieldAccessGo:
				converted.Packages = typeres.CollectFieldAccessGoPackages(converted.Value)
			}
			svc.Constructor.Args[i] = converted
		}
	}

	return svc, nil
}

// substituteThisInFuncRef replaces the $this token in a constructor
// reference like "$this.NewPool[$this.Message]": at the start of the
// function path and anywhere within the generic type arguments, where any
// type expression may appear.
func substituteThisInFuncRef(ref, thisPackage string) string {
	base, typeArgs, hasTypeArgs := strings.Cut(ref, "[")
	if strings.HasPrefix(base, "$this.") {
		base = strings.Replace(base, "$this.", thisPackage+".", 1)
	}
	if !hasTypeArgs {
		return base
	}
	return base + "[" + strings.ReplaceAll(typeArgs, "$this.", thisPackage+".")
}

func (p *Parser) convertArgumentWithFile(raw *RawArgument, filePath string) (di.Argument, error) {
	loc := newLocation(filePath, raw.Node)

	if raw.Value != nil {
		kind, val, err := ParseArgumentString(*raw.Value)
		if err != nil {
			return di.Argument{}, srcloc.Errorf(loc, "%s", err)
		}
		if kind != di.ArgLiteral {
			return di.Argument{
				Kind:      kind,
				Value:     val,
				SourceLoc: loc,
			}, nil
		}
		return di.Argument{
			Kind:      di.ArgLiteral,
			Literal:   di.NewStringLiteral(*raw.Value),
			SourceLoc: loc,
		}, nil
	}

	if raw.Node != nil {
		lit, err := p.convertLiteral(raw.Node, filePath)
		if err != nil {
			return di.Argument{}, err
		}
		return di.Argument{
			Kind:      di.ArgLiteral,
			Literal:   lit,
			SourceLoc: loc,
		}, nil
	}

	return di.Argument{}, srcloc.Errorf(loc, "argument must have a value")
}

func (p *Parser) convertLiteral(node ast.Node, filePath string) (di.Literal, error) {
	loc := newLocation(filePath, node)

	switch n := node.(type) {
	case *ast.StringNode:
		return di.NewStringLiteral(n.Value), nil
	case *ast.LiteralNode:
		if n.Value == nil {
			return di.NewStringLiteral(""), nil
		}
		return di.NewStringLiteral(n.Value.Value), nil
	case *ast.IntegerNode:
		switch v := n.Value.(type) {
		case int64:
			return di.NewIntLiteral(v), nil
		case uint64:
			if v > math.MaxInt64 {
				return di.Literal{}, srcloc.Errorf(loc, "integer value %d does not fit in int64", v)
			}
			return di.NewIntLiteral(int64(v)), nil
		default:
			return di.Literal{}, srcloc.Errorf(loc, "unsupported integer kind %T", v)
		}
	case *ast.FloatNode:
		return di.NewFloatLiteral(n.Value), nil
	case *ast.InfinityNode:
		return di.Literal{}, srcloc.Errorf(loc, ".inf is not supported as a literal value")
	case *ast.NanNode:
		return di.Literal{}, srcloc.Errorf(loc, ".nan is not supported as a literal value")
	case *ast.BoolNode:
		return di.NewBoolLiteral(n.Value), nil
	case *ast.NullNode:
		return di.NewNullLiteral(), nil
	default:
		return di.Literal{}, srcloc.Errorf(loc, "unsupported literal type %T", node)
	}
}

// validateDefaults ensures _default only contains allowed fields (shared, public, autoconfigure)
func (p *Parser) validateDefaults(raw *RawService) error {
	if raw.Type != "" {
		return fmt.Errorf("field 'type' is not allowed in _default")
	}
	if raw.Constructor.Func != "" || raw.Constructor.Method != "" || len(raw.Constructor.Args) > 0 {
		return fmt.Errorf("field 'constructor' is not allowed in _default")
	}
	if raw.Alias != "" {
		return fmt.Errorf("field 'alias' is not allowed in _default")
	}
	if raw.Decorates != "" {
		return fmt.Errorf("field 'decorates' is not allowed in _default")
	}
	if raw.DecorationPriority != 0 {
		return fmt.Errorf("field 'decoration_priority' is not allowed in _default")
	}
	if len(raw.Tags) > 0 {
		return fmt.Errorf("field 'tags' is not allowed in _default")
	}
	// Only shared, public, and autoconfigure are allowed, which we already extracted
	return nil
}
