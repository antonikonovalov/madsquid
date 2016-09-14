package main

import "server"

func main() {
	config := server.GetConfig()
	server := server.New(config)
	server.Run()
}
