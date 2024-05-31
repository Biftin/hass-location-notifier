package main

import (
	"fmt"
	"html/template"
	"log"
	"strings"

	"github.com/biftin/hass-location-notifier/internal/config"
	hassclient "github.com/biftin/hass-location-notifier/pkg/hass-client"
)

func main() {
	log.SetFlags(0)

	conf, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalln("Error: loading config:", err)
	}

	templates := parseTemplates(conf)

	client := hassclient.Connect(conf.Hass.Server, conf.Hass.Token)
	if err != nil {
		log.Fatalln("Error: connecting websocket:", err)
	}
	defer client.Close()

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
			log.Printf("Person ID \"%s\" is unknown.", personId)
			continue
		}

		var youMessage, otherMessage string
		if stateChange.NewState == "not_home" {
			location, ok := conf.FindLocation(stateChange.OldState)
			if !ok {
				log.Printf("Location ID \"%s\" is unknown.", stateChange.OldState)
				continue
			}

			templateData := templateData{
				Location: location.Texts.Leave,
				Person:   person.Name,
			}
			youMessage = renderTemplate(templates.Leave.You, &templateData)
			otherMessage = renderTemplate(templates.Leave.Other, &templateData)
		} else {
			location, ok := conf.FindLocation(stateChange.NewState)
			if !ok {
				log.Printf("Location ID \"%s\" is unknown.", stateChange.NewState)
				continue
			}

			templateData := templateData{
				Location: location.Texts.Arrive,
				Person:   person.Name,
			}
			youMessage = renderTemplate(templates.Arrive.You, &templateData)
			otherMessage = renderTemplate(templates.Arrive.Other, &templateData)
		}
		log.Print(otherMessage)

		notificationConfig := hassclient.NotificationConfig{
			Tag:     "family-location-" + personId,
			Group:   "family-locations",
			Channel: "FamilyLocations",
		}

		for _, receiver := range conf.People {
			if receiver.NotificationDevice != "" {
				if receiver.ID == personId {
					err = client.SendNotification(receiver.NotificationDevice, person.Name, youMessage, notificationConfig)
				} else {
					err = client.SendNotification(receiver.NotificationDevice, person.Name, otherMessage, notificationConfig)
				}

				if err != nil {
					log.Printf("Error: sending notification: %v", err)
				}
			}
		}
	}
}

type messageTemplates struct {
	Arrive struct {
		You   *template.Template
		Other *template.Template
	}
	Leave struct {
		You   *template.Template
		Other *template.Template
	}
}

func parseTemplates(config *config.Config) messageTemplates {
	var result messageTemplates

	result.Arrive.You = template.Must(template.New("arrive_you").Parse(config.Templates.Arrive.You))
	result.Arrive.Other = template.Must(template.New("arrive_other").Parse(config.Templates.Arrive.Other))

	result.Leave.You = template.Must(template.New("leave_you").Parse(config.Templates.Leave.You))
	result.Leave.Other = template.Must(template.New("leave_other").Parse(config.Templates.Leave.Other))

	return result
}

type templateData struct {
	Person   string
	Location string
}

func renderTemplate(t *template.Template, data *templateData) string {
	var sb strings.Builder

	err := t.Execute(&sb, data)
	if err != nil {
		fmt.Printf("Error: rendering message template: %v", err)
		return ""
	}

	return sb.String()
}
