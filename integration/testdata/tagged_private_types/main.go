package main

import "fmt"

func main() {
	container := NewContainer(nil)
	registry, err := container.GetRegistry()
	if err != nil {
		panic(err)
	}
	fmt.Print(registry.Describe())
}
