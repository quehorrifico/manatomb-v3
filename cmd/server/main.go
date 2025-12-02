package main

import (
	"context"
	"log"
	"net/http"

	"manatomb/app/internal/account"
	"manatomb/app/internal/config"
	"manatomb/app/internal/db"
	"manatomb/app/internal/web"
)

func main() {
	cfg := config.Load()
	database := db.Open(cfg.DatabaseURL)
	defer database.Close()

	if err := account.EnsureUserTable(context.Background(), database); err != nil {
		log.Fatalf("failed to ensure users table: %v", err)
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

	// Wrap with middleware that injects CurrentUser
	handler := app.WithUserMiddleware(mux)

	addr := ":" + cfg.Port
	log.Printf("Listening on %s...", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
