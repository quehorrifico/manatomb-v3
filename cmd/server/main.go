package main

import (
	"context"
	"log"
	"net/http"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/config"
	"manatomb/app/internal/db"
	"manatomb/app/internal/decks"
	"manatomb/app/internal/web"
)

func main() {
	cfg := config.Load()
	database := db.Open(cfg.DatabaseURL)
	defer database.Close()

	// ✅ Ensure all required tables exist
	if err := account.EnsureUserTable(context.Background(), database); err != nil {
		log.Fatalf("failed to ensure users table: %v", err)
	}

	if err := account.EnsureSessionsTable(context.Background(), database); err != nil {
		log.Fatalf("failed to ensure sessions table: %v", err)
	}

	if err := cards.EnsureCardsTable(context.Background(), database); err != nil {
		log.Fatalf("failed to ensure cards table: %v", err)
	}

	if err := decks.EnsureDeckTables(context.Background(), database); err != nil {
		log.Fatalf("failed to ensure deck and deck_cards tables: %v", err)
	}

	renderer := web.NewRenderer()
	app := &web.App{
		DB:       database,
		Renderer: renderer,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.HandleHome)
	mux.HandleFunc("/signup", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			app.HandleSignupShow(w, r)
		case http.MethodPost:
			app.HandleSignupPost(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			app.HandleLoginShow(w, r)
		} else if r.Method == http.MethodPost {
			app.HandleLoginPost(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/logout", app.HandleLogout)

	mux.HandleFunc("/decks", app.HandleDecksList)
	mux.HandleFunc("/decks/new", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			app.HandleDeckNewShow(w, r)
		} else if r.Method == http.MethodPost {
			app.HandleDeckNewPost(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/decks/edit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			app.HandleDeckEditShow(w, r)
		} else if r.Method == http.MethodPost {
			app.HandleDeckEditPost(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/decks/delete", app.HandleDeckDeletePost)
	mux.HandleFunc("/decks/", app.HandleDeckShow) // /decks/{id}

	mux.HandleFunc("/cards/search", app.HandleCardSearch)
	mux.HandleFunc("/cards/add-to-deck", app.HandleCardAddToDeck)
	mux.HandleFunc("/commanders/search", app.HandleCommanderSearch)

	// Wrap with middleware (NotFound → User → Recovery)
	var handler http.Handler = mux
	handler = app.WithNotFoundMiddleware(handler)
	handler = app.WithUserMiddleware(handler)
	handler = app.WithRecoveryMiddleware(handler)

	addr := ":" + cfg.Port
	log.Printf("Listening on %s...", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
