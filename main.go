package main

import (
	"log"
	"strings"

	"github.com/biftin/hass-location-notifier/internal/config"
	"github.com/biftin/hass-location-notifier/pkg/hass-client"
)

func main() {
	log.SetFlags(0)

	conf, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalln("Error loading config:", err)
	}

	client, err := hassclient.Connect(conf.Hass.Server, conf.Hass.Token)
	if err != nil {
		log.Fatalln("Error connecting websocket:", err)
	}
	defer client.Close()
	log.Println("Connected to websocket API")

	stateChanges, _ := client.SubscribeStateChanges()
	for stateChange := range stateChanges {
		log.Printf("lel")
		personName, isPerson := strings.CutPrefix(stateChange.EntityId, "person.")
		if !isPerson {
			continue
		}

		person, ok := conf.People[personName]
		if !ok {
			continue
		}

		log.Printf("Person '%s' has moved from '%s' to '%s'", person.Name, stateChange.OldState, stateChange.NewState)
		// TODO: send notifications to devices
	}

	for err = range client.Error() {
		log.Println("Websocket error:", err)
	}
}
