package main

type Greeter struct{ prefix string }

func NewGreeter(prefix string) *Greeter { return &Greeter{prefix: prefix} }
