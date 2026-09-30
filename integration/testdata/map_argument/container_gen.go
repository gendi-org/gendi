package main

type Container struct{}

func NewContainer(_ any) *Container {
	return &Container{}
}

func (c *Container) MustRouter() *Router {
	panic("implement me")
}
