package main

import (
	"net/http"
	"testing"
)

func newTestProSwitch(url string) ProSwitch {
	return ProSwitch{config: ProSwitchConfig{Url: url, User: "admin", Password: "secret"}}
}

func TestProSwitchGetOutlets(t *testing.T) {
	_, srv := newFakeSwitch(t, defaultOutlets()...)
	p := newTestProSwitch(srv.URL)

	outlets, err := p.GetOutlets()
	if err != nil {
		t.Fatalf("GetOutlets: %v", err)
	}
	if len(outlets) != 8 {
		t.Fatalf("got %d outlets, want 8", len(outlets))
	}
	if outlets[1].Name != "disco ball" || !outlets[1].State {
		t.Errorf("outlet 1 = %+v, want disco ball/on", outlets[1])
	}
}

func TestProSwitchGetOutlet(t *testing.T) {
	_, srv := newFakeSwitch(t, defaultOutlets()...)
	p := newTestProSwitch(srv.URL)

	outlet, err := p.GetOutlet(0)
	if err != nil {
		t.Fatalf("GetOutlet: %v", err)
	}
	if outlet.Name != "sprinkler" || outlet.State {
		t.Errorf("outlet 0 = %+v, want sprinkler/off", outlet)
	}
}

func TestProSwitchSetOutlet(t *testing.T) {
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	p := newTestProSwitch(srv.URL)

	if err := p.SetOutlet(0, true); err != nil {
		t.Fatalf("SetOutlet on: %v", err)
	}
	if fake.lastPutPath != "/restapi/relay/outlets/0/state/" {
		t.Errorf("PUT path = %q", fake.lastPutPath)
	}
	if fake.lastPutBody != "value=true" {
		t.Errorf("PUT body = %q", fake.lastPutBody)
	}
	if !fake.state(0) {
		t.Errorf("outlet 0 not on after SetOutlet(true)")
	}

	if err := p.SetOutlet(0, false); err != nil {
		t.Fatalf("SetOutlet off: %v", err)
	}
	if fake.lastPutBody != "value=false" {
		t.Errorf("PUT body = %q", fake.lastPutBody)
	}
	if fake.state(0) {
		t.Errorf("outlet 0 still on after SetOutlet(false)")
	}
}

func TestProSwitchSendsRequiredHeaders(t *testing.T) {
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	p := newTestProSwitch(srv.URL)

	if _, err := p.GetOutlets(); err != nil {
		t.Fatalf("GetOutlets: %v", err)
	}

	want := map[string]string{
		"X-Csrf":       "x",
		"Accept":       "*/*",
		"Content-Type": "application/x-www-form-urlencoded",
	}
	for k, v := range want {
		if got := fake.lastHeaders.Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
}

func TestProSwitchErrorStatus(t *testing.T) {
	// Any non-2xx response from the switch should surface as an error
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	fake.failStatus = http.StatusInternalServerError
	p := newTestProSwitch(srv.URL)

	if _, err := p.GetOutlets(); err == nil {
		t.Errorf("GetOutlets: expected error")
	}
	if _, err := p.GetOutlet(0); err == nil {
		t.Errorf("GetOutlet: expected error")
	}
	if err := p.SetOutlet(0, true); err == nil {
		t.Errorf("SetOutlet: expected error")
	}
}

func TestProSwitchSetOutletRejected(t *testing.T) {
	// A rejected PUT has no body to fail parsing on, so this relies on the
	// status check
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	fake.failPutStatus = http.StatusForbidden
	p := newTestProSwitch(srv.URL)

	if err := p.SetOutlet(0, true); err == nil {
		t.Errorf("SetOutlet: expected error")
	}
}

func TestProSwitchMissingOutlet(t *testing.T) {
	_, srv := newFakeSwitch(t, defaultOutlets()[:5]...)
	p := newTestProSwitch(srv.URL)

	if _, err := p.GetOutlet(5); err == nil {
		t.Errorf("GetOutlet: expected error")
	}
	if err := p.SetOutlet(5, true); err == nil {
		t.Errorf("SetOutlet: expected error")
	}
}

func TestProSwitchUnreachable(t *testing.T) {
	_, srv := newFakeSwitch(t)
	url := srv.URL
	srv.Close()

	p := newTestProSwitch(url)
	if _, err := p.GetOutlets(); err == nil {
		t.Errorf("GetOutlets: expected error")
	}
	if err := p.SetOutlet(0, true); err == nil {
		t.Errorf("SetOutlet: expected error")
	}
}
