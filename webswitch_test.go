package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestProSwitch(url string) *ProSwitch {
	return NewProSwitch(ProSwitchConfig{Url: url, User: "admin", Password: "secret"})
}

func TestProSwitchGetOutlets(t *testing.T) {
	_, srv := newFakeSwitch(t, defaultOutlets()...)
	p := newTestProSwitch(srv.URL)

	outlets, err := p.GetOutlets(context.Background())
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

func TestProSwitchSetOutletState(t *testing.T) {
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	p := newTestProSwitch(srv.URL)

	if err := p.SetOutletState(context.Background(), 0, true); err != nil {
		t.Fatalf("SetOutletState on: %v", err)
	}
	if fake.lastPutPath != "/restapi/relay/outlets/0/state/" {
		t.Errorf("PUT path = %q", fake.lastPutPath)
	}
	if fake.lastPutBody != "value=true" {
		t.Errorf("PUT body = %q", fake.lastPutBody)
	}
	if !fake.state(0) {
		t.Errorf("outlet 0 not on after SetOutletState(true)")
	}

	if err := p.SetOutletState(context.Background(), 0, false); err != nil {
		t.Fatalf("SetOutletState off: %v", err)
	}
	if fake.lastPutBody != "value=false" {
		t.Errorf("PUT body = %q", fake.lastPutBody)
	}
	if fake.state(0) {
		t.Errorf("outlet 0 still on after SetOutletState(false)")
	}
}

func TestProSwitchSetOutletName(t *testing.T) {
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	p := newTestProSwitch(srv.URL)

	// Characters that need form escaping, plus quotes that must not be added
	name := `lava "lamp" & co=1+2`
	if err := p.SetOutletName(context.Background(), 2, name); err != nil {
		t.Fatalf("SetOutletName: %v", err)
	}
	if fake.lastPutPath != "/restapi/relay/outlets/2/name/" {
		t.Errorf("PUT path = %q", fake.lastPutPath)
	}
	if want := "value=lava%20%22lamp%22%20%26%20co%3D1%2B2"; fake.lastPutBody != want {
		t.Errorf("PUT body = %q, want %q", fake.lastPutBody, want)
	}
	if got := fake.name(2); got != name {
		t.Errorf("name = %q, want %q", got, name)
	}
}

func TestProSwitchSetOutletNameRejected(t *testing.T) {
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	fake.failPutStatus = http.StatusForbidden
	p := newTestProSwitch(srv.URL)

	if err := p.SetOutletName(context.Background(), 0, "x"); err == nil {
		t.Errorf("SetOutletName: expected error")
	}
}

func TestProSwitchSendsRequiredHeaders(t *testing.T) {
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	p := newTestProSwitch(srv.URL)

	if _, err := p.GetOutlets(context.Background()); err != nil {
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

func TestProSwitchTrailingSlashInUrl(t *testing.T) {
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	p := newTestProSwitch(srv.URL + "/")

	if err := p.SetOutletState(context.Background(), 0, true); err != nil {
		t.Fatalf("SetOutletState: %v", err)
	}
	if fake.lastPutPath != "/restapi/relay/outlets/0/state/" {
		t.Errorf("PUT path = %q", fake.lastPutPath)
	}
}

func TestProSwitchReusesConnections(t *testing.T) {
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	p := newTestProSwitch(srv.URL)

	for i := 0; i < 5; i++ {
		if _, err := p.GetOutlets(context.Background()); err != nil {
			t.Fatalf("GetOutlets: %v", err)
		}
		if err := p.SetOutletState(context.Background(), 0, true); err != nil {
			t.Fatalf("SetOutletState: %v", err)
		}
	}
	if n := fake.conns(); n != 1 {
		t.Errorf("opened %d connections for sequential requests, want 1", n)
	}
}

func TestProSwitchErrorStatus(t *testing.T) {
	// Any non-2xx response from the switch should surface as an error
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	fake.failStatus = http.StatusInternalServerError
	p := newTestProSwitch(srv.URL)

	if _, err := p.GetOutlets(context.Background()); err == nil {
		t.Errorf("GetOutlets: expected error")
	}
	if err := p.SetOutletState(context.Background(), 0, true); err == nil {
		t.Errorf("SetOutletState: expected error")
	}
}

func TestProSwitchSetOutletStateRejected(t *testing.T) {
	// A rejected PUT has no body to fail parsing on, so this relies on the
	// status check
	fake, srv := newFakeSwitch(t, defaultOutlets()...)
	fake.failPutStatus = http.StatusForbidden
	p := newTestProSwitch(srv.URL)

	if err := p.SetOutletState(context.Background(), 0, true); err == nil {
		t.Errorf("SetOutletState: expected error")
	}
}

func TestProSwitchMissingOutlet(t *testing.T) {
	_, srv := newFakeSwitch(t, defaultOutlets()[:5]...)
	p := newTestProSwitch(srv.URL)

	if err := p.SetOutletState(context.Background(), 5, true); err == nil {
		t.Errorf("SetOutletState: expected error")
	}
}

func TestProSwitchUnreachable(t *testing.T) {
	_, srv := newFakeSwitch(t)
	url := srv.URL
	srv.Close()

	p := newTestProSwitch(url)
	if _, err := p.GetOutlets(context.Background()); err == nil {
		t.Errorf("GetOutlets: expected error")
	}
	if err := p.SetOutletState(context.Background(), 0, true); err == nil {
		t.Errorf("SetOutletState: expected error")
	}
}

func TestProSwitchHungSwitch(t *testing.T) {
	// A switch that accepts the request and never answers must not hang the
	// caller once its context is done
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	p := newTestProSwitch(srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := p.GetOutlets(ctx); err == nil {
		t.Errorf("GetOutlets: expected error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("GetOutlets took %v after its context expired", elapsed)
	}
}

func TestProSwitchHasTimeout(t *testing.T) {
	// Backstop for callers whose context never expires
	p := newTestProSwitch("http://127.0.0.1")
	if p.client.Timeout <= 0 || p.client.Timeout >= 15*time.Second {
		t.Errorf("client timeout = %v, want between 0 and the 15s server write timeout", p.client.Timeout)
	}
}
