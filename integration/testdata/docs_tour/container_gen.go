package main

import "test/app"

type Container struct{}

func NewContainer(_ any) *Container {
	return &Container{}
}

func (c *Container) GetServer() (*app.Server, error) {
	panic("implement me")
}

func (c *Container) MustServer() *app.Server {
	panic("implement me")
}
