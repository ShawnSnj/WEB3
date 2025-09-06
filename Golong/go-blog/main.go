package main

import (
	"go-blog/config"
	"go-blog/routes"
)

func main() {
	config.ConnectDB()
	r := routes.SetupRouter()
	r.Run(":8080")
}
