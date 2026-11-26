// Package yaml turns configuration files into a [di.Config]: parsing, import
// resolution, and the merge that lets a later file override an earlier one.
//
// Loading is where a configuration is confined. Each file's real path is
// checked against its boundary immediately before it is read, so a symlinked
// or imported file cannot pull in something outside the module it belongs to.
//
// Parse errors and validation errors both carry the position of the offending
// node, which is what lets the generator print the line and a caret rather
// than a bare message.
package yaml

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/goccy/go-yaml/ast"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/typeres"
	"github.com/gendi-org/gendi/xmaps"
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

	// Resolve $this package path from config file directory. A config
	// without $this needs no Go package, so the error is kept until a value
	// uses $this.
	this := thisPackage{err: errors.New("the directory of the config file is unknown")}
	if configDir != "" {
		this.path, this.err = resolvePackagePath(configDir)
	}

	// Convert parameters
	for _, name := range xmaps.OrderedKeys(raw.Parameters) {
		param := raw.Parameters[name]
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
	for _, name := range xmaps.OrderedKeys(raw.Tags) {
		tag := raw.Tags[name]
		elementType := tag.ElementType
		if err := this.check(elementType, newLocation(filePath, tag.Node), "element_type"); err != nil {
			return nil, srcloc.AddContext(err, "tag %q", name)
		}
		// Substitute $this with the resolved package path
		if this.path != "" && strings.Contains(elementType, "$this.") {
			elementType = strings.ReplaceAll(elementType, "$this.", this.path+".")
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

	// Convert services
	for _, name := range xmaps.OrderedKeys(raw.Services) {
		svc := raw.Services[name]
		converted, err := p.convertServiceWithPackageAndFile(svc, raw.Defaults, this, filePath)
		if err != nil {
			return nil, srcloc.AddContext(err, "service %q", name)
		}
		cfg.Services[name] = converted
	}

	return cfg, nil
}

func (p *Parser) convertServiceWithPackageAndFile(raw *RawService, defaults *ServiceDefaults, this thisPackage, filePath string) (di.Service, error) {
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
	if raw.Alias != "" && raw.Autoconfigure != nil {
		return di.Service{}, srcloc.Errorf(
			newLocation(filePath, raw.Node),
			"alias cannot define autoconfigure; autoconfigure applies to target %q",
			aliasTarget,
		)
	}
	// Symfony rejects the key on an alias, also with an empty list.
	if raw.Alias != "" && raw.TagsKey != nil {
		return di.Service{}, srcloc.Errorf(
			newLocation(filePath, raw.TagsKey),
			"alias cannot define tags; tag the target %q instead",
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
		// Autoconfigure also applies only to the target.
		svc.Shared = false
		svc.Autoconfigure = false
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

	if err := this.check(svc.Type, svc.SourceLoc, "type"); err != nil {
		return di.Service{}, err
	}
	if err := this.check(svc.Constructor.Func, svc.Constructor.SourceLoc, "constructor.func"); err != nil {
		return di.Service{}, err
	}

	// Substitute $this with the resolved package path
	if this.path != "" {
		// Substitute in type field (can appear anywhere due to type prefixes like *, [], etc.)
		if strings.Contains(svc.Type, "$this.") {
			svc.Type = strings.ReplaceAll(svc.Type, "$this.", this.path+".")
		}
		// Substitute in the constructor func (at the start of the path and
		// anywhere within generic type arguments). The method field takes no
		// $this: it addresses a service (@svc.Method), never a package.
		svc.Constructor.Func = substituteThisInFuncRef(svc.Constructor.Func, this.path)
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
			if converted.Kind == di.ArgGoRef || converted.Kind == di.ArgFieldAccessGo {
				if err := this.check(converted.Value, converted.SourceLoc, fmt.Sprintf("arg[%d]", i)); err != nil {
					return di.Service{}, err
				}
			}
			// Substitute $this in !go: argument values
			if this.path != "" && converted.Kind == di.ArgGoRef && strings.Contains(converted.Value, "$this.") {
				converted.Value = strings.Replace(converted.Value, "$this.", this.path+".", 1)
			}
			// Substitute $this in !field:!go: argument values
			if this.path != "" && converted.Kind == di.ArgFieldAccessGo && strings.Contains(converted.Value, "$this.") {
				converted.Value = strings.Replace(converted.Value, "$this.", this.path+".", 1)
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
