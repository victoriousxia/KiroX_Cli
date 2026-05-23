package main

import (
	"log"
	"os"

	"reg_go/server"
)

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	os.MkdirAll(dataDir, 0755)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := server.NewServer(dataDir)
	log.Printf("KiroX Web UI running on :%s", port)
	r.Run(":" + port)
}
