package main

import "test/handlers"

type Container struct{}

func NewContainer(_ any) *Container {
	return &Container{}
}

func (c *Container) GetRegistry() (*handlers.Registry, error) {
	panic("implement me")
}
