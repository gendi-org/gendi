package ir

import (
	"fmt"
	"go/types"

	di "github.com/gendi-org/gendi"
	"github.com/gendi-org/gendi/xmaps"
)

type autoTagPhase struct{}

// Apply auto-tags services that match tag element types.
func (p *autoTagPhase) Apply(cfg *di.Config, container *Container) error {
	if len(cfg.Tags) == 0 || len(container.Services) == 0 {
		return nil
	}

	for _, tagName := range xmaps.OrderedKeys(cfg.Tags) {
		tagCfg := cfg.Tags[tagName]
		if !tagCfg.Autoconfigure {
			continue
		}

		tag := container.tags[tagName]
		if tag == nil {
			return fmt.Errorf("tag %q not found", tagName)
		}
		if tag.ElementType == nil {
			return fmt.Errorf("tag %q autoconfigure requires element_type", tagName)
		}

		existing := make(map[string]bool)
		for _, svc := range container.Services {
			for _, st := range svc.Tags {
				if st.Tag == tag {
					existing[svc.ID] = true
					break
				}
			}
		}

		for _, svcID := range xmaps.OrderedKeys(container.Services) {
			svc := container.Services[svcID]
			if svc == nil {
				continue
			}
			if svc.IsAlias() {
				continue
			}
			if !svc.Autoconfigure {
				continue
			}
			if existing[svc.ID] {
				continue
			}
			if svc.Type == nil || !types.AssignableTo(svc.Type, tag.ElementType) {
				continue
			}

			svc.Tags = append(svc.Tags, &ServiceTag{
				Tag:     tag,
				Service: svc,
				Key:     svc.TagKey,
			})
			existing[svc.ID] = true
		}
	}

	return nil
}
