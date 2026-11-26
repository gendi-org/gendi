package main

import "fmt"

type Handler interface {
	Name() string
}

type Users struct{}

func NewUsers() *Users { return &Users{} }

func (u *Users) Name() string { return "users" }

type Orders struct{}

func NewOrders() *Orders { return &Orders{} }

func (o *Orders) Name() string { return "orders" }

type Logging struct{ inner Handler }

func NewLogging(inner Handler) *Logging { return &Logging{inner: inner} }

func (l *Logging) Name() string { return "logging(" + l.inner.Name() + ")" }

type Metrics struct{ inner Handler }

func NewMetrics(inner Handler) *Metrics { return &Metrics{inner: inner} }

func (m *Metrics) Name() string { return "metrics(" + m.inner.Name() + ")" }

type App struct {
	handlers []Handler
}

func NewApp(handlers []Handler) *App {
	return &App{handlers: handlers}
}

func (a *App) Run() {
	for _, h := range a.handlers {
		fmt.Println(h.Name())
	}
}
