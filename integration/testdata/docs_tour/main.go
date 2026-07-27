package main

import (
	"fmt"

	"github.com/gendi-org/gendi/parameters"
)

func main() {
	fmt.Println(NewContainer(nil).MustServer().Handle("world"))

	custom := NewContainer(parameters.NewProviderMap(map[string]any{
		"greeting": "Hola",
	}))
	fmt.Println(custom.MustServer().Handle("world"))
}
