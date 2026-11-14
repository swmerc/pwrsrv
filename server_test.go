package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// newTestServer wires a server up to a fake 8 outlet switch and returns both
func newTestServer(t *testing.T) (*fakeSwitch, http.Handler) {
	t.Helper()
	return newTestServerWith(t, defaultOutlets()...)
}

func newTestServerWith(t *testing.T, outlets ...ProSwitchOutlet) (*fakeSwitch, http.Handler) {
	t.Helper()
	fake, srv := newFakeSwitch(t, outlets...)
	s := &server{webSwitchConfig: newTestProSwitch(srv.URL)}
	return fake, s.routes()
}

func doRequest(t *testing.T, h http.Handler, method, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	respBody, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, string(respBody)
}

func TestContentTypes(t *testing.T) {
	_, h := newTestServer(t)

	tests := map[string]string{
		"/api/outlets":         "application/json",
		"/api/outlets/1":       "application/json",
		"/api/outlets/1/state": "text/plain",
		"/api/outlets/1/name":  "text/plain",
	}
	for path, want := range tests {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get("Content-Type"); got != want {
			t.Errorf("GET %s: Content-Type = %q, want %q", path, got, want)
		}
	}
}

func TestGetAllOutlets(t *testing.T) {
	_, h := newTestServer(t)

	code, body := doRequest(t, h, http.MethodGet, "/api/outlets", "")
	if code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", code, body)
	}

	var outlets []Outlet
	if err := json.Unmarshal([]byte(body), &outlets); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(outlets) != 8 {
		t.Fatalf("got %d outlets, want 8", len(outlets))
	}
	// Numbering matches the hardware labels, starting at 1
	for i, outlet := range outlets {
		if outlet.Index != i+1 {
			t.Errorf("outlets[%d].Index = %d, want %d", i, outlet.Index, i+1)
		}
	}
	want := Outlet{Index: 2, Name: "disco ball", On: true}
	if outlets[1] != want {
		t.Errorf("outlets[1] = %+v, want %+v", outlets[1], want)
	}
}

func TestGetOutlet(t *testing.T) {
	_, h := newTestServer(t)

	for _, id := range []string{"2", "disco%20ball"} {
		t.Run(id, func(t *testing.T) {
			code, body := doRequest(t, h, http.MethodGet, "/api/outlets/"+id, "")
			if code != http.StatusOK {
				t.Fatalf("status = %d, body = %q", code, body)
			}

			var outlet Outlet
			if err := json.Unmarshal([]byte(body), &outlet); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			want := Outlet{Index: 2, Name: "disco ball", On: true}
			if outlet != want {
				t.Errorf("got %+v, want %+v", outlet, want)
			}
		})
	}
}

func TestUnknownOutlet(t *testing.T) {
	_, h := newTestServer(t)

	requests := []struct{ method, suffix, body string }{
		{http.MethodGet, "", ""},
		{http.MethodGet, "/state", ""},
		{http.MethodGet, "/name", ""},
		{http.MethodPut, "/state", "on"},
		{http.MethodPut, "/name", "lamp"},
	}
	for _, id := range []string{"nope", "0", "9", "-1", "02", "+2"} {
		for _, req := range requests {
			path := "/api/outlets/" + id + req.suffix
			code, _ := doRequest(t, h, req.method, path, req.body)
			if code != http.StatusBadRequest {
				t.Errorf("%s %s: status = %d, want %d", req.method, path, code, http.StatusBadRequest)
			}
		}
	}
}

func TestOutletNumberBeyondSwitch(t *testing.T) {
	// Only 5 outlets, so 6 is out of range even though a full switch has 8
	fake, h := newTestServerWith(t, defaultOutlets()[:5]...)

	if code, body := doRequest(t, h, http.MethodGet, "/api/outlets/5", ""); code != http.StatusOK {
		t.Errorf("GET outlet 5: status = %d, body = %q", code, body)
	}
	if code, _ := doRequest(t, h, http.MethodGet, "/api/outlets/6", ""); code != http.StatusBadRequest {
		t.Errorf("GET outlet 6: status = %d, want %d", code, http.StatusBadRequest)
	}
	if code, _ := doRequest(t, h, http.MethodPut, "/api/outlets/6/state", "on"); code != http.StatusBadRequest {
		t.Errorf("PUT outlet 6: status = %d, want %d", code, http.StatusBadRequest)
	}
	if n := fake.puts(); n != 0 {
		t.Errorf("switch got %d PUTs, want 0", n)
	}
}

func TestNumericNameIsFoundWhenOutOfRange(t *testing.T) {
	outlets := defaultOutlets()
	outlets[3].Name = "42"
	_, h := newTestServerWith(t, outlets...)

	code, body := doRequest(t, h, http.MethodGet, "/api/outlets/42", "")
	if code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", code, body)
	}
	if !strings.Contains(body, `"index":4`) {
		t.Errorf("body = %q, want outlet 4", body)
	}
}

func TestGetOutletState(t *testing.T) {
	_, h := newTestServer(t)

	tests := map[string]string{
		"1":            "off",
		"sprinkler":    "off",
		"2":            "on",
		"disco%20ball": "on",
	}
	for id, want := range tests {
		t.Run(id, func(t *testing.T) {
			code, body := doRequest(t, h, http.MethodGet, "/api/outlets/"+id+"/state", "")
			if code != http.StatusOK {
				t.Fatalf("status = %d, body = %q", code, body)
			}
			if body != want {
				t.Errorf("body = %q, want %q", body, want)
			}
		})
	}
}

func TestSetOutletState(t *testing.T) {
	fake, h := newTestServer(t)

	tests := []struct {
		id          string
		body        string
		switchIndex int
		want        bool
	}{
		{"1", "on", 0, true},
		{"1", "off", 0, false},
		{"sprinkler", "true", 0, true},
		{"sprinkler", "false", 0, false},
		{"8", "on", 7, true},
		{"outlet8", "off", 7, false},
	}
	for _, tc := range tests {
		code, body := doRequest(t, h, http.MethodPut, "/api/outlets/"+tc.id+"/state", tc.body)
		if code != http.StatusOK {
			t.Fatalf("PUT %s %q: status = %d, body = %q", tc.id, tc.body, code, body)
		}
		if got := fake.state(tc.switchIndex); got != tc.want {
			t.Errorf("PUT %s %q: switch index %d state = %v, want %v", tc.id, tc.body, tc.switchIndex, got, tc.want)
		}
	}
}

func TestSetOutletStateBadBody(t *testing.T) {
	fake, h := newTestServer(t)

	code, _ := doRequest(t, h, http.MethodPut, "/api/outlets/2/state", "maybe")
	if code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", code, http.StatusBadRequest)
	}
	if !fake.state(1) {
		t.Errorf("outlet changed despite bad body")
	}

	// Bad input is the client's fault even when the switch is down
	fake.failStatus = http.StatusInternalServerError
	code, _ = doRequest(t, h, http.MethodPut, "/api/outlets/2/state", "maybe")
	if code != http.StatusBadRequest {
		t.Errorf("switch down: status = %d, want %d", code, http.StatusBadRequest)
	}
}

func TestSetOutletStateSwitchRejects(t *testing.T) {
	fake, h := newTestServer(t)
	fake.failPutStatus = http.StatusForbidden

	code, _ := doRequest(t, h, http.MethodPut, "/api/outlets/1/state", "on")
	if code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", code, http.StatusInternalServerError)
	}
	if fake.state(0) {
		t.Errorf("outlet changed despite switch rejecting the PUT")
	}
}

func TestGetOutletName(t *testing.T) {
	_, h := newTestServer(t)

	tests := map[string]string{
		"1":            "sprinkler",
		"2":            "disco ball",
		"disco%20ball": "disco ball",
		"8":            "outlet8",
	}
	for id, want := range tests {
		code, body := doRequest(t, h, http.MethodGet, "/api/outlets/"+id+"/name", "")
		if code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, body = %q", id, code, body)
		}
		if body != want {
			t.Errorf("GET %s: body = %q, want %q", id, body, want)
		}
	}
}

func TestLookupEscapedName(t *testing.T) {
	outlets := defaultOutlets()
	outlets[4].Name = `hot & "cold"/50%`
	_, h := newTestServerWith(t, outlets...)

	code, body := doRequest(t, h, http.MethodGet, "/api/outlets/"+url.PathEscape(outlets[4].Name)+"/name", "")
	if code != http.StatusOK || body != outlets[4].Name {
		t.Errorf("status = %d, body = %q, want %q", code, body, outlets[4].Name)
	}
}

func TestSetOutletName(t *testing.T) {
	fake, h := newTestServer(t)

	tests := []struct {
		id          string
		body        string
		switchIndex int
		want        string
	}{
		{"3", "lava lamp", 2, "lava lamp"},
		{"lava%20lamp", "  heater\n", 2, "heater"},
		{"sprinkler", "hose", 0, "hose"},
	}
	for _, tc := range tests {
		code, body := doRequest(t, h, http.MethodPut, "/api/outlets/"+tc.id+"/name", tc.body)
		if code != http.StatusOK {
			t.Fatalf("PUT %s %q: status = %d, body = %q", tc.id, tc.body, code, body)
		}
		if got := fake.name(tc.switchIndex); got != tc.want {
			t.Errorf("PUT %s %q: name = %q, want %q", tc.id, tc.body, got, tc.want)
		}
	}

	// And the new name reads back and works for lookups
	code, body := doRequest(t, h, http.MethodGet, "/api/outlets/3/name", "")
	if code != http.StatusOK || body != "heater" {
		t.Errorf("GET outlet 3 name: status = %d, body = %q", code, body)
	}
	code, body = doRequest(t, h, http.MethodGet, "/api/outlets/heater/state", "")
	if code != http.StatusOK || body != "off" {
		t.Errorf("GET heater state: status = %d, body = %q", code, body)
	}
}

func TestSetOutletNameBadBody(t *testing.T) {
	fake, h := newTestServer(t)
	fake.failStatus = http.StatusInternalServerError

	// Rejected before the switch is contacted, so the switch being down
	// doesn't matter
	for _, body := range []string{"", "  \n"} {
		code, _ := doRequest(t, h, http.MethodPut, "/api/outlets/1/name", body)
		if code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, want %d", body, code, http.StatusBadRequest)
		}
	}
}

func TestSetOutletNameSwitchRejects(t *testing.T) {
	fake, h := newTestServer(t)
	fake.failPutStatus = http.StatusForbidden

	code, _ := doRequest(t, h, http.MethodPut, "/api/outlets/1/name", "hose")
	if code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", code, http.StatusInternalServerError)
	}
	if got := fake.name(0); got != "sprinkler" {
		t.Errorf("name = %q, want unchanged", got)
	}
}

func TestSwitchErrorsPropagate(t *testing.T) {
	fake, h := newTestServer(t)
	fake.failStatus = http.StatusInternalServerError

	// Includes a name lookup, which must not be reported as a bad id
	for _, path := range []string{"/api/outlets", "/api/outlets/1", "/api/outlets/sprinkler/state"} {
		code, _ := doRequest(t, h, http.MethodGet, path, "")
		if code != http.StatusInternalServerError {
			t.Errorf("GET %s: status = %d, want %d", path, code, http.StatusInternalServerError)
		}
	}
}

func TestGetBodyIsOn(t *testing.T) {
	tests := []struct {
		body    string
		want    bool
		wantErr bool
	}{
		{"on", true, false},
		{"true", true, false},
		{"off", false, false},
		{"false", false, false},
		{"ON", true, false},
		{"on\n", true, false},
		{" Off ", false, false},
		{"\tTRUE\r\n", true, false},
		{"", false, true},
		{"maybe", false, true},
		{"o n", false, true},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(tc.body))
		got, err := getBodyIsOn(req)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("getBodyIsOn(%q) = %v, %v; want %v, err=%v", tc.body, got, err, tc.want, tc.wantErr)
		}
	}
}
