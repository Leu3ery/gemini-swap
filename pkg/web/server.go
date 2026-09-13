package web

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"gemini-swap/pkg/account"
	"gemini-swap/pkg/quota"
	"gemini-swap/pkg/session"
)

//go:embed index.html
var IndexHTML string

type Server struct {
	storage *account.Storage
	port    int
}

func NewServer(storage *account.Storage, port int) *Server {
	return &Server{
		storage: storage,
		port:    port,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/accounts", func(w http.ResponseWriter, r *http.Request) {
		store, err := s.storage.Load()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(store)
	})

	mux.HandleFunc("/api/switch", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		acc, err := session.SwitchTo(s.storage, id, session.Options{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(acc)
	})

	mux.HandleFunc("/api/refresh", func(w http.ResponseWriter, r *http.Request) {
		store, err := s.storage.Load()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		sessID := quota.SessionAccountID(store)
		for _, a := range store.Accounts {
			isLive := a.ID == sessID
			if sessID == "" && a.ID == store.ActiveAccountID {
				isLive = true
			}
			_, _ = quota.FetchAccountQuota(a, isLive)
		}
		_ = s.storage.Save(store)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(store)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(IndexHTML))
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", s.port),
		Handler: mux,
	}

	log.Printf("[Web] Gemini Swap Web Dashboard running at http://0.0.0.0:%d", s.port)

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
