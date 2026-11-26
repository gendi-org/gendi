package main

import (
	"fmt"
	"strings"
)

type Named interface {
	Name() string
}

type Users struct{}

func NewUsers() *Users { return &Users{} }

func (u *Users) Name() string { return "users" }

type Orders struct{}

func NewOrders() *Orders { return &Orders{} }

func (o *Orders) Name() string { return "orders" }

type Cache struct{ inner Named }

func NewCache(inner Named) *Cache { return &Cache{inner: inner} }

func (c *Cache) Name() string { return "cache(" + c.inner.Name() + ")" }

type Logging struct{ inner Named }

func NewLogging(inner Named) *Logging { return &Logging{inner: inner} }

func (l *Logging) Name() string { return "logging(" + l.inner.Name() + ")" }

type Tracing struct{ inner Named }

func NewTracing(inner Named) *Tracing { return &Tracing{inner: inner} }

func (t *Tracing) Name() string { return "tracing(" + t.inner.Name() + ")" }

type App struct {
	handlers  []Named
	auditable []Named
	clearable []Named
	users     Named
	ordersAPI Named
	orders    Named
}

func NewApp(handlers, auditable, clearable []Named, users, ordersAPI, orders Named) *App {
	return &App{
		handlers:  handlers,
		auditable: auditable,
		clearable: clearable,
		users:     users,
		ordersAPI: ordersAPI,
		orders:    orders,
	}
}

func (a *App) names(services []Named) string {
	names := make([]string, len(services))
	for i, s := range services {
		names[i] = s.Name()
	}
	return strings.Join(names, ", ")
}

func (a *App) Run() {
	fmt.Println("!tagged:handler   ", a.names(a.handlers))
	fmt.Println("!tagged:auditable ", a.names(a.auditable))
	fmt.Println("!tagged:clearable ", a.names(a.clearable))
	fmt.Println("@users            ", a.users.Name())
	fmt.Println("@orders.api       ", a.ordersAPI.Name())
	fmt.Println("@orders           ", a.orders.Name())
}
