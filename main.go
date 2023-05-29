package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/biftin/hass-location-notifier/internal/config"
	hassclient "github.com/biftin/hass-location-notifier/pkg/hass-client"
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
		if stateChange.OldState == stateChange.NewState {
			continue
		}

		personId, isPerson := strings.CutPrefix(stateChange.EntityId, "person.")
		if !isPerson {
			continue
		}

		person, ok := conf.FindPerson(personId)
		if !ok {
			continue
		}

		var message, selfMessage string
		if stateChange.NewState == "not_home" {
			locationName := getLocationName(conf, personId, stateChange.OldState)
			message = fmt.Sprintf("%s left %s.", person.Name, locationName)
			selfMessage = fmt.Sprintf("You left %s.", locationName)
		} else {
			locationName := getLocationName(conf, personId, stateChange.NewState)
			message = fmt.Sprintf("%s arrived at %s.", person.Name, locationName)
			selfMessage = fmt.Sprintf("You arrived at %s.", locationName)
		}
		log.Print(message)

		notificationConfig := hassclient.NotificationConfig{
			Tag:     "",
			Group:   "family-locations",
			Channel: "FamilyLocations",
		}

		for _, receiver := range conf.People {
			if receiver.NotificationDevice != "" {
				notificationConfig.Tag = "family-location-" + personId

				if receiver.ID == personId {
					client.SendNotification(receiver.NotificationDevice, person.Name, selfMessage, notificationConfig)
				} else {
					client.SendNotification(receiver.NotificationDevice, person.Name, message, notificationConfig)
				}
			}
		}
	}

	for err = range client.Error() {
		log.Println("Websocket error:", err)
	}
}

func getLocationName(config *config.Config, personId, locationId string) string {
	location, ok := config.FindLocation(locationId)
	if !ok {
		return ""
	}

	if location.Owner == personId {
		return location.OwnerName
	}

	return location.Name
}
