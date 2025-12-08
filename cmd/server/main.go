package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/config"
	"manatomb/app/internal/db"
	"manatomb/app/internal/decks"
	"manatomb/app/internal/web"
)

func ensureTables(ctx context.Context, database *sql.DB) {
	if err := account.EnsureUserTable(ctx, database); err != nil {
		log.Fatalf("failed to ensure users table: %v", err)
	}

	if err := account.EnsureSessionsTable(ctx, database); err != nil {
		log.Fatalf("failed to ensure sessions table: %v", err)
	}

	if err := cards.EnsureCardsTable(ctx, database); err != nil {
		log.Fatalf("failed to ensure cards table: %v", err)
	}

	if err := decks.EnsureDeckTables(ctx, database); err != nil {
		log.Fatalf("failed to ensure deck and deck_cards tables: %v", err)
	}
}

// methodSwitch returns a handler that dispatches to the given GET / POST handlers
// and responds with 405 for unsupported methods.
func methodSwitch(get, post http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if get != nil {
				get(w, r)
				return
			}
		case http.MethodPost:
			if post != nil {
				post(w, r)
				return
			}
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// registerRoutes wires up all HTTP routes for the application.
func registerRoutes(mux *http.ServeMux, app *web.App) {
	// Home / auth
	mux.HandleFunc("/", app.HandleHome)
	mux.HandleFunc("/signup", methodSwitch(app.HandleSignupShow, app.HandleSignupPost))
	mux.HandleFunc("/login", methodSwitch(app.HandleLoginShow, app.HandleLoginPost))
	mux.HandleFunc("/logout", app.HandleLogout)

	// User settings
	mux.HandleFunc("/settings", methodSwitch(app.HandleSettingsShow, app.HandleSettingsPost))

	// Decks
	mux.HandleFunc("/decks", app.HandleDecksList)
	mux.HandleFunc("/decks/new", methodSwitch(app.HandleDeckNewShow, app.HandleDeckNewPost))
	mux.HandleFunc("/decks/edit", methodSwitch(app.HandleDeckEditShow, app.HandleDeckEditPost))
	mux.HandleFunc("/decks/delete", app.HandleDeckDeletePost)
	mux.HandleFunc("/decks/create-from-commander", app.HandleDeckCreateFromCommander)
	mux.HandleFunc("/decks/public", app.HandlePublicDecks)
	mux.HandleFunc("/decks/", app.HandleDeckShow) // /decks/{id}

	// Cards / commanders / rules
	mux.HandleFunc("/cards/search", app.HandleCardSearch)
	mux.HandleFunc("/cards/add-to-deck", app.HandleCardAddToDeck)
	mux.HandleFunc("/commanders/search", app.HandleCommanderSearch)
	mux.HandleFunc("/rules", app.HandleRulesHome)
}

func main() {
	cfg := config.Load()
	database := db.Open(cfg.DatabaseURL)
	defer database.Close()

	ensureTables(context.Background(), database)

	renderer := web.NewRenderer()
	app := &web.App{
		DB:       database,
		Renderer: renderer,
	}

	mux := http.NewServeMux()
	registerRoutes(mux, app)

	// Wrap with middleware (outermost last)
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
