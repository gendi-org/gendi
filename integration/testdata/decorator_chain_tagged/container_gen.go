package main

type Container struct{}

func NewContainer(_ any) *Container {
	return &Container{}
}

func (c *Container) GetApp() (*App, error) {
	panic("implement me")
}

func (c *Container) MustApp() *App {
	panic("implement me")
}
