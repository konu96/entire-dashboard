package main

import (
	"entire-dashboard/db"
	"entire-dashboard/generated"
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

	ogenServer, err := generated.NewServer(h)
	if err != nil {
		log.Fatalf("create ogen server: %v", err)
	}

	// Resolve frontend static file directory
	var fileServer http.Handler
	frontendDir := filepath.Join(filepath.Dir(os.Args[0]), "..", "frontend", "dist")
	if _, err := os.Stat(frontendDir); err == nil {
		fileServer = http.FileServer(http.Dir(frontendDir))
	} else {
		devFrontend := filepath.Join(".", "..", "frontend", "dist")
		if _, err := os.Stat(devFrontend); err == nil {
			fileServer = http.FileServer(http.Dir(devFrontend))
		}
	}

	// Route /api/* to ogen, everything else to static files
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			ogenServer.ServeHTTP(w, r)
			return
		}
		if fileServer != nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})

	// CORS middleware for development
	handler := corsMiddleware(root)

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
