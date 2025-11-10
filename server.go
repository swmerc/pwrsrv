package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/go-chi/chi/v5"
)

// startServer just first up the server and never returns
func startServer(cfg Config) {
	s := server{localConfig: cfg.LocalServer, webSwitchConfig: DLIProSwitch{config: cfg.PowerServer}}
	s.run()
}

type server struct {
	localConfig LocalServerConfig
	webSwitchConfig DLIProSwitch
}

func (s *server) run() {
	router := chi.NewRouter()

	router.Route("/api/outlets", func(r chi.Router) {
		r.Get("/", s.getAllOutlets)
		r.Get("/{id}", s.getOutlet)
		r.Put("/{id}/state", s.setOutletState)
		r.Get("/{id}/state", s.getOutletState)
	})

	log.Infof("Starting server on port %d", s.localConfig.Port)
	srv := &http.Server{
		Handler:      logRequest(router),
		Addr:         fmt.Sprintf(":%d", s.localConfig.Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

// Simplified outlet with name and proper boolean
type Outlet struct {
	Name string `json:"name"`
	On   bool   `json:"on"`
}

func NewOutlet(webOutlet WebSwitchOutlet) Outlet {
	return Outlet{Name: webOutlet.Name, On: webOutlet.State }
}

func (s *server) getAllOutlets(w http.ResponseWriter, r *http.Request) {
	if webOutlets, err := s.webSwitchConfig.GetOutlets(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		outlets := make([]Outlet, 0, len(webOutlets))
		for _, webOutlet := range webOutlets {
			outlets = append(outlets, NewOutlet(webOutlet))
		}
		if body, err := json.Marshal(outlets); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(body)
		}
	}
}

func (s *server) getOutlet(w http.ResponseWriter, r *http.Request) {
	if index, err := strconv.Atoi(chi.URLParam(r, "id")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else if outlet, err := s.webSwitchConfig.GetOutlet(index); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else if body, err := json.Marshal(NewOutlet(outlet)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Write(body)
	}
}

func (s *server) getOutletState(w http.ResponseWriter, r *http.Request) {
	if index, err := s.getIndexFromIdOrName(r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else if outlet, err := s.webSwitchConfig.GetOutlet(index); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Write([]byte(fmt.Sprintf("%t", outlet.State)))
	}
}

func (s *server) setOutletState(w http.ResponseWriter, r *http.Request) {
	if index, err := s.getIndexFromIdOrName(r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else if on, err := getBodyIsOn(r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else if err := s.webSwitchConfig.SetOutlet(index, on); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Simple logging, which is applied to all requests
func logRequest(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Infof("request: %s: %s%s from %s", r.Method, r.Host, r.URL, r.RemoteAddr)
		h.ServeHTTP(w, r)
	})
}

// Translate bodys of on/true to true, off/false to false
func getBodyIsOn(r *http.Request) (bool, error) {
	result := false

	if bytes, err := io.ReadAll(r.Body); err != nil {
		return false, err
	} else {
		value := string(bytes)
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

func (s *server) getIndexFromIdOrName(r *http.Request) (int, error) {
	// Grab the id
	id := chi.URLParam(r, "id")

	// See if it is a port index
	if index, err := strconv.Atoi(id); err == nil {
		if index >= 0 && index <= s.webSwitchConfig.GetMaxIndex() {
			return index, nil
		}
	}

	// Look it up as a name.  This allows numbers for names, which is ... fun.
	if webOutlets, err := s.webSwitchConfig.GetOutlets(); err != nil {
		return -1, fmt.Errorf("unable to look up outlet name")
	} else {
		for i, outlet := range webOutlets {
			if outlet.Name == id {
				return i, nil
			}
		}
	}

	// Nope...
	return -1, fmt.Errorf("did not find outlet with name %s", id)
}

