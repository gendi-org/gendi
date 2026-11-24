package yaml

import (
	"github.com/goccy/go-yaml/ast"
)

// AnchorExpander replaces aliases with the nodes of their anchors and
// replaces merge keys with the merged entries, before the DTOs decode the
// tree.
//
// The DTOs decode their own nodes with yamllib.NodeToValue. Each call starts
// a new decoder that knows only the anchors inside its own node, so an alias
// to an anchor in another part of the file does not resolve there. After the
// expansion, the tree has no aliases and no merge keys.
//
// The expander shares nodes and does not copy them. Each anchor expands one
// time, so a chain of aliases does not grow the tree exponentially. An error
// in a node that came from an anchor points to the anchor definition.
type AnchorExpander struct{}

func NewAnchorExpander() *AnchorExpander {
	return &AnchorExpander{}
}

// expand returns the node that replaces n. It visits the nodes in source
// order. An alias thus refers to the last anchor with that name before it,
// as YAML specifies, also when the file defines the name again later.
func (e *AnchorExpander) expand(anchors map[string]ast.Node, n ast.Node) (ast.Node, error) {
	switch n := n.(type) {
	case *ast.AnchorNode:
		value, err := e.expand(anchors, n.Value)
		if err != nil {
			return nil, err
		}
		// Register the anchor after its value. An alias inside the value
		// thus cannot refer to the anchor itself.
		anchors[n.Name.GetToken().Value] = value
		return value, nil
	case *ast.AliasNode:
		name := n.Value.GetToken().Value
		value, ok := anchors[name]
		if !ok {
			return nil, nodeErrorf(n, "unknown anchor %q", name)
		}
		return value, nil
	case *ast.TagNode:
		value, err := e.expand(anchors, n.Value)
		if err != nil {
			return nil, err
		}
		n.Value = value
		return n, nil
	case *ast.SequenceNode:
		for i, item := range n.Values {
			value, err := e.expand(anchors, item)
			if err != nil {
				return nil, err
			}
			n.Values[i] = value
		}
		return n, nil
	case *ast.MappingValueNode:
		// A block mapping with one entry can come as a bare entry.
		return e.expandMapping(anchors, ast.Mapping(n.GetToken(), false, n))
	case *ast.MappingNode:
		return e.expandMapping(anchors, n)
	default:
		return n, nil
	}
}

// expandMapping expands the keys and values of m in place and replaces each
// merge key with the merged entries. An explicit key of m overrides a merged
// key. When two merged mappings have the same key, the first one wins.
func (e *AnchorExpander) expandMapping(anchors map[string]ast.Node, m *ast.MappingNode) (*ast.MappingNode, error) {
	explicit := make(map[string]bool)
	hasMerge := false
	for _, entry := range m.Values {
		key, err := e.expand(anchors, entry.Key)
		if err != nil {
			return nil, err
		}
		mapKey, ok := key.(ast.MapKeyNode)
		if !ok {
			return nil, nodeErrorf(entry.Key, "mapping key must be a scalar")
		}
		entry.Key = mapKey

		value, err := e.expand(anchors, entry.Value)
		if err != nil {
			return nil, err
		}
		entry.Value = value

		if mapKey.IsMergeKey() {
			hasMerge = true
		} else {
			explicit[mapKey.GetToken().Value] = true
		}
	}
	if !hasMerge {
		return m, nil
	}

	seen := explicit
	values := make([]*ast.MappingValueNode, 0, len(m.Values))
	for _, entry := range m.Values {
		if !entry.Key.IsMergeKey() {
			values = append(values, entry)
			continue
		}
		sources := []ast.Node{entry.Value}
		if seq, ok := entry.Value.(*ast.SequenceNode); ok {
			sources = seq.Values
		}
		for _, source := range sources {
			merged, ok := source.(*ast.MappingNode)
			if !ok {
				return nil, nodeErrorf(source, "merge value must be a mapping or a sequence of mappings")
			}
			for _, mergedEntry := range merged.Values {
				name := mergedEntry.Key.GetToken().Value
				if seen[name] {
					continue
				}
				seen[name] = true
				values = append(values, mergedEntry)
			}
		}
	}
	m.Values = values
	return m, nil
}

// Expand expands one document body in place and returns its new root. The
// anchors of a document are not visible in the next document.
func (e *AnchorExpander) Expand(body ast.Node) (ast.Node, error) {
	return e.expand(make(map[string]ast.Node), body)
}
