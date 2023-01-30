package hassclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"nhooyr.io/websocket"
)

type HassClient struct {
	ctx  context.Context
	conn *websocket.Conn

	cmdId uint
}

func Connect(ctx context.Context, server, token string) (*HassClient, error) {
	c, _, err := websocket.Dial(ctx, server, nil)
	if err != nil {
		return nil, err
	}

	c.SetReadLimit(512000000)

	client := HassClient{
		ctx:   ctx,
		conn:  c,
		cmdId: 2,
	}

	err = client.authenticate(token)
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func (client *HassClient) GetStates() {
	cmdId := client.cmdId
	client.cmdId++

	msgData := make(map[string]interface{})
	msgData["id"] = cmdId
	msgData["type"] = "get_states"

	err := client.sendJson(&msgData)
	if err != nil {
		fmt.Println(err)
		return
	}

	msgData = make(map[string]interface{})
	err = client.readJson(&msgData)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(msgData)
}

func (client *HassClient) Close() {
	client.conn.Close(websocket.StatusNormalClosure, "")
}

func (client *HassClient) sendJson(data interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = client.conn.Write(client.ctx, websocket.MessageText, bytes)
	if err != nil {
		return err
	}

	return nil
}

func (client *HassClient) readJson(data interface{}) error {
	msgType, bytes, err := client.conn.Read(client.ctx)
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
	msgData := make(map[string]string)
	err := client.readJson(&msgData)
	if err != nil {
		return err
	}

	if msgData["type"] != "auth_required" {
		return errors.New("received unexpected message type: " + msgData["type"])
	}

	msgData = make(map[string]string)
	msgData["type"] = "auth"
	msgData["access_token"] = token
	err = client.sendJson(&msgData)
	if err != nil {
		return err
	}

	msgData = make(map[string]string)
	err = client.readJson(&msgData)
	if err != nil {
		return err
	}

	if msgData["type"] != "auth_ok" {
		if msgData["type"] == "auth_invalid" {
			return errors.New("invalid authentication token")
		}
		return errors.New("received unexpected message type: " + msgData["type"])
	}

	return nil
}
