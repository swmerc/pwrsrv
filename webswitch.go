package main

import (
	"bytes"
	"crypto/tls"
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

// ProSwitchOutlet is a subset of what Digital Loggers returns
type ProSwitchOutlet struct {
	Name  string `json:"name"`
	State bool   `json:"physical_state"`
}

type ProSwitch struct {
	config ProSwitchConfig
}

func (p *ProSwitch) GetOutlets() ([]ProSwitchOutlet, error) {
	var outlets []ProSwitchOutlet

	if body, err := p.get("restapi/relay/outlets/"); err != nil {
		return nil, err
	} else if err := json.Unmarshal(body, &outlets); err != nil {
		return nil, err
	}

	return outlets, nil
}

func (p *ProSwitch) GetOutlet(index int) (ProSwitchOutlet, error) {
	var outlet ProSwitchOutlet

	if body, err := p.get("restapi/relay/outlets/" + fmt.Sprintf("%d/", index)); err != nil {
		return ProSwitchOutlet{}, err
	} else if err := json.Unmarshal(body, &outlet); err != nil {
		return ProSwitchOutlet{}, err
	}

	return outlet, nil
}

func (p *ProSwitch) SetOutlet(index int, on bool) error {
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

func (p *ProSwitch) get(path string) ([]byte, error) {
	return p.performOperation(http.MethodGet, path, []byte{})
}

func (p *ProSwitch) put(path string, value string) ([]byte, error) {
	return p.performOperation(http.MethodPut, path, []byte(value))
}

func (p *ProSwitch) performOperation(method string, path string, data []byte) ([]byte, error) {
	// Create a client with digest auth that optionally ignores cert errors if the config says so.
	//
	// NOTE: This allows man in the middle attacks, so don't use it unless your DLI LP9 starts rejecting
	//       perfectly fine certs and insists on using a self-signed one.
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: p.config.IgnoreCert},
	}

	client := &http.Client{
		Transport: &digest.Transport{
			Username:  p.config.User,
			Password:  p.config.Password,
			Transport: transport,
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
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	// Anything but success means the operation did not happen
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("%s %s: %s", method, url, response.Status)
	}

	return body, nil
}
