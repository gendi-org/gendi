package main

type Handler interface{ Handle() }

type Service interface{ Name() string }

type Users struct{}

func NewUsers() *Users { return &Users{} }

func (u *Users) Handle()      {}
func (u *Users) Name() string { return "users" }

type Logging struct{ inner Service }

func NewLogging(inner Service) Service { return &Logging{inner: inner} }

func (l *Logging) Name() string { return "logging" }

type App struct{}

func NewApp(handlers []Handler) *App { return &App{} }
