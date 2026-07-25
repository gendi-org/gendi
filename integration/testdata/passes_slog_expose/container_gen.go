package main

type Container struct{}

func NewContainer(_ any) *Container {
	return &Container{}
}

func (c *Container) GetLogger() (*Logger, error) {
	panic("implement me")
}

func (c *Container) MustLogger() *Logger {
	panic("implement me")
}

func (c *Container) GetWorkerLogger() (*Logger, error) {
	panic("implement me")
}

func (c *Container) MustWorkerLogger() *Logger {
	panic("implement me")
}

func (c *Container) GetWorker() (*Worker, error) {
	panic("implement me")
}

func (c *Container) MustWorker() *Worker {
	panic("implement me")
}
