package yaml

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	yamllib "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/gendi-org/gendi/imprt"
)

// checkFields reports the first key of m, in source order, that no yaml tag
// of T names. The DTOs call it before they decode a mapping, because the
// goccy decoder ignores unknown keys, and its DisallowUnknownField option
// names a random key when a mapping has two or more of them.
func checkFields[T any](m *ast.MappingNode) error {
	typ := reflect.TypeFor[T]()
	known := make(map[string]bool, typ.NumField())
	for i := range typ.NumField() {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("yaml"), ",")
		if name != "" && name != "-" {
			known[name] = true
		}
	}
	for _, entry := range m.Values {
		name := entry.Key.GetToken().Value
		if !known[name] {
			return nodeErrorf(entry.Key, "unknown field %q", name)
		}
	}
	return nil
}

// checkStringKeys reports the first key of m that is not a YAML string. In a
// mapping where the user chooses the keys, the decoder converts a scalar key
// to a string in its own way: 0x10 becomes "16", null becomes "". The name in
// the config then differs from the text in the file, so the loader rejects
// such a key. what names the key in the error, for example "service ID".
func checkStringKeys(m *ast.MappingNode, what string) error {
	for _, entry := range m.Values {
		if _, ok := entry.Key.(*ast.StringNode); ok {
			continue
		}
		// Show the key as the file spells it: "~", not the "null" of String.
		text := entry.Key.GetToken().Value
		if tag, ok := entry.Key.(*ast.TagNode); ok && tag.Value != nil {
			text += " " + tag.Value.GetToken().Value
		}
		return nodeErrorf(entry.Key, "%s %s is not a string, quote it", what, text)
	}
	return nil
}

// ServiceDefaults holds default values for service configuration.
type ServiceDefaults struct {
	Shared        *bool `yaml:"shared"`
	Public        *bool `yaml:"public"`
	Autoconfigure *bool `yaml:"autoconfigure"`
}

func (d *ServiceDefaults) UnmarshalYAML(node ast.Node) error {
	m, ok := node.(*ast.MappingNode)
	if !ok {
		return nodeErrorf(node, "service defaults must be a mapping")
	}
	if err := checkFields[ServiceDefaults](m); err != nil {
		return err
	}
	type alias ServiceDefaults
	var decoded alias
	if err := yamllib.NodeToValue(node, &decoded); err != nil {
		return err
	}
	*d = ServiceDefaults(decoded)
	return nil
}

// RawConfig is the YAML-specific representation of a config file.
type RawConfig struct {
	Imports    []RawImport             `yaml:"imports"`
	Parameters map[string]RawParameter `yaml:"parameters"`
	Tags       map[string]RawTag       `yaml:"tags"`
	Services   map[string]*RawService  `yaml:"services"`

	// Defaults holds the reserved _default entry of services. Services does
	// not contain it.
	Defaults *ServiceDefaults `yaml:"-"`
}

// decodeDefaults decodes the reserved _default entry of the services mapping.
func (c *RawConfig) decodeDefaults(root *ast.MappingNode) (*ServiceDefaults, error) {
	for _, kv := range root.Values {
		services, ok := kv.Value.(*ast.MappingNode)
		if !ok || keyString(kv.Key) != "services" {
			continue
		}
		for _, sv := range services.Values {
			if keyString(sv.Key) != "_default" {
				continue
			}
			var defaults ServiceDefaults
			if err := defaults.UnmarshalYAML(sv.Value); err != nil {
				return nil, err
			}
			return &defaults, nil
		}
	}
	return nil, nil
}

func (c *RawConfig) checkKeys(root *ast.MappingNode) error {
	for _, kv := range root.Values {
		m, ok := kv.Value.(*ast.MappingNode)
		if !ok {
			continue
		}
		var what string
		switch keyString(kv.Key) {
		case "services":
			what = "service ID"
		case "parameters":
			what = "parameter name"
		case "tags":
			what = "tag name"
		default:
			continue
		}
		if err := checkStringKeys(m, what); err != nil {
			return err
		}
	}
	return nil
}

func (c *RawConfig) UnmarshalYAML(node ast.Node) error {
	mapping, isMapping := node.(*ast.MappingNode)
	var defaults *ServiceDefaults
	if isMapping {
		if err := checkFields[RawConfig](mapping); err != nil {
			return err
		}
		if err := c.checkKeys(mapping); err != nil {
			return err
		}
		// Decode _default before the services. The decoder also decodes
		// _default as a RawService, which accepts more fields, so the error
		// for such a field must come from ServiceDefaults first.
		var err error
		if defaults, err = c.decodeDefaults(mapping); err != nil {
			return err
		}
	}

	type alias RawConfig
	var decoded alias
	if err := yamllib.NodeToValue(node, &decoded); err != nil {
		return err
	}
	delete(decoded.Services, "_default")
	decoded.Defaults = defaults

	if isMapping {
		for _, kv := range mapping.Values {
			switch keyString(kv.Key) {
			case "parameters":
				// goccy does not call RawParameter.UnmarshalYAML for null
				// values; register the entry here so the parser can report
				// a located error for it.
				if pm, ok := kv.Value.(*ast.MappingNode); ok {
					for _, pv := range pm.Values {
						name := keyString(pv.Key)
						param := decoded.Parameters[name]
						param.Node = pv.Value
						decoded.Parameters[name] = param
					}
				}
			case "tags":
				if tm, ok := kv.Value.(*ast.MappingNode); ok {
					for _, tv := range tm.Values {
						name := keyString(tv.Key)
						if tag, ok := decoded.Tags[name]; ok {
							tag.Node = tv.Value
							decoded.Tags[name] = tag
						}
					}
				}
			case "imports":
				// goccy does not call RawImport.UnmarshalYAML for null
				// elements, leaving entries without a node.
				if seq, ok := untag(kv.Value).(*ast.SequenceNode); ok {
					for i, v := range seq.Values {
						if decoded.Imports[i].Node == nil {
							return nodeErrorf(v, "import must be a string or mapping")
						}
					}
				}
			case "services":
				// goccy does not call RawService.UnmarshalYAML for null
				// values, leaving nil entries in the map.
				if sm, ok := kv.Value.(*ast.MappingNode); ok {
					for _, sv := range sm.Values {
						if keyString(sv.Key) == "_default" {
							continue
						}
						if decoded.Services[keyString(sv.Key)] == nil {
							node := sv.Value
							if node == nil {
								node = sv.Key
							}
							return nodeErrorf(node, "service must be a mapping or alias")
						}
					}
				}
			}
		}
	}

	*c = RawConfig(decoded)
	return nil
}

// untag returns the node under the explicit tags of n, for example the
// sequence of "!!seq [a, b]". The decoder reads the tagged node as the node
// under its tag.
func untag(n ast.Node) ast.Node {
	for {
		tag, ok := n.(*ast.TagNode)
		if !ok {
			return n
		}
		n = tag.Value
	}
}

// keyString returns the value of a string key, and "" for any other key.
// checkFields and checkStringKeys reject the other keys before the callers
// read the decoded values.
func keyString(n ast.Node) string {
	if s, ok := n.(*ast.StringNode); ok {
		return s.Value
	}
	return ""
}

type RawImport struct {
	Path    string   `yaml:"path"`
	Exclude []string `yaml:"exclude"`

	// Node holds the import entry node for location tracking.
	Node ast.Node `yaml:"-"`
	// ExcludeNodes holds the node of each exclude pattern, in the order of
	// Exclude.
	ExcludeNodes []ast.Node `yaml:"-"`
}

// errorNode returns the node that an error of the import points at: the
// exclude pattern that the error names, or else the import entry.
func (i *RawImport) errorNode(err error) ast.Node {
	var exclusionErr *imprt.ExclusionError
	if errors.As(err, &exclusionErr) && exclusionErr.Index < len(i.ExcludeNodes) {
		return i.ExcludeNodes[exclusionErr.Index]
	}
	return i.Node
}

func (i *RawImport) UnmarshalYAML(node ast.Node) error {
	switch n := node.(type) {
	case *ast.StringNode:
		i.Path = n.Value
		i.Node = node
		return nil
	case *ast.LiteralNode:
		if n.Value != nil {
			i.Path = n.Value.Value
		}
		i.Node = node
		return nil
	case *ast.MappingNode:
		if err := checkFields[RawImport](n); err != nil {
			return err
		}
		type alias RawImport
		var decoded alias
		if err := yamllib.NodeToValue(node, &decoded); err != nil {
			return err
		}
		if decoded.Path == "" {
			return nodeErrorf(node, "import path is required")
		}
		*i = RawImport(decoded)
		for _, entry := range n.Values {
			switch keyString(entry.Key) {
			case "path":
				// Errors of the import name its path, so they point at its
				// value.
				i.Node = entry.Value
			case "exclude":
				if seq, ok := untag(entry.Value).(*ast.SequenceNode); ok {
					i.ExcludeNodes = seq.Values
				}
			}
		}
		return nil
	default:
		return nodeErrorf(node, "import must be a string or mapping")
	}
}

// RawParameter holds the raw scalar node of a parameter default value.
type RawParameter struct {
	Value ast.Node

	// Node holds the parameter value node for location tracking.
	Node ast.Node `yaml:"-"`
}

func (p *RawParameter) UnmarshalYAML(node ast.Node) error {
	p.Value = node
	p.Node = node
	return nil
}

type RawTag struct {
	ElementType   string `yaml:"element_type"`
	SortBy        string `yaml:"sort_by"`
	Public        bool   `yaml:"public"`
	Autoconfigure bool   `yaml:"autoconfigure"`

	// Node holds the tag mapping node for location tracking.
	Node ast.Node `yaml:"-"`
}

func (t *RawTag) UnmarshalYAML(node ast.Node) error {
	if m, ok := node.(*ast.MappingNode); ok {
		if err := checkFields[RawTag](m); err != nil {
			return err
		}
	}
	type alias RawTag
	var decoded alias
	if err := yamllib.NodeToValue(node, &decoded); err != nil {
		return err
	}
	*t = RawTag(decoded)
	return nil
}

type RawServiceTag struct {
	Name       string
	Attributes map[string]any

	// Node holds the tag mapping node for location tracking.
	Node ast.Node `yaml:"-"`
}

func (t *RawServiceTag) UnmarshalYAML(node ast.Node) error {
	t.Node = node
	t.Attributes = make(map[string]any)

	switch n := node.(type) {
	case *ast.StringNode:
		if n.Value == "" {
			return nodeErrorf(node, "tag name is required")
		}
		t.Name = n.Value
		return nil
	case *ast.LiteralNode:
		if n.Value == nil || n.Value.Value == "" {
			return nodeErrorf(node, "tag name is required")
		}
		t.Name = n.Value.Value
		return nil
	case *ast.MappingNode:
		if err := checkStringKeys(n, "tag attribute"); err != nil {
			return err
		}
		for _, kv := range n.Values {
			key := keyString(kv.Key)
			if key == "name" {
				if err := yamllib.NodeToValue(kv.Value, &t.Name); err != nil {
					return wrapNodeError(node, "failed to decode tag name", err)
				}
			} else {
				var value any
				if err := yamllib.NodeToValue(kv.Value, &value); err != nil {
					return wrapNodeError(kv.Value, fmt.Sprintf("failed to decode tag attribute %q", key), err)
				}
				t.Attributes[key] = value
			}
		}
		if t.Name == "" {
			return nodeErrorf(node, "tag name is required")
		}
		return nil
	default:
		return nodeErrorf(node, "tag must be a string or mapping")
	}
}

type RawService struct {
	Type               string          `yaml:"type"`
	Constructor        RawConstructor  `yaml:"constructor"`
	Shared             *bool           `yaml:"shared"`
	Public             *bool           `yaml:"public"`
	Autoconfigure      *bool           `yaml:"autoconfigure"`
	Decorates          string          `yaml:"decorates"`
	DecorationPriority int             `yaml:"decoration_priority"`
	Tags               []RawServiceTag `yaml:"tags"`
	Alias              string          `yaml:"alias"`

	// Node holds the service mapping node for location tracking.
	Node ast.Node `yaml:"-"`
}

func (s *RawService) UnmarshalYAML(node ast.Node) error {
	switch n := node.(type) {
	case *ast.StringNode:
		*s = RawService{Alias: n.Value, Node: node}
		return nil
	case *ast.LiteralNode:
		ref := ""
		if n.Value != nil {
			ref = n.Value.Value
		}
		*s = RawService{Alias: ref, Node: node}
		return nil
	case *ast.MappingNode:
		if err := checkFields[RawService](n); err != nil {
			return err
		}
		type alias RawService
		var decoded alias
		if err := yamllib.NodeToValue(node, &decoded); err != nil {
			return err
		}
		// goccy does not call RawServiceTag.UnmarshalYAML for null sequence
		// entries, so they bypass the name validation.
		for _, kv := range n.Values {
			if keyString(kv.Key) != "tags" {
				continue
			}
			if seq, ok := kv.Value.(*ast.SequenceNode); ok {
				for i, tagNode := range seq.Values {
					if i < len(decoded.Tags) && decoded.Tags[i].Name == "" {
						errNode := tagNode
						if errNode == nil {
							errNode = kv.Key
						}
						return nodeErrorf(errNode, "tag name is required")
					}
				}
			}
		}
		*s = RawService(decoded)
		s.Node = node
		return nil
	default:
		return nodeErrorf(node, "service must be a mapping or alias")
	}
}

type RawConstructor struct {
	Func   string        `yaml:"func"`
	Method string        `yaml:"method"`
	Args   []RawArgument `yaml:"args"`

	// Node points at func/method scalar for location tracking.
	Node ast.Node `yaml:"-"`
}

func (c *RawConstructor) UnmarshalYAML(node ast.Node) error {
	c.Node = node
	if m, ok := node.(*ast.MappingNode); ok {
		if err := checkFields[RawConstructor](m); err != nil {
			return err
		}
	}

	type raw struct {
		Func   string `yaml:"func"`
		Method string `yaml:"method"`
	}
	var decoded raw
	if err := yamllib.NodeToValue(node, &decoded); err != nil {
		return err
	}
	c.Func = decoded.Func
	c.Method = decoded.Method

	// Walk the AST manually to grab arg nodes. Cannot use
	// NodeToValue with `Args []ast.Node` because goccy v1.19.2 decodes
	// null sequence entries as typed-nil interfaces, losing the
	// *ast.NullNode information needed by convertLiteral.
	var argNodes []ast.Node
	if mapping, ok := node.(*ast.MappingNode); ok {
		for _, kv := range mapping.Values {
			switch keyString(kv.Key) {
			case "func", "method":
				c.Node = kv.Value
			case "args":
				if seq, ok := kv.Value.(*ast.SequenceNode); ok {
					argNodes = seq.Values
				}
			}
		}
	}

	if len(argNodes) == 0 {
		return nil
	}
	c.Args = make([]RawArgument, len(argNodes))
	for i := range argNodes {
		if err := c.Args[i].UnmarshalYAML(argNodes[i]); err != nil {
			return err
		}
	}
	return nil
}

type RawArgument struct {
	Value *string
	Node  ast.Node
}

func (a *RawArgument) UnmarshalYAML(node ast.Node) error {
	a.Node = node

	switch n := node.(type) {
	case *ast.StringNode:
		val := n.Value
		a.Value = &val
	case *ast.LiteralNode:
		if n.Value != nil {
			val := n.Value.Value
			a.Value = &val
		}
	}
	return nil
}
