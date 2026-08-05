package main

func main() {
	c := NewContainer(nil)
	c.MustNamedRouter().Run()
	c.MustAliasRouter().Run()
}
