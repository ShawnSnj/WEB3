package main

import (
	"fmt"

	"github.com/bwmarrin/snowflake"
)

func main() {
	n, err := snowflake.NewNode(1)
	if err != nil {
		fmt.Println(err)
		return
	}

	for i := 0; i < 10; i++ {
		id := n.Generate()
		fmt.Println(id)
		fmt.Println(
			"Node:", id.Node(),
			"Step:", id.Step(),
			"Time:", id.Time(),
		)
	}

	// fmt.Println("Hello, World!")

	// myMap := map[string]int{
	// 	"apple":  5,
	// 	"banana": 3,
	// 	"cherry": 7,
	// }

	// for key, value := range myMap {
	// 	fmt.Printf("%s: %d\n", key, value)
	// }

	// mMap := make(map[string]int)
	// mMap["date"] = 4
	// mMap["elderberry"] = 2
	// mMap["fig"] = 6

	// for key, value := range mMap {
	// 	fmt.Printf("%s: %d\n", key, value)
	// }
}
