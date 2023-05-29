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

type callServiceRequest struct {
	envelope
	Domain  string `json:"domain"`
	Service string `json:"service"`
}

type callServiceNotifyRequest struct {
	callServiceRequest
	ServiceData callServiceNotifyData `json:"service_data"`
}

type callServiceNotifyData struct {
	Title   string                        `json:"title"`
	Message string                        `json:"message"`
	Data    callServiceNotifyPlatformData `json:"data"`
}

type callServiceNotifyPlatformData struct {
	Group   string `json:"group,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Channel string `json:"channel,omitempty"`
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
