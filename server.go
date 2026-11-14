package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/go-chi/chi/v5"
)

// startServer just fires up the server and never returns
func startServer(cfg Config) {
	s := server{localConfig: cfg.LocalServer, webSwitchConfig: NewProSwitch(cfg.ProSwitch)}
	s.run()
}

type server struct {
	localConfig     LocalServerConfig
	webSwitchConfig *ProSwitch
}

func (s *server) routes() http.Handler {
	router := chi.NewRouter()

	router.Route("/api/outlets", func(r chi.Router) {
		r.Get("/", s.getAllOutlets)
		r.Get("/{id}", s.getOutlet)
		r.Put("/{id}/state", s.setOutletState)
		r.Get("/{id}/state", s.getOutletState)
		r.Get("/{id}/name", s.getOutletName)
		r.Put("/{id}/name", s.setOutletName)
	})

	return router
}

func (s *server) run() {
	addr := net.JoinHostPort(s.localConfig.Address, strconv.FormatUint(uint64(s.localConfig.Port), 10))

	log.Infof("Starting server on %s", addr)
	srv := &http.Server{
		Handler:      logRequest(s.routes()),
		Addr:         addr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

// Simplified outlet with name and proper boolean.  Index is the 1-based
// number printed on the hardware, not the 0-based index the switch API uses.
type Outlet struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	On    bool   `json:"on"`
}

func NewOutlet(webOutlet ProSwitchOutlet, switchIndex int) Outlet {
	return Outlet{Index: switchIndex + 1, Name: webOutlet.Name, On: webOutlet.State}
}

func (s *server) getAllOutlets(w http.ResponseWriter, r *http.Request) {
	if webOutlets, err := s.webSwitchConfig.GetOutlets(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		outlets := make([]Outlet, 0, len(webOutlets))
		for i, webOutlet := range webOutlets {
			outlets = append(outlets, NewOutlet(webOutlet, i))
		}

		writeJSON(w, outlets)
	}
}

func (s *server) getOutlet(w http.ResponseWriter, r *http.Request) {
	if index, outlet, err := s.lookupOutlet(r); err != nil {
		writeLookupError(w, err)
	} else {
		writeJSON(w, NewOutlet(outlet, index))
	}
}

func (s *server) getOutletState(w http.ResponseWriter, r *http.Request) {
	if _, outlet, err := s.lookupOutlet(r); err != nil {
		writeLookupError(w, err)
	} else {
		state := "off"
		if outlet.State {
			state = "on"
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(state))
	}
}

func (s *server) setOutletState(w http.ResponseWriter, r *http.Request) {
	// Check the body first so bad input doesn't cost a trip to the switch
	if on, err := getBodyIsOn(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else if index, _, err := s.lookupOutlet(r); err != nil {
		writeLookupError(w, err)
	} else if err := s.webSwitchConfig.SetOutletState(r.Context(), index, on); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *server) getOutletName(w http.ResponseWriter, r *http.Request) {
	if _, outlet, err := s.lookupOutlet(r); err != nil {
		writeLookupError(w, err)
	} else {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(outlet.Name))
	}
}

func (s *server) setOutletName(w http.ResponseWriter, r *http.Request) {
	// Check the body first so bad input doesn't cost a trip to the switch
	if name, err := getBodyName(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else if index, _, err := s.lookupOutlet(r); err != nil {
		writeLookupError(w, err)
	} else if err := s.webSwitchConfig.SetOutletName(r.Context(), index, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	if body, err := json.Marshal(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}
}

// Simple logging, which is applied to all requests
func logRequest(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Infof("request: %s: %s%s from %s", r.Method, r.Host, r.URL, r.RemoteAddr)
		h.ServeHTTP(w, r)
	})
}

// Translate bodys of on/true to true, off/false to false, ignoring case and
// surrounding whitespace
func getBodyIsOn(r *http.Request) (bool, error) {
	result := false

	if bytes, err := io.ReadAll(r.Body); err != nil {
		return false, err
	} else {
		value := strings.ToLower(strings.TrimSpace(string(bytes)))
		switch value {
		case "true", "on":
			result = true
		case "false", "off":
			result = false
		default:
			return false, fmt.Errorf("unexpected status")
		}
	}

	return result, nil
}

// The new name is the whole body, minus surrounding whitespace
func getBodyName(r *http.Request) (string, error) {
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	name := strings.TrimSpace(string(bytes))
	if len(name) == 0 {
		return "", fmt.Errorf("empty name")
	}

	return name, nil
}

var errOutletNotFound = errors.New("outlet not found")

// writeLookupError reports a failure from lookupOutlet, blaming the client
// for a bad id and the switch for anything else
func writeLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, errOutletNotFound) {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// lookupOutlet finds the outlet for the id in the URL, which is either a
// 1-based outlet number matching the hardware labels or an outlet name.  It
// checks against the outlets the switch actually has and returns the 0-based
// index the switch API uses along with the outlet itself.
func (s *server) lookupOutlet(r *http.Request) (int, ProSwitchOutlet, error) {
	// Grab the id.  chi hands back the raw path segment when the request
	// escapes anything unusual (say "%26" for "&"), so unescape it to match
	// names with those characters in them.
	id, err := url.PathUnescape(chi.URLParam(r, "id"))
	if err != nil {
		return -1, ProSwitchOutlet{}, fmt.Errorf("%w: %s", errOutletNotFound, chi.URLParam(r, "id"))
	}

	webOutlets, err := s.webSwitchConfig.GetOutlets(r.Context())
	if err != nil {
		return -1, ProSwitchOutlet{}, fmt.Errorf("unable to get outlets: %w", err)
	}

	// See if it is an outlet number, written plainly (no "02" or "+2")
	if number, err := strconv.Atoi(id); err == nil && strconv.Itoa(number) == id {
		if number >= 1 && number <= len(webOutlets) {
			return number - 1, webOutlets[number-1], nil
		}
	}

	// Look it up as a name.  This allows numbers for names, which is ... fun.
	for i, outlet := range webOutlets {
		if outlet.Name == id {
			return i, outlet, nil
		}
	}

	// Nope...
	return -1, ProSwitchOutlet{}, fmt.Errorf("%w: %s", errOutletNotFound, id)
}
