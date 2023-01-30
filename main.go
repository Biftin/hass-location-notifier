package main

import (
	"context"
	"fmt"
	"os"

	"github.com/biftin/hass-location-notifier/internal/config"
	"github.com/biftin/hass-location-notifier/pkg/hass-client"
)

func main() {
	conf, err := config.LoadConfig("config.yaml")
	if err != nil {
		fmt.Println("Error loading config:", err)
		os.Exit(1)
	}

	fmt.Println(conf)

	client, err := hassclient.Connect(context.Background(), conf.Hass.Server, conf.Hass.Token)
	if err != nil {
		fmt.Println("Error connecting websocket:", err)
		os.Exit(1)
	}

	client.GetStates()

	defer client.Close()
}
