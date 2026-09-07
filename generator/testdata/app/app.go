package app

import (
	"io"
	"time"
)

type A struct{}

type B struct {
	A *A
}

type C struct{}

func NewA() *A {
	return &A{}
}

func NewB(a *A) *B {
	return &B{A: a}
}

func NewC() *C {
	return &C{}
}

type Logger struct {
	Prefix string
}

func NewLogger(prefix string) *Logger {
	return &Logger{Prefix: prefix}
}

type Timer struct {
	Delay time.Duration
}

func NewTimer(delay time.Duration) *Timer {
	return &Timer{Delay: delay}
}

type Service interface {
	Value() string
}

type BaseService struct{}

func NewServiceBase() Service {
	return &BaseService{}
}

func NewServiceBaseConcrete() *BaseService {
	return &BaseService{}
}

func (s *BaseService) Value() string { return "base" }

type DecoratorA struct {
	inner Service
}

func NewServiceDecoratorA(inner Service) Service {
	return &DecoratorA{inner: inner}
}

func NewServiceDecoratorAConcrete(inner Service) *DecoratorA {
	return &DecoratorA{inner: inner}
}

func (d *DecoratorA) Value() string { return d.inner.Value() }

type DecoratorB struct {
	inner Service
}

func NewServiceDecoratorB(inner Service) Service {
	return &DecoratorB{inner: inner}
}

func (d *DecoratorB) Value() string { return d.inner.Value() }

type Consumer struct {
	Svc Service
}

func NewConsumer(svc Service) *Consumer {
	return &Consumer{Svc: svc}
}

type InterfaceConsumer struct {
	Items []interface{}
}

func NewInterfaceConsumer(items []interface{}) *InterfaceConsumer {
	return &InterfaceConsumer{Items: items}
}

// Handler types for testing variadic and spread
type Handler interface {
	Handle()
}

type HandlerA struct{}

func NewHandlerA() *HandlerA {
	return &HandlerA{}
}

func (h *HandlerA) Handle() {}

type HandlerB struct{}

func NewHandlerB() *HandlerB {
	return &HandlerB{}
}

func (h *HandlerB) Handle() {}

// GetAllHandlers returns a slice of handlers for testing spread
func GetAllHandlers(a, b *HandlerA) []Handler {
	return []Handler{a, b}
}

func GetAllHandlersWithError(a, b *HandlerA) ([]Handler, error) {
	return GetAllHandlers(a, b), nil
}

// Server with variadic handlers parameter
type Server struct {
	Handlers []Handler
}

func NewServer(handlers ...Handler) *Server {
	return &Server{Handlers: handlers}
}

// Router with a map-of-handlers parameter, for testing map arguments.
type Router struct {
	routes map[string]Handler
}

func NewRouter(routes map[string]Handler) *Router {
	return &Router{routes: routes}
}

func NewRouterWithError(routes map[string]Handler) (*Router, error) {
	return &Router{routes: routes}, nil
}

// Routes is an exported named map type, for testing that a map argument for
// a named parameter type still renders the named type by name.
type Routes map[string]Handler

func NewNamedRouter(routes Routes) *Router {
	return &Router{routes: routes}
}

// unexportedRoutes is an unexported named map type, for testing that a map
// argument for a parameter type inaccessible from the generated package
// falls back to rendering the underlying map type: the generated package
// cannot spell unexportedRoutes, but an unnamed map[string]Handler literal
// is still assignable to it.
type unexportedRoutes map[string]Handler

func NewUnexportedRouter(routes unexportedRoutes) *Router {
	return &Router{routes: routes}
}

type unexportedRouteName string

type unexportedRouteNames map[string]unexportedRouteName

func NewRouterWithUnexportedRouteNames(unexportedRouteNames) *Router {
	return &Router{}
}

// LabeledRouter takes two map arguments — one of services, one resolved from
// a parameter — for testing that an error source nested inside either map
// argument makes the whole build function fallible, even though the
// constructor itself never returns an error, and that unrelated service refs
// inside a map get a real error check instead of being discarded with `_ :=`.
type LabeledRouter struct {
	Routes map[string]Handler
	Labels map[string]string
}

func NewLabeledRouter(routes map[string]Handler, labels map[string]string) *LabeledRouter {
	return &LabeledRouter{Routes: routes, Labels: labels}
}

// Writer wraps an io.Writer for testing !go: references
type Writer struct {
	Out io.Writer
}

func NewWriter(out io.Writer) *Writer {
	return &Writer{Out: out}
}

// DefaultPrefix is a package-level variable for testing !go: with $this
var DefaultPrefix = "app"

// DatabaseConfig holds database configuration for testing !field: access
type DatabaseConfig struct {
	DSN string
}

// AppConfig holds application configuration for testing !field: access
type AppConfig struct {
	Host     string
	Port     int
	Database DatabaseConfig
}

func LoadConfig() *AppConfig {
	return &AppConfig{
		Host:     "localhost",
		Port:     8080,
		Database: DatabaseConfig{DSN: "postgres://localhost/db"},
	}
}

func LoadConfigWithError() (*AppConfig, error) {
	return LoadConfig(), nil
}

func NewServerWithAddr(host string, port int) *Server {
	return &Server{}
}

type Hub struct {
	items []interface{}
}

func NewHub(items []interface{}) (Hub, error) {
	return Hub{items: items}, nil
}

type Timed struct {
	ds []time.Duration
}

func NewTimed(ds ...time.Duration) *Timed {
	return &Timed{ds: ds}
}

func NewPrefixedServer(prefix string, handlers ...Handler) *Server {
	return &Server{Handlers: handlers}
}
