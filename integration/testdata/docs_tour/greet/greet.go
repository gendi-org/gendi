// greet/greet.go
package greet

type Greeter struct {
	prefix string
}

func New(prefix string) *Greeter {
	return &Greeter{prefix: prefix}
}

func (g *Greeter) Greet(name string) string {
	return g.prefix + ", " + name + "!"
}
