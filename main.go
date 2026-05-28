package main

import (
	"log"
	"os"

	"wakirim/config"
	"wakirim/routes"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Fatal(err)
	}

	if err := config.InitDatabase(); err != nil {
		log.Fatal(err)
	}
	defer config.CloseDatabase()

	router := routes.SetupRouter()
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
