package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/config"
	"manatomb/app/internal/db"
	"manatomb/app/internal/decks"
	"manatomb/app/internal/web"
)

type methodRoute struct {
	pattern string
	get     http.HandlerFunc
	post    http.HandlerFunc
}

func ensureTables(ctx context.Context, database *sql.DB) error {
	if err := account.EnsureUserTable(ctx, database); err != nil {
		return fmt.Errorf("ensure users table: %w", err)
	}

	if err := account.EnsureSessionsTable(ctx, database); err != nil {
		return fmt.Errorf("ensure sessions table: %w", err)
	}

	if err := cards.EnsureCardsTable(ctx, database); err != nil {
		return fmt.Errorf("ensure cards table: %w", err)
	}

	if err := decks.EnsureDeckTables(ctx, database); err != nil {
		return fmt.Errorf("ensure deck/deck_cards tables: %w", err)
	}

	return nil
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

func registerMethodRoutes(mux *http.ServeMux, routes []methodRoute) {
	for _, rt := range routes {
		mux.HandleFunc(rt.pattern, methodSwitch(rt.get, rt.post))
	}
}

func registerHomeAndAuthRoutes(mux *http.ServeMux, app *web.App) {
	mux.HandleFunc("/", app.HandleHome)
	mux.HandleFunc("/logout", app.HandleLogout)

	registerMethodRoutes(mux, []methodRoute{
		{pattern: "/signup", get: app.HandleSignupShow, post: app.HandleSignupPost},
		{pattern: "/login", get: app.HandleLoginShow, post: app.HandleLoginPost},
	})
}

func registerSettingsRoutes(mux *http.ServeMux, app *web.App) {
	registerMethodRoutes(mux, []methodRoute{
		{pattern: "/settings", get: app.HandleSettingsShow, post: app.HandleSettingsPost},
	})
}

func registerDeckRoutes(mux *http.ServeMux, app *web.App) {
	mux.HandleFunc("/decks", app.HandleDecksList)
	mux.HandleFunc("/decks/delete", app.HandleDeckDeletePost)
	mux.HandleFunc("/decks/create-from-commander", app.HandleDeckCreateFromCommander)
	mux.HandleFunc("/decks/public", app.HandlePublicDecks)
	mux.HandleFunc("/decks/guest", app.HandleGuestDeckShow)
	mux.HandleFunc("/decks/analytics", app.HandleDeckAnalytics)
	mux.HandleFunc("/decks/import-draft", app.HandleDeckImportDraft)
	mux.HandleFunc("/decks/import-text", app.HandleDeckImportText)
	mux.HandleFunc("/decks/sandbox", app.HandleDeckSandboxWIP)
	mux.HandleFunc("/decks/commander", app.HandleDeckUpdateCommander)

	registerMethodRoutes(mux, []methodRoute{
		{pattern: "/decks/new", get: app.HandleDeckNewShow, post: app.HandleDeckNewPost},
		{pattern: "/decks/edit", get: app.HandleDeckEditShow, post: app.HandleDeckEditPost},
	})

	// Keep before "/decks/" catch-all.
	mux.HandleFunc("/decks/playtest/guest", app.HandleDeckPlaytestGuest)
	mux.HandleFunc("/decks/playtest/", app.HandleDeckPlaytest)
	mux.HandleFunc("/decks/", app.HandleDeckShow) // /decks/{id}
}

func registerCardAndRulesRoutes(mux *http.ServeMux, app *web.App) {
	mux.HandleFunc("/cards/search", app.HandleCardSearch)
	mux.HandleFunc("/cards/add-to-deck", app.HandleCardAddToDeck)
	mux.HandleFunc("/commanders/search", app.HandleCommanderSearch)
	mux.HandleFunc("/rules", app.HandleRulesHome)
}

// registerRoutes wires up all HTTP routes for the application.
func registerRoutes(mux *http.ServeMux, app *web.App) {
	registerHomeAndAuthRoutes(mux, app)
	registerSettingsRoutes(mux, app)
	registerDeckRoutes(mux, app)
	registerCardAndRulesRoutes(mux, app)
}

func wrapMiddleware(app *web.App, handler http.Handler) http.Handler {
	// Middleware order matters; last wrapper here is the outermost handler.
	handler = app.WithNotFoundMiddleware(handler)
	handler = app.WithUserMiddleware(handler)
	handler = app.WithRecoveryMiddleware(handler)
	return handler
}

func run() error {
	cfg := config.Load()
	database := db.Open(cfg.DatabaseURL)
	defer database.Close()

	if err := ensureTables(context.Background(), database); err != nil {
		return err
	}

	// Keep the local cards catalog fresh from Scryfall bulk data.
	syncCtx, cancelSync := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancelSync()
	due, err := cards.CardSyncDue(syncCtx, database, 24*time.Hour)
	if err != nil {
		log.Printf("cards sync due check failed: %v", err)
	} else if due {
		log.Printf("cards sync: starting startup bulk refresh")
		result, syncErr := cards.SyncCardsFromScryfallBulk(syncCtx, database)
		if syncErr != nil {
			log.Printf("cards sync: startup refresh failed: %v", syncErr)
		} else {
			log.Printf(
				"cards sync: startup refresh complete (%d cards, source updated %s)",
				result.ImportedCards,
				result.SourceUpdatedAt.UTC().Format(time.RFC3339),
			)
		}
	}
	cards.StartCardBulkSyncLoop(database, 24*time.Hour, log.Default())

	app := &web.App{
		DB:       database,
		Renderer: web.NewRenderer(),
	}

	mux := http.NewServeMux()
	registerRoutes(mux, app)

	handler := wrapMiddleware(app, mux)
	addr := ":" + cfg.Port
	log.Printf("Listening on %s...", addr)
	return http.ListenAndServe(addr, handler)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
