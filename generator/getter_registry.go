package generator

import (
	"fmt"
	"slices"
)

// GetterRegistry assigns getter names and validates all generated container identifiers.
// Note: After tag desugaring, tags become regular services carrying the
// ir.TagServicePrefix ("__tagged_with.") and are named by the same rules as
// any other service, which turns that prefix into a GetTaggedWith… getter.
type GetterRegistry struct {
	identGenerator *IdentGenerator

	publicService     map[string]string
	mustPublicService map[string]string
	privateService    map[string]string
}

// NewGetterRegistry creates a new getter registry.
func NewGetterRegistry(identGenerator *IdentGenerator) *GetterRegistry {
	return &GetterRegistry{
		identGenerator:    identGenerator,
		publicService:     make(map[string]string),
		mustPublicService: make(map[string]string),
		privateService:    make(map[string]string),
	}
}

func (gr *GetterRegistry) registerIdentifier(kind, name, serviceID string, used map[string]string) error {
	if previousID, ok := used[name]; ok {
		ids := []string{previousID, serviceID}
		slices.Sort(ids)
		return fmt.Errorf(
			"service identifiers %q and %q normalize to the same %s %q",
			ids[0], ids[1], kind, name,
		)
	}

	used[name] = serviceID

	return nil
}

// Assign assigns normalized getter names and rejects collisions between identifiers
// that will be emitted in the same Go namespace.
func (gr *GetterRegistry) Assign(orderedServiceIDs []string, services map[string]*serviceDef) error {
	gr.publicService = make(map[string]string)
	gr.mustPublicService = make(map[string]string)
	gr.privateService = make(map[string]string)

	serviceIDs := slices.Clone(orderedServiceIDs)
	slices.Sort(serviceIDs)

	methods := make(map[string]string)
	for _, id := range serviceIDs {
		svc := services[id]

		privateGetter := gr.identGenerator.Getter(id, false)
		if err := gr.registerIdentifier("container method", privateGetter, id, methods); err != nil {
			return err
		}
		gr.privateService[id] = privateGetter

		if !svc.IsAlias() {
			if err := gr.registerIdentifier("container method", gr.identGenerator.Build(id), id, methods); err != nil {
				return err
			}
		}

		if svc.public {
			publicGetter := gr.identGenerator.Getter(id, true)
			if err := gr.registerIdentifier("container method", publicGetter, id, methods); err != nil {
				return err
			}
			gr.publicService[id] = publicGetter

			mustGetter := gr.identGenerator.Must(id)
			if err := gr.registerIdentifier("container method", mustGetter, id, methods); err != nil {
				return err
			}
			gr.mustPublicService[id] = mustGetter
		}
	}

	fields := make(map[string]string)
	for _, id := range serviceIDs {
		svc := services[id]
		if svc.IsAlias() || !svc.shared {
			continue
		}

		field := gr.identGenerator.Field(id)
		if err := gr.registerIdentifier("container field", field, id, fields); err != nil {
			return err
		}
		if !isNilable(svc.GetterType()) {
			if err := gr.registerIdentifier("container field", field+"Init", id, fields); err != nil {
				return err
			}
		}
	}

	return nil
}

// PublicService returns the public getter name for a service.
func (gr *GetterRegistry) PublicService(id string) string {
	return gr.publicService[id]
}

// PrivateService returns the private getter name for a service.
func (gr *GetterRegistry) PrivateService(id string) string {
	return gr.privateService[id]
}

// MustService returns the Must* getter name for a public service.
// It transforms the public getter name (e.g., "GetService") to "MustService".
func (gr *GetterRegistry) MustService(id string) string {
	return gr.mustPublicService[id]
}
