package main

import "fmt"

func main() {
	container := NewContainer(nil)
	router, err := container.GetRouter()
	if err != nil {
		panic(err)
	}

	fmt.Println("map:")
	for _, key := range router.Keys() {
		fmt.Printf("- %s: %s\n", key, router.routes[key].Name())
	}

	// Iterate two times: the sequence is reusable and keeps the sort_by order.
	for range 2 {
		fmt.Println("seq:")
		for key, h := range router.order {
			fmt.Printf("- %s: %s\n", key, h.Name())
		}
	}
}
