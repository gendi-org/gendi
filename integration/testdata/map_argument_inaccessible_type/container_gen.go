package main

import "test/routes"

type Container struct{}

func NewContainer(_ any) *Container {
	return &Container{}
}

func (c *Container) MustNamedRouter() *routes.NamedRouter {
	panic("implement me")
}

func (c *Container) MustAliasRouter() *routes.AliasRouter {
	panic("implement me")
}
