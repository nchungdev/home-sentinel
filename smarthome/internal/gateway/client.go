package gateway

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"smarthome/internal/model"
)

var (
	ErrDeviceNotFound = errors.New("device not found")
	ErrCriticalDevice = errors.New("critical device cannot be turned off")
	ErrInvalidState   = errors.New("invalid state: must be on or off")
)

type homeAssistantStateItem struct {
	EntityID string `json:"entity_id"`
	State    string `json:"state"`
}

type homeAssistantServiceCall struct {
	EntityID string `json:"entity_id"`
}

type Client struct {
	gatewayBaseURL string
	bearerToken    string
	httpClient     *http.Client
	stateMutex     sync.RWMutex
	devicesByID    map[string]model.Device
	devicesByHAID  map[string]string
}

func NewClient(gatewayBaseURL string, bearerToken string) *Client {
	transportConfig := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     30 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transportConfig,
		Timeout:   4 * time.Second,
	}

	devicesList := model.Definitions()
	devicesMap := make(map[string]model.Device, len(devicesList))
	haMap := make(map[string]string, len(devicesList))

	for _, deviceItem := range devicesList {
		devicesMap[deviceItem.ID] = deviceItem
		haMap[deviceItem.HomeAssistantID] = deviceItem.ID
	}

	return &Client{
		gatewayBaseURL: strings.TrimRight(gatewayBaseURL, "/"),
		bearerToken:    bearerToken,
		httpClient:     httpClient,
		devicesByID:    devicesMap,
		devicesByHAID:  haMap,
	}
}

func (c *Client) Sync(parentContext context.Context) error {
	requestURL := fmt.Sprintf("%s/api/states", c.gatewayBaseURL)
	httpRequest, errorRequest := http.NewRequestWithContext(
		parentContext,
		http.MethodGet,
		requestURL,
		nil,
	)
	if errorRequest != nil {
		return errorRequest
	}

	httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken))

	httpResponse, errorExecute := c.httpClient.Do(httpRequest)
	if errorExecute != nil {
		return errorExecute
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway returned status %d", httpResponse.StatusCode)
	}

	var stateItems []homeAssistantStateItem
	if errorDecode := json.NewDecoder(httpResponse.Body).Decode(&stateItems); errorDecode != nil {
		return errorDecode
	}

	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()

	for _, item := range stateItems {
		internalID, found := c.devicesByHAID[item.EntityID]
		if !found {
			continue
		}

		currentDevice := c.devicesByID[internalID]
		currentDevice.State = item.State
		c.devicesByID[internalID] = currentDevice
	}

	return nil
}

func (c *Client) Devices() []model.Device {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()

	orderedList := model.Definitions()
	results := make([]model.Device, 0, len(orderedList))

	for _, item := range orderedList {
		if cachedDevice, exists := c.devicesByID[item.ID]; exists {
			results = append(results, cachedDevice)
		} else {
			results = append(results, item)
		}
	}

	return results
}

func (c *Client) Device(id string) (model.Device, bool) {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()

	foundDevice, exists := c.devicesByID[id]
	return foundDevice, exists
}

func (c *Client) SetState(
	parentContext context.Context,
	id string,
	targetState string,
) error {
	normalizedState := strings.ToLower(strings.TrimSpace(targetState))
	if normalizedState != "on" && normalizedState != "off" {
		return ErrInvalidState
	}

	c.stateMutex.RLock()
	targetDevice, exists := c.devicesByID[id]
	c.stateMutex.RUnlock()

	if !exists {
		return ErrDeviceNotFound
	}

	if targetDevice.IsCritical && normalizedState == "off" {
		return ErrCriticalDevice
	}

	actionService := "turn_on"
	if normalizedState == "off" {
		actionService = "turn_off"
	}

	requestURL := fmt.Sprintf("%s/api/services/light/%s", c.gatewayBaseURL, actionService)
	requestBody := homeAssistantServiceCall{EntityID: targetDevice.HomeAssistantID}

	serializedPayload, errorSerialize := json.Marshal(requestBody)
	if errorSerialize != nil {
		return errorSerialize
	}

	httpRequest, errorRequest := http.NewRequestWithContext(
		parentContext,
		http.MethodPost,
		requestURL,
		bytes.NewReader(serializedPayload),
	)
	if errorRequest != nil {
		return errorRequest
	}

	httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.bearerToken))
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorExecute := c.httpClient.Do(httpRequest)
	if errorExecute != nil {
		return errorExecute
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("service call returned status %d", httpResponse.StatusCode)
	}

	c.stateMutex.Lock()
	targetDevice.State = normalizedState
	c.devicesByID[id] = targetDevice
	c.stateMutex.Unlock()

	return nil
}

func (c *Client) Toggle(parentContext context.Context, id string) error {
	c.stateMutex.RLock()
	targetDevice, exists := c.devicesByID[id]
	c.stateMutex.RUnlock()

	if !exists {
		return ErrDeviceNotFound
	}

	nextState := "on"
	if targetDevice.State == "on" {
		nextState = "off"
	}

	return c.SetState(parentContext, id, nextState)
}

func (c *Client) StartBackgroundSync(
	serviceContext context.Context,
	syncInterval time.Duration,
) {
	tickerInstance := time.NewTicker(syncInterval)
	go func() {
		defer tickerInstance.Stop()
		for {
			select {
			case <-serviceContext.Done():
				return
			case <-tickerInstance.C:
				_ = c.Sync(serviceContext)
			}
		}
	}()
}
