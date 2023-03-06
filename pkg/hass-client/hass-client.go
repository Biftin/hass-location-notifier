package hassclient

import (
	"context"
	"encoding/json"
	"errors"

	"nhooyr.io/websocket"
)

var InvalidResponseError = errors.New("invalid response")

type EntityState struct {
	id         string
	state      string
	attributes map[string]interface{}
}

func (entity *EntityState) ID() string {
    return entity.id
}

func (entity *EntityState) State() string {
    return entity.state
}

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

func (client *HassClient) GetStates() ([]EntityState, error) {
	cmdId := client.cmdId
	client.cmdId++

	msgData := map[string]interface{}{
		"id":   cmdId,
		"type": "get_states",
	}

	err := client.sendJson(&msgData)
	if err != nil {
		return nil, err
	}

	msgData = map[string]interface{}{}
	err = client.readJson(&msgData)
	if err != nil {
		return nil, err
	}

	success, ok := readBool(msgData, "success")
	if !ok {
		return nil, InvalidResponseError
	}

	if !success {
		return nil, readError(msgData)
	}

	result, ok := readSlice(msgData, "result")
	if !ok {
		return nil, InvalidResponseError
	}

	entities := []EntityState{}

	for _, resultItem := range result {
		if resultItem, ok := resultItem.(map[string]interface{}); ok {
			id, ok := readString(resultItem, "entity_id")
			if !ok {
				return nil, InvalidResponseError
			}

			state, ok := readString(resultItem, "state")
			if !ok {
				return nil, InvalidResponseError
			}

			attributes, ok := readMap(resultItem, "attributes")
			if !ok {
				return nil, InvalidResponseError
			}

			entities = append(entities, EntityState{id, state, attributes})
		}
	}

	return entities, nil
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

	msgData = map[string]string{
		"type":         "auth",
		"access_token": token,
	}

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

func readBool(data map[string]interface{}, key string) (bool, bool) {
	raw, ok := data[key]
	if !ok {
		return false, false
	}

	output, ok := raw.(bool)
	return output, ok
}

func readString(data map[string]interface{}, key string) (string, bool) {
	raw, ok := data[key]
	if !ok {
		return "", false
	}

	output, ok := raw.(string)
	return output, ok
}

func readSlice(data map[string]interface{}, key string) ([]interface{}, bool) {
	raw, ok := data[key]
	if !ok {
		return nil, false
	}

	output, ok := raw.([]interface{})
	return output, ok
}

func readMap(data map[string]interface{}, key string) (map[string]interface{}, bool) {
	raw, ok := data[key]
	if !ok {
		return nil, false
	}

	output, ok := raw.(map[string]interface{})
	return output, ok
}

func readErrorMessage(data map[string]interface{}) (string, bool) {
	errorMap, ok := readMap(data, "error")
	if !ok {
		return "", false
	}

	errorString, ok := readString(errorMap, "message")
	return errorString, ok
}

func readError(data map[string]interface{}) error {
	errorMessage, ok := readErrorMessage(data)
	if !ok {
		return InvalidResponseError
	}

	return errors.New(errorMessage)
}
