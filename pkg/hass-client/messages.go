package hassclient

type envelope struct {
	ID      uint   `json:"id"`
	Type    string `json:"type"`
	Success *bool  `json:"success,omitempty"`
}

type authenticationRequest struct {
	Type        string `json:"type"`
	AccessToken string `json:"access_token"`
}

type subscribeEventsRequest struct {
	envelope
	EventType string `json:"event_type"`
}

type eventMessage struct {
	envelope
	Event eventBody `json:"event"`
}

type eventBody struct {
	Data      eventData `json:"data"`
	EventType string    `json:"event_type"`
}

type eventData struct {
	EntityID string     `json:"entity_id"`
	NewState eventState `json:"new_state"`
	OldState eventState `json:"old_state"`
}

type eventState struct {
	State string `json:"state"`
}
