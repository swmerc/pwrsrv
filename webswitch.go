package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/icholy/digest"
)

//
// This deals with a subset of the API on Digital Loggers Pro Switch, which
// I intend to eventually expose on my LAN using something a bit more modern
// than digest auth at some point.
//

// WebSwitchOutlet is a subset of what Digital Loggers returns
type WebSwitchOutlet struct {
	Name  string `json:"name"`
	State bool   `json:"physical_state"`
}

type DLIProSwitch struct {
	config DLIProSwitchConfig
}

func (p *DLIProSwitch) GetMaxIndex() int {
	return 7
}

func (p *DLIProSwitch) GetOutlets() ([]WebSwitchOutlet, error) {
	var outlets []WebSwitchOutlet

	if body, err := p.get("restapi/relay/outlets/"); err != nil {
		return nil, err
	} else if err := json.Unmarshal(body, &outlets); err != nil {
		return nil, err
	}

	return outlets, nil
}

func (p *DLIProSwitch) GetOutlet(index int) (WebSwitchOutlet, error) {
	var outlet WebSwitchOutlet

	if body, err := p.get("restapi/relay/outlets/" + fmt.Sprintf("%d/", index)); err != nil {
		return WebSwitchOutlet{}, err
	} else if err := json.Unmarshal(body, &outlet); err != nil {
		return WebSwitchOutlet{}, err
	}

	return outlet, nil
}

func (p *DLIProSwitch) SetOutlet(index int, on bool) error {
	path := fmt.Sprintf("restapi/relay/outlets/%d/state/", index)
	var value string

	if on {
		value = "value=true"
	} else {
		value = "value=false"
	}

	_, err := p.put(path, value)
	return err
}

func (p *DLIProSwitch) get(path string) ([]byte, error) {
	return p.performOperation(http.MethodGet, path, []byte{})
}

func (p *DLIProSwitch) put(path string, value string) ([]byte, error) {
	return p.performOperation(http.MethodPut, path, []byte(value))
}

func (p *DLIProSwitch) performOperation(method string, path string, data []byte) ([]byte, error) {
	// Create a client with digest auth
	client := &http.Client{
		Transport: &digest.Transport{
			Username: p.config.User,
			Password: p.config.Password,
		},
	}

	// Create the request, adding in the headers the device seems to want
	url := p.config.Url + "/" + path
	request, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("request: %s", err.Error())
	}
	request.Header.Add("X-CSRF", "x")
	request.Header.Add("Accept", "*/*")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// Run it
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// Process the body
	if body, err := io.ReadAll(response.Body); err != nil {
		return nil, err
	} else {
		return body, nil
	}
}
