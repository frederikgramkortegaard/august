package api

import (
	"gocuria/api/handlers"
	"gocuria/blockchain/store"
	"log"
	"net/http"
)

// Server represents the HTTP API server
type Server struct {
	store store.ChainStore
	port  string
	mux   *http.ServeMux
}

// NewServer creates a new API server
func NewServer(store store.ChainStore, port string) *Server {
	server := &Server{
		store: store,
		port:  port,
		mux:   http.NewServeMux(),
	}
	
	server.setupRoutes()
	return server
}

// setupRoutes configures all HTTP endpoints
func (s *Server) setupRoutes() {
	// Block endpoints
	s.mux.HandleFunc("/api/blocks", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleBlocks(w, r, s.store)
	})
	s.mux.HandleFunc("/api/blocks/", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleBlocks(w, r, s.store) // Handles /api/blocks/{hash}
	})

	// Chain endpoints
	s.mux.HandleFunc("/api/chain/height", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleChainHeight(w, r, s.store)
	})
	s.mux.HandleFunc("/api/chain/head", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleChainHead(w, r, s.store)
	})

	// Transaction endpoints
	s.mux.HandleFunc("/api/transactions", func(w http.ResponseWriter, r *http.Request) {
		handlers.HandleTransactions(w, r, s.store)
	})
}

// Start begins serving HTTP requests (blocks forever)
func (s *Server) Start() error {
	log.Printf("Starting HTTP API server on port %s", s.port)
	return http.ListenAndServe(":"+s.port, s.mux)
}

// StartServer is the legacy function for backwards compatibility
func StartServer(store store.ChainStore, httpPort string) {
	server := NewServer(store, httpPort)
	log.Fatal(server.Start())
}
