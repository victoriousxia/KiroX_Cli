package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"reg_go/server"
)

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	os.MkdirAll(dataDir, 0755)

	// Load saved config from data/.env on startup
	loadSavedEnv(filepath.Join(dataDir, ".env"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "9527"
	}

	r := server.NewServer(dataDir)
	log.Printf("KiroX Web UI running on :%s", port)
	log.Fatal(r.Run(":" + port))
}

func loadSavedEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Only set if not already set by Docker environment
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}
