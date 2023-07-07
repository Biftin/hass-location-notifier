package hassclient

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

var ErrInvalidResponse = errors.New("invalid response")

type StateChange struct {
	EntityId string
	OldState string
	NewState string
}

type HassClient struct {
	state clientState

	url   string
	token string

	conn *websocket.Conn

	cmdId uint

	eventCmdId           uint
	eventSubscribers     []eventSubscriber
	eventSubscribersLock sync.Mutex

	readyCh  chan struct{}
	cmdIdCh  chan uint
	reconnCh chan struct{}
	closeCh  chan struct{}
}

func Connect(url, token string) *HassClient {
	client := HassClient{
		state: stateInitial,

		url:   url,
		token: token,

		conn: nil,

		cmdId: 2,

		eventCmdId:           0,
		eventSubscribers:     []eventSubscriber{},
		eventSubscribersLock: sync.Mutex{},

		readyCh:  make(chan struct{}),
		cmdIdCh:  make(chan uint),
		reconnCh: make(chan struct{}),
		closeCh:  make(chan struct{}),
	}

	go client.startWorker()

	return &client
}

func (client *HassClient) SubscribeStateChanges() (<-chan StateChange, func()) {
	dataCh := make(chan StateChange)

	subscription := eventSubscriber{dataCh}

	client.eventSubscribersLock.Lock()
	client.eventSubscribers = append(client.eventSubscribers, subscription)
	client.eventSubscribersLock.Unlock()

	return dataCh, func() {
		client.eventSubscribersLock.Lock()

		if len(client.eventSubscribers) == 0 {
			client.eventSubscribersLock.Unlock()
			return
		}

		for i, other := range client.eventSubscribers {
			if other == subscription {
				client.eventSubscribers = append(
					client.eventSubscribers[:i],
					client.eventSubscribers[i+1:]...,
				)
				break
			}
		}

		client.eventSubscribersLock.Unlock()

		close(dataCh)
	}
}

type NotificationConfig struct {
	Tag     string
	Group   string
	Channel string
}

func (client *HassClient) SendNotification(deviceId, title, message string, config NotificationConfig) error {
	eventCmdId, ok := <-client.cmdIdCh
	if !ok {
		return errors.New("client closed")
	}

	// Send subscription request
	err := client.sendJson(&callServiceNotifyRequest{
		callServiceRequest: callServiceRequest{
			envelope: envelope{
				ID:   eventCmdId,
				Type: "call_service",
			},
			Domain:  "notify",
			Service: "mobile_app_" + deviceId,
		},
		ServiceData: callServiceNotifyData{
			Title:   title,
			Message: message,
			Data: callServiceNotifyPlatformData{
				Tag:     config.Tag,
				Group:   config.Group,
				Channel: config.Channel,
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (client *HassClient) Close() {
	if !client.waitReady() {
		return
	}

	client.closeCh <- struct{}{}
}

type clientState int

const (
	stateInitial clientState = iota
	stateConnecting
	stateReady
	stateClosed
)

func (client *HassClient) startWorker() {
	for {
		if client.state == stateInitial || client.state == stateConnecting {
			log.Println("connecting to home assistant...")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			conn, _, err := websocket.Dial(ctx, client.url, nil)
			cancel()
			if err != nil {
				log.Println("error: connect:", err)
				time.Sleep(1 * time.Second)
				continue
			}

			conn.SetReadLimit(512000000)

			client.conn = conn

			err = client.authenticate(client.token)
			if err != nil {
				log.Println("error: authenticate:", err)
				conn.Close(websocket.StatusNormalClosure, "")
				time.Sleep(1 * time.Second)
				continue
			}

			err = client.subscribeStateChanges()
			if err != nil {
				log.Println("error: subscribe state changes:", err)
				conn.Close(websocket.StatusNormalClosure, "")
				time.Sleep(1 * time.Second)
				continue
			}

			log.Println("connection successful")

			go client.startReadWorker()
			client.state = stateReady
		}

		if client.state == stateReady {
			select {
			case client.readyCh <- struct{}{}:
			case client.cmdIdCh <- client.nextCmdId():
			case <-client.reconnCh:
				log.Println("connection lost, trying to reconnect...")
				client.state = stateConnecting
			case <-client.closeCh:
				log.Println("closing connection...")
				client.state = stateClosed
			}
		}

		if client.state == stateClosed {
			client.conn.Close(websocket.StatusNormalClosure, "")
			client.eventSubscribersLock.Lock()
			for _, eventSub := range client.eventSubscribers {
				close(eventSub.dataCh)
			}
			close(client.readyCh)
			close(client.cmdIdCh)
			client.eventSubscribersLock.Unlock()
			break
		}
	}

	log.Println("connection closed")
}

func (client *HassClient) waitReady() bool {
	_, ok := <-client.readyCh
	return ok
}

func (client *HassClient) startReadWorker() {
	for {
		if !client.waitReady() {
			return
		}

		data, err := client.readData()
		if err != nil {
			log.Println("error: read websocket data:", err)
			break
		}

		var envelope envelope
		err = json.Unmarshal(data, &envelope)
		if err != nil {
			log.Println("error: parse json data:", err)
			continue
		}

		if envelope.ID == client.eventCmdId {
			if envelope.Type != "event" {
				log.Println("error: invalid message type:", envelope.Type)
				continue
			}

			var eventMessage eventMessage
			err = json.Unmarshal(data, &eventMessage)
			if err != nil {
				log.Println("error: failed to read event message:", err)
				continue
			}

			client.eventSubscribersLock.Lock()
			for _, subscriber := range client.eventSubscribers {
				subscriber.dataCh <- StateChange{
					EntityId: eventMessage.Event.Data.EntityID,
					OldState: eventMessage.Event.Data.OldState.State,
					NewState: eventMessage.Event.Data.NewState.State,
				}
			}
			client.eventSubscribersLock.Unlock()
		}
	}

	client.reconnCh <- struct{}{}
}

type eventSubscriber struct {
	dataCh chan StateChange
}

func (client *HassClient) nextCmdId() uint {
	cmdId := client.cmdId
	client.cmdId++
	return cmdId
}

func (client *HassClient) sendJson(data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = client.conn.Write(ctx, websocket.MessageText, bytes)
	if err != nil {
		return err
	}

	return nil
}

func (client *HassClient) readData() ([]byte, error) {
	msgType, bytes, err := client.conn.Read(context.Background())
	if err != nil {
		return nil, err
	}
	if msgType != websocket.MessageText {
		return nil, errors.New("received unexpected non-text message")
	}

	return bytes, nil
}

func (client *HassClient) readJson(data interface{}) error {
	msgType, bytes, err := client.conn.Read(context.Background())
	if err != nil {
		return err
	}
	if msgType != websocket.MessageText {
		return errors.New("received unexpected non-text message")
	}

	err = json.Unmarshal(bytes, data)
	if err != nil {
		return err
	}

	return nil
}

func (client *HassClient) authenticate(token string) error {
	// Read initial message; should be "auth_required"
	{
		var msgData envelope
		err := client.readJson(&msgData)
		if err != nil {
			return err
		}

		if msgData.Type != "auth_required" {
			return errors.New("received unexpected message type: " + msgData.Type)
		}
	}

	// Send auth message
	{
		msgData := authenticationRequest{
			Type:        "auth",
			AccessToken: token,
		}

		err := client.sendJson(&msgData)
		if err != nil {
			return err
		}
	}

	// Read auth response
	{
		var msgData envelope
		err := client.readJson(&msgData)
		if err != nil {
			return err
		}

		if msgData.Type != "auth_ok" {
			if msgData.Type == "auth_invalid" {
				return errors.New("invalid authentication token")
			}
			return errors.New("received unexpected message type: " + msgData.Type)
		}
	}

	return nil
}

func (client *HassClient) subscribeStateChanges() error {
	eventCmdId := client.nextCmdId()

	// Send subscription request
	{
		err := client.sendJson(&subscribeEventsRequest{
			envelope:  envelope{ID: eventCmdId, Type: "subscribe_events"},
			EventType: "state_changed",
		})
		if err != nil {
			return err
		}
	}

	// Read success response
	{
		var returnData envelope
		err := client.readJson(&returnData)
		if err != nil {
			return err
		}

		if returnData.Success == nil || !*returnData.Success {
			return ErrInvalidResponse
		}
	}

	client.eventCmdId = eventCmdId

	return nil
}
