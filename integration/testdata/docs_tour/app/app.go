package app

import "test/greet"

type Server struct {
	greeter *greet.Greeter
}

func NewServer(g *greet.Greeter) *Server {
	return &Server{greeter: g}
}

func (s *Server) Handle(name string) string {
	return s.greeter.Greet(name)
}
