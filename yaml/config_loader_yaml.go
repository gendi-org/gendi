package yaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	yamllib "github.com/goccy/go-yaml"
	yamlparser "github.com/goccy/go-yaml/parser"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/imprt"
	"github.com/gendi-org/gendi/srcloc"
)

// ImportResolver resolves one import entry — path plus its exclusion masks —
// to addressed config candidates and their confinement boundaries.
type ImportResolver interface {
	// ResolveImport resolves an import path to config files, dropping the
	// files matched by the exclusion masks before the sandbox check.
	ResolveImport(baseDir, path string, excludes []string) ([]imprt.Candidate, error)
}

// ConfigLoaderYaml loads YAML configuration files with import resolution.
type ConfigLoaderYaml struct {
	resolver ImportResolver
	confiner *Confiner
	expander *AnchorExpander
	parser   *Parser
}

// NewConfigLoaderYaml creates a new YAML config loader with dependencies. A
// loader is not safe for concurrent use — the underlying resolver memoizes
// module lookups without locking; use one loader per goroutine.
func NewConfigLoaderYaml(resolver ImportResolver, parser *Parser) *ConfigLoaderYaml {
	return &ConfigLoaderYaml{
		resolver: resolver,
		confiner: NewConfiner(),
		expander: NewAnchorExpander(),
		parser:   parser,
	}
}

// loadState carries the per-Load bookkeeping shared across the recursion: the
// imports currently on the stack (for cycle detection, keyed by canonical real
// path) and the configs already loaded (so a diamond reaches each once, keyed
// by addressed path).
type loadState struct {
	inProgress map[string]bool
	cache      map[string]*di.Config
}

// Load loads a YAML config file with imports resolved. Every config, including
// the root, is confined immediately before it can be read.
func (l *ConfigLoaderYaml) Load(path, boundary string) (*di.Config, error) {
	state := &loadState{
		inProgress: map[string]bool{},
		cache:      map[string]*di.Config{},
	}
	return l.loadRecursive(imprt.Candidate{Path: path, Boundary: boundary}, nil, state)
}

// importError locates err at origin, the import entry that reached the
// config. The root config has no such entry, so its errors stay as they are.
func (l *ConfigLoaderYaml) importError(origin *srcloc.Location, err error) error {
	if origin == nil {
		return err
	}
	return srcloc.WrapError(origin, "import", err)
}

// loadRecursive loads the config of candidate. origin is the location of the
// import entry that reached candidate, and nil for the root config.
func (l *ConfigLoaderYaml) loadRecursive(candidate imprt.Candidate, origin *srcloc.Location, state *loadState) (*di.Config, error) {
	abs, id, err := l.confiner.Confine(candidate.Boundary, candidate.Path)
	if err != nil {
		return nil, l.importError(origin, err)
	}
	// A config already loaded through this same addressed spelling is reused:
	// its merged result is a pure function of that path — the path fixes $this
	// and the directory its own imports resolve against — so a diamond that
	// reaches it again need not re-read, re-parse and re-merge it. Keyed by the
	// addressed path, not the real path, so distinct symlink aliases still load
	// independently and keep their own $this context.
	if cfg, ok := state.cache[abs]; ok {
		return cfg, nil
	}
	// Cycle identity is canonical: an active import reached through another
	// spelling of the same real file is still a cycle. Confine returns id as
	// the symlink-resolved real path; loading and conversion stay on the
	// addressed path so every occurrence gets its own relative import and
	// $this context.
	if state.inProgress[id] {
		return nil, srcloc.Errorf(origin, "cyclic import detected at %s", abs)
	}
	state.inProgress[id] = true
	defer delete(state.inProgress, id)

	data, err := l.readFile(abs)
	if err != nil {
		return nil, l.importError(origin, err)
	}

	raw, err := l.parseRaw(data)
	if err != nil {
		return nil, l.toSrclocError(abs, err)
	}

	merged := di.NewConfig()

	baseDir := filepath.Dir(abs)
	for _, imp := range raw.Imports {
		loc := newLocation(abs, imp.Node)
		candidates, err := l.resolver.ResolveImport(baseDir, imp.Path, imp.Exclude)
		if err != nil {
			return nil, srcloc.WrapError(newLocation(abs, imp.errorNode(err)), fmt.Sprintf("resolve import %q", imp.Path), err)
		}

		for _, candidate := range candidates {
			child, err := l.loadRecursive(candidate, loc, state)
			if err != nil {
				return nil, err
			}
			merged = merged.MergeWith(child)
		}
	}

	cfg, err := l.parser.ConvertConfigWithDirAndFile(raw, baseDir, abs)
	if err != nil {
		return nil, srcloc.AddContext(err, "convert %s", abs)
	}

	result := merged.MergeWith(cfg)
	state.cache[abs] = result
	return result, nil
}

// parseRaw parses YAML data into raw config (with imports). It decodes the
// first document that has content, as yamllib.Unmarshal does.
func (l *ConfigLoaderYaml) parseRaw(data []byte) (*RawConfig, error) {
	file, err := yamlparser.ParseBytes(data, 0)
	if err != nil {
		return nil, err
	}
	var raw RawConfig
	for _, doc := range file.Docs {
		if doc.Body == nil {
			continue
		}
		body, err := l.expander.Expand(doc.Body)
		if err != nil {
			return nil, err
		}
		if err := yamllib.NodeToValue(body, &raw); err != nil {
			return nil, err
		}
		break
	}
	return &raw, nil
}

func (l *ConfigLoaderYaml) readFile(path string) ([]byte, error) {
	return l.osReadFile(path)
}

// osReadFile can be replaced for testing.
var defaultOsReadFile = os.ReadFile

func (l *ConfigLoaderYaml) osReadFile(path string) ([]byte, error) {
	return defaultOsReadFile(path)
}

// toSrclocError converts errors from parseRaw into *srcloc.Error, so
// the existing srcloc.Renderer can render snippet + caret.
//
// Three buckets:
//
//  1. *NodeError — our DTO-level validation. Pre-existing.
//  2. yaml.Error (interface) — goccy syntax errors and decode errors
//     (type mismatch, integer overflow, duplicate key, unknown field,
//     unexpected node). All implement GetToken / GetMessage.
//  3. anything else — fallback via srcloc.AddContext (does not
//     double-locate if the residual error is itself *srcloc.Error).
func (l *ConfigLoaderYaml) toSrclocError(file string, err error) error {
	var ne *NodeError
	if errors.As(err, &ne) {
		loc := newLocation(file, ne.Node)

		// ne.Err may itself wrap a goccy yaml.Error (e.g. NodeToValue
		// failed inside a custom UnmarshalYAML). srcloc.WrapError would
		// call ne.Err.Error(), which routes through goccy's own
		// formatter and breaks the single-style invariant. Normalize:
		// strip the wrapped yaml.Error down to its plain message.
		wrapped := ne.Err
		var inner yamllib.Error
		if errors.As(wrapped, &inner) {
			wrapped = errors.New(inner.GetMessage())
		}
		return srcloc.WrapError(loc, ne.Msg, wrapped)
	}

	var ye yamllib.Error
	if errors.As(err, &ye) {
		var loc *srcloc.Location
		if tok := ye.GetToken(); tok != nil && tok.Position != nil {
			loc = srcloc.NewLocation(file, tok.Position.Line, tok.Position.Column)
		}
		return srcloc.Errorf(loc, "%s", ye.GetMessage())
	}

	return srcloc.AddContext(err, "parse %s", file)
}
