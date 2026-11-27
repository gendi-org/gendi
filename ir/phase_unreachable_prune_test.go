package ir

import "testing"

func TestPruneUnreachableRemovesServices(t *testing.T) {
	publicSvc := &Service{ID: "public", Public: true}
	privateSvc := &Service{ID: "private"}
	sharedDep := &Service{ID: "dep"}

	publicSvc.Dependencies = []*Service{sharedDep}

	container := &Container{
		Services: map[string]*Service{
			publicSvc.ID:  publicSvc,
			privateSvc.ID: privateSvc,
			sharedDep.ID:  sharedDep,
		},
		tags: map[string]*Tag{},
	}

	_ = (&unreachablePrunePhase{}).Apply(nil, container)

	if _, ok := container.Services[publicSvc.ID]; !ok {
		t.Fatalf("expected public service to remain")
	}
	if _, ok := container.Services[sharedDep.ID]; !ok {
		t.Fatalf("expected dependency to remain")
	}
	if _, ok := container.Services[privateSvc.ID]; ok {
		t.Fatalf("expected private service to be removed")
	}
	if len(container.Services) != 2 {
		t.Fatalf("expected 2 services after prune, got %d", len(container.Services))
	}
}

func TestPruneUnreachableFiltersTagServices(t *testing.T) {
	publicSvc := &Service{ID: "public", Public: true}
	privateSvc := &Service{ID: "private"}
	tag := &Tag{
		Name:   "t",
		Public: true,
	}
	tag.Entries = []*ServiceTag{{Tag: tag, Service: privateSvc}}

	container := &Container{
		Services: map[string]*Service{
			publicSvc.ID:  publicSvc,
			privateSvc.ID: privateSvc,
		},
		tags: map[string]*Tag{
			tag.Name: tag,
		},
	}

	_ = (&unreachablePrunePhase{}).Apply(nil, container)

	if len(tag.Entries) != 1 || tag.Entries[0].Service.ID != privateSvc.ID {
		t.Fatalf("expected tag services to be filtered to reachable ones")
	}
}
