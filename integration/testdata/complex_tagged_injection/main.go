package main

import "fmt"

func (a *App) Run() {
	fmt.Printf("Middleware chain (%d):\n", len(a.middleware))
	for _, m := range a.middleware {
		fmt.Printf("- %s\n", m.Name())
	}
}

func main() {
	container := NewContainer(nil)
	app, err := container.GetApp()
	if err != nil {
		panic(err)
	}
	app.Run()

	// Test public tag getter
	middleware, err := container.GetTaggedWithMiddleware()
	if err != nil {
		panic(err)
	}
	fmt.Println("Public getter:")
	for key, m := range middleware {
		fmt.Printf("- %s: %s\n", key, m.Name())
	}
}
