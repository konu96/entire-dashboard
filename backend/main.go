package main

import (
	"entire-dashboard/db"
	"entire-dashboard/handlers"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	repoPath := flag.String("repo", "", "Path to git repository (optional, can be added from UI)")
	port := flag.String("port", "8080", "Server port")
	flag.Parse()

	// DB stored in ~/.entire-dashboard/
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("get home dir: %v", err)
	}
	dataDir := filepath.Join(homeDir, ".entire-dashboard")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	dbPath := filepath.Join(dataDir, "dashboard.db")

	store, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer store.Close()

	// If --repo is provided, register it as a repository
	if *repoPath != "" {
		absRepo, err := filepath.Abs(*repoPath)
		if err != nil {
			log.Fatalf("resolve repo path: %v", err)
		}
		name := filepath.Base(absRepo)
		if _, err := store.AddRepo(absRepo, name); err != nil {
			// Ignore duplicate error (already registered)
			log.Printf("repo registration: %v (may already exist)", err)
		}
	}

	h := handlers.New(store)

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/repos", h.GetRepos)
	mux.HandleFunc("POST /api/repos", h.AddRepo)
	mux.HandleFunc("DELETE /api/repos/{id}", h.DeleteRepo)
	mux.HandleFunc("GET /api/daily-stats", h.GetDailyStats)
	mux.HandleFunc("GET /api/sessions", h.GetSessions)
	mux.HandleFunc("POST /api/sync", h.Sync)

	// Serve frontend static files
	frontendDir := filepath.Join(filepath.Dir(os.Args[0]), "..", "frontend", "dist")
	if _, err := os.Stat(frontendDir); err == nil {
		mux.Handle("/", http.FileServer(http.Dir(frontendDir)))
	} else {
		// Fallback: serve from relative path during development
		devFrontend := filepath.Join(".", "..", "frontend", "dist")
		if _, err := os.Stat(devFrontend); err == nil {
			mux.Handle("/", http.FileServer(http.Dir(devFrontend)))
		}
	}

	// CORS middleware for development
	handler := corsMiddleware(mux)

	log.Printf("Starting server on :%s", *port)
	log.Fatal(http.ListenAndServe(":"+*port, handler))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
