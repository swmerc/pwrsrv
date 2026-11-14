package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/icholy/digest"
)

//
// This deals with a subset of the API on Digital Loggers Pro Switch, which
// I intend to eventually expose on my LAN using something a bit more modern
// than digest auth at some point.
//

// Upper bound on a single request to the switch, digest round trips included.
// A local request can make two of these in a row, so keep the total under
// the local server's 15 second write timeout.
const proSwitchTimeout = 5 * time.Second

// ProSwitchOutlet is a subset of what Digital Loggers returns
type ProSwitchOutlet struct {
	Name  string `json:"name"`
	State bool   `json:"physical_state"`
}

type ProSwitch struct {
	config ProSwitchConfig
	client *http.Client
}

// NewProSwitch builds the one client used for every request, so connections
// to the switch get reused instead of piling up
func NewProSwitch(config ProSwitchConfig) *ProSwitch {
	config.Url = strings.TrimRight(config.Url, "/")

	// Optionally ignore cert errors if the config says so.
	//
	// NOTE: This allows man in the middle attacks, so don't use it unless your DLI LP9 starts rejecting
	//       perfectly fine certs and insists on using a self-signed one.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: config.IgnoreCert}

	client := &http.Client{
		Timeout: proSwitchTimeout,
		Transport: &digest.Transport{
			Username:  config.User,
			Password:  config.Password,
			Transport: transport,
		},
	}

	return &ProSwitch{config: config, client: client}
}

func (p *ProSwitch) GetOutlets(ctx context.Context) ([]ProSwitchOutlet, error) {
	var outlets []ProSwitchOutlet

	if body, err := p.get(ctx, "restapi/relay/outlets/"); err != nil {
		return nil, err
	} else if err := json.Unmarshal(body, &outlets); err != nil {
		return nil, err
	}

	return outlets, nil
}

func (p *ProSwitch) SetOutlet(ctx context.Context, index int, on bool) error {
	path := fmt.Sprintf("restapi/relay/outlets/%d/state/", index)
	var value string

	if on {
		value = "value=true"
	} else {
		value = "value=false"
	}

	_, err := p.put(ctx, path, value)
	return err
}

func (p *ProSwitch) get(ctx context.Context, path string) ([]byte, error) {
	return p.performOperation(ctx, http.MethodGet, path, []byte{})
}

func (p *ProSwitch) put(ctx context.Context, path string, value string) ([]byte, error) {
	return p.performOperation(ctx, http.MethodPut, path, []byte(value))
}

func (p *ProSwitch) performOperation(ctx context.Context, method string, path string, data []byte) ([]byte, error) {
	// Create the request, adding in the headers the device seems to want
	url := p.config.Url + "/" + path
	request, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("request: %s", err.Error())
	}
	request.Header.Add("X-CSRF", "x")
	request.Header.Add("Accept", "*/*")
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// Run it
	response, err := p.client.Do(request)
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
