package hassclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"
)

var InvalidResponseError = errors.New("invalid response")

type StateChange struct {
	EntityId string
	OldState string
	NewState string
}

type HassClient struct {
	conn *websocket.Conn

	cmdId uint

	eventCmdId           uint
	eventSubscribers     []eventSubscriber
	eventSubscribersLock sync.Mutex
    
	closed atomic.Bool

	errorCh chan error
}

func Connect(server, token string) (*HassClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, server, nil)
	if err != nil {
		return nil, err
	}

	c.SetReadLimit(512000000)

	client := HassClient{
		conn: c,

		cmdId: 2,

		errorCh: make(chan error),
	}

	err = client.authenticate(token)
	if err != nil {
		return nil, err
	}

	err = client.subscribeStateChanges()
	if err != nil {
		return nil, err
	}

	go client.handleIncoming()

	return &client, nil
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
				client.eventSubscribers = append(client.eventSubscribers[:i], client.eventSubscribers[i+1:]...)
				break
			}
		}

		client.eventSubscribersLock.Unlock()

		close(dataCh)
	}
}

func (client *HassClient) Error() <-chan error {
	return client.errorCh
}

func (client *HassClient) Close() {
    if !client.closed.CompareAndSwap(false, true) {
        return
    }

	client.eventSubscribersLock.Lock()
	eventSubscribers := client.eventSubscribers
	client.eventSubscribers = []eventSubscriber{}
	client.eventSubscribersLock.Unlock()

	for _, subscriber := range eventSubscribers {
		close(subscriber.dataCh)
	}

	close(client.errorCh)
	client.conn.Close(websocket.StatusNormalClosure, "")
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

		if returnData.Success == nil || *returnData.Success == false {
			return InvalidResponseError
		}
	}

	client.eventCmdId = eventCmdId

	return nil
}

func (client *HassClient) handleIncoming() {
	for {
		data, err := client.readData()
		if err != nil {
			client.writeError(fmt.Errorf("read websocket data: %s", err))
			client.Close()
			return
			// TODO: reconnect
		}

		var envelope envelope
		err = json.Unmarshal(data, &envelope)
		if err != nil {
			client.writeError(fmt.Errorf("parse json data: %s", err))
			continue
		}

		if envelope.ID == client.eventCmdId {
			if envelope.Type != "event" {
				client.writeError(fmt.Errorf("invalid message type '%s'", envelope.Type))
				continue
			}

			var eventMessage eventMessage
			err = json.Unmarshal(data, &eventMessage)
			if err != nil {
				client.writeError(fmt.Errorf("failed to read event message: %s", err))
				continue
			}

			client.eventSubscribersLock.Lock()
			for _, subscriber := range client.eventSubscribers {
				subscriber.dataCh <- StateChange{
					EntityId: eventMessage.Event.Data.EntityID,
					OldState: eventMessage.Event.Data.OldState.State,
					NewState: eventMessage.Event.Data.NewState.State}
			}
			client.eventSubscribersLock.Unlock()
		}
	}
}

func (client *HassClient) writeError(err error) {
	select {
	case client.errorCh <- err:
	default:
		log.Println(err)
	}
}
