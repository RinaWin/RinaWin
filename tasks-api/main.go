package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func mustEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func main() {
	ctx := context.Background()

	dsn := mustEnv("DATABASE_URL", "postgres://app:secret@localhost:5432/app?sslmode=disable")
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("cannot create pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("cannot reach database: %v", err)
	}

	h := &TaskHandler{store: NewTaskStore(pool)}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /tasks", h.list)
	mux.HandleFunc("POST /tasks", h.create)
	mux.HandleFunc("GET /tasks/{id}", h.getOne)
	mux.HandleFunc("DELETE /tasks/{id}", h.remove)

	addr := ":" + mustEnv("PORT", "8080")
	log.Printf("listening on %s", addr)

	srv := &http.Server{
		Addr:         addr,
		Handler:      withLogging(withCORS(mux)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}