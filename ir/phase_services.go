package ir

import (
	"cmp"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/srcloc"
	"github.com/gendi-org/gendi/xmaps"
)

// servicePhase initializes services from config
type servicePhase struct{}

// sortKey converts the value of a sort_by attribute to an integer. A value
// that is not an integer is an error: it would sort as 0 without a warning.
func (p *servicePhase) sortKey(v any) (int, error) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(rv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if rv.Uint() > math.MaxInt {
			return 0, fmt.Errorf("%d overflows int", rv.Uint())
		}
		return int(rv.Uint()), nil
	case reflect.Float32, reflect.Float64:
		f := rv.Float()
		if f != math.Trunc(f) || f < math.MinInt || f >= math.MaxInt {
			return 0, fmt.Errorf("%v is not an integer", v)
		}
		return int(f), nil
	case reflect.String:
		n, err := strconv.Atoi(rv.String())
		if err != nil {
			return 0, fmt.Errorf("%q is not an integer", rv.String())
		}
		return n, nil
	default:
		return 0, fmt.Errorf("%v is not an integer", v)
	}
}

// tagKey returns the key of a service in a keyed collection of the tag: the
// reserved attribute, or the default key of the service without it. The
// attribute must be a non-empty string also when no consumer reads the key, so
// that an error does not appear only after a keyed consumer is added.
func (p *servicePhase) tagKey(svc *Service, st di.ServiceTag) (string, error) {
	key, ok, err := st.IDAttribute(svc.ID)
	if err != nil || ok {
		return key, err
	}
	return svc.TagKey, nil
}

// Apply initializes IR services and determines service order
func (p *servicePhase) Apply(cfg *di.Config, container *Container) error {
	for _, id := range xmaps.OrderedKeys(cfg.Services) {
		svc := cfg.Services[id]
		// Validate service ID is not empty or whitespace-only
		if id == "" {
			return srcloc.Errorf(svc.SourceLoc, "service ID cannot be empty")
		}
		if strings.TrimSpace(id) == "" {
			return srcloc.Errorf(svc.SourceLoc, "service ID %q cannot be whitespace-only", id)
		}
		if svc.Alias != "" && svc.Shared {
			return srcloc.Errorf(
				svc.SourceLoc,
				"service %q: alias cannot define shared; lifecycle is inherited from target %q",
				id,
				svc.Alias,
			)
		}
		if svc.Alias != "" && svc.Autoconfigure {
			return srcloc.Errorf(
				svc.SourceLoc,
				"service %q: alias cannot define autoconfigure; autoconfigure applies to target %q",
				id,
				svc.Alias,
			)
		}
		if svc.Alias != "" && len(svc.Tags) > 0 {
			// A decorated alias points to its decorator, not to the target
			// that the user declared, so the message does not name a target.
			loc := svc.Tags[0].SourceLoc
			if loc == nil {
				loc = svc.SourceLoc
			}
			return srcloc.Errorf(loc, "service %q: alias cannot define tags; tag the target service instead", id)
		}

		irSvc := &Service{
			ID:            id,
			Shared:        svc.Shared,
			Public:        svc.Public,
			Autoconfigure: svc.Autoconfigure,
			Tags:          []*ServiceTag{},
			TagKey:        cmp.Or(svc.DecoratedID, id),
		}

		// Build service tags (create tags on-demand if not declared)
		for _, st := range svc.Tags {
			tag, ok := container.tags[st.Name]
			if !ok {
				// Create tag on-demand - ElementType will be inferred later
				tag = &Tag{
					Name: st.Name,
				}
				container.tags[st.Name] = tag
			}
			key, err := p.tagKey(irSvc, st)
			if err != nil {
				return err
			}
			irTag := &ServiceTag{
				Tag:        tag,
				Service:    irSvc,
				Attributes: st.Attributes,
				Key:        key,
				SourceLoc:  st.SourceLoc,
			}
			if v, ok := st.Attributes[tag.SortBy]; ok && tag.SortBy != "" {
				key, err := p.sortKey(v)
				if err != nil {
					return srcloc.WrapError(
						st.SourceLoc,
						fmt.Sprintf("service %q: tag %q: sort_by attribute %q", id, st.Name, tag.SortBy),
						err,
					)
				}
				irTag.SortKey = key
			}
			irSvc.Tags = append(irSvc.Tags, irTag)
		}

		container.Services[id] = irSvc
	}

	return nil
}
