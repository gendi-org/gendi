package main

type Handler interface{}

type Dummy struct{}

func NewDummy() *Dummy {
	return &Dummy{}
}
