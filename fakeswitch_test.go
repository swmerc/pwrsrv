package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeSwitch is a minimal stand-in for the DLI Pro Switch REST API.  It only
// implements the handful of endpoints that ProSwitch actually uses.
type fakeSwitch struct {
	t *testing.T

	mu      sync.Mutex
	outlets []ProSwitchOutlet

	// Last PUT seen, for checking what the client sent
	lastPutPath string
	lastPutBody string
	lastHeaders http.Header
	putCount    int

	// When set, every request fails with this status
	failStatus int

	// When set, only PUTs fail with this status
	failPutStatus int
}

func newFakeSwitch(t *testing.T, outlets ...ProSwitchOutlet) (*fakeSwitch, *httptest.Server) {
	t.Helper()

	f := &fakeSwitch{t: t, outlets: outlets}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	return f, srv
}

// defaultOutlets mirrors the 8 outlets on a real switch.  Names follow the
// hardware labels, so switch index 0 is "outlet1".
func defaultOutlets() []ProSwitchOutlet {
	outlets := make([]ProSwitchOutlet, 8)
	for i := range outlets {
		outlets[i] = ProSwitchOutlet{Name: fmt.Sprintf("outlet%d", i+1)}
	}
	outlets[0].Name = "sprinkler"
	outlets[1].Name = "disco ball"
	outlets[1].State = true
	return outlets
}

func (f *fakeSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastHeaders = r.Header.Clone()

	if f.failStatus != 0 {
		http.Error(w, "nope", f.failStatus)
		return
	}

	const prefix = "/restapi/relay/outlets/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")

	// GET /restapi/relay/outlets/
	if parts[0] == "" {
		if r.Method != http.MethodGet {
			http.Error(w, "bad method", http.StatusMethodNotAllowed)
			return
		}
		json.NewEncoder(w).Encode(f.outlets)
		return
	}

	index, err := strconv.Atoi(parts[0])
	if err != nil || index < 0 || index >= len(f.outlets) {
		http.NotFound(w, r)
		return
	}

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		json.NewEncoder(w).Encode(f.outlets[index])

	case len(parts) == 2 && parts[1] == "state" && r.Method == http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		f.putCount++
		f.lastPutPath = r.URL.Path
		f.lastPutBody = string(body)
		if f.failPutStatus != 0 {
			http.Error(w, "nope", f.failPutStatus)
			return
		}
		switch string(body) {
		case "value=true":
			f.outlets[index].State = true
		case "value=false":
			f.outlets[index].State = false
		default:
			http.Error(w, "bad value", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.NotFound(w, r)
	}
}

func (f *fakeSwitch) puts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.putCount
}

func (f *fakeSwitch) state(index int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.outlets[index].State
}
