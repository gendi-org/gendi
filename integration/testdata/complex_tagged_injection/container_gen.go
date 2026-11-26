package main

import "iter"

type Container struct{}

func NewContainer(_ any) *Container {
	return &Container{}
}

func (c *Container) GetApp() (*App, error) {
	panic("implement me")
}

func (c *Container) GetTaggedWithMiddleware() (iter.Seq2[string, Middleware], error) {
	panic("implement me")
}
