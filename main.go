package main

import (
	"fmt"
	"os"

	"github.com/biftin/hass-location-notifier/internal/config"
)

func main() {
	conf, err := config.LoadConfig("config.yaml")
	if err != nil {
		fmt.Println("Error loading config:", err)
		os.Exit(1)
	}

	fmt.Println(conf)
}
