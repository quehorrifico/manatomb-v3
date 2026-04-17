package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"manatomb/app/internal/account"
	"manatomb/app/internal/cards"
	"manatomb/app/internal/config"
	"manatomb/app/internal/db"
	"manatomb/app/internal/decks"
	"manatomb/app/internal/quickbuild"
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

	if err := quickbuild.EnsureTables(ctx, database); err != nil {
		return fmt.Errorf("ensure quick build tables: %w", err)
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
	mux.HandleFunc("/healthz", app.HandleHealthz)
	mux.HandleFunc("/privacy", app.HandlePrivacy)
	mux.HandleFunc("/terms", app.HandleTerms)
	mux.HandleFunc("/users/", app.HandleProfileShow)

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

func registerDeckCompatibilityRoutes(mux *http.ServeMux, app *web.App) {
	mux.HandleFunc("/decks/new/leader", app.HandleDeckCommanderSelect)
	mux.HandleFunc("/decks/select-leader", app.HandleDeckCommanderSelect)
	mux.HandleFunc("/decks/create-from-leader", app.HandleDeckCommanderSelect)
	mux.HandleFunc("/decks/leader", app.HandleDeckCommanderUpdate)
	mux.HandleFunc("/decks/workbench", app.HandleDeckWorkbenchAliasRedirect)
	mux.HandleFunc("/decks/guest", app.HandleDeckWorkbenchAliasRedirect)
	mux.HandleFunc("/decks/import-draft", app.HandleDeckImportDraft)
	mux.HandleFunc("/decks/import-text", app.HandleDeckImportText)
	mux.HandleFunc("/decks/sandbox", app.HandleDeckSandboxRedirect)
	mux.HandleFunc("/decks/playtest/workbench", app.HandleDeckWorkbenchPlaytest)
	mux.HandleFunc("/decks/playtest/guest", app.HandleDeckWorkbenchPlaytest)

	registerMethodRoutes(mux, []methodRoute{
		{pattern: "/decks/edit", get: app.HandleDeckEditShow, post: app.HandleDeckEditPost},
	})
}

func registerDeckRoutes(mux *http.ServeMux, app *web.App) {
	mux.HandleFunc("/decks", app.HandleDeckList)
	mux.HandleFunc("/decks/delete", app.HandleDeckDeletePost)
	mux.HandleFunc("/decks/public", app.HandlePublicDecks)
	mux.HandleFunc("/decks/public/fork", app.HandlePublicDeckForkPost)
	mux.HandleFunc("/decks/public/", app.HandlePublicDeckShow)
	mux.HandleFunc("/decks/analytics", app.HandleDeckAnalytics)
	mux.HandleFunc("/decks/new/commander/", app.HandleDeckNewCommanderShow)
	mux.HandleFunc("/decks/new/workbench", app.HandleDeckWorkbench)
	mux.HandleFunc("/decks/new/sandbox", app.HandleDeckSandboxRedirect)
	mux.HandleFunc("/decks/new/playtest", app.HandleDeckWorkbenchPlaytest)
	mux.HandleFunc("/decks/import/save", app.HandleDeckImportDraft)
	mux.HandleFunc("/decks/commander", app.HandleDeckCommanderUpdate)

	registerMethodRoutes(mux, []methodRoute{
		{pattern: "/decks/new", get: app.HandleDeckNewShow, post: app.HandleDeckNewPost},
		{pattern: "/decks/new/commander", get: app.HandleDeckNewCommanderRedirect, post: app.HandleDeckCommanderSelect},
		{pattern: "/decks/new/commander/create", post: app.HandleDeckCommanderSelect},
		{pattern: "/decks/import", get: app.HandleDeckImportShow, post: app.HandleDeckImportText},
		{pattern: "/decks/settings", get: app.HandleDeckEditShow, post: app.HandleDeckEditPost},
		{pattern: "/decks/quick-build", post: app.HandleDeckQuickBuild},
	})

	registerDeckCompatibilityRoutes(mux, app)

	mux.HandleFunc("/decks/playtest/", app.HandleDeckPlaytest)
	mux.HandleFunc("/decks/", app.HandleDeckShow) // /decks/{id}
}

func registerCardCompatibilityRoutes(mux *http.ServeMux, app *web.App) {
	mux.HandleFunc("/cards/search/autocomplete", app.HandleCardAutocomplete)
	mux.HandleFunc("/cards/search/deck", app.HandleCardAutocomplete)
}

func registerCardAndRulesRoutes(mux *http.ServeMux, app *web.App) {
	mux.HandleFunc("/cards", app.HandleCardList)
	mux.HandleFunc("/cards/autocomplete", app.HandleCardAutocomplete)
	mux.HandleFunc("/cards/search", app.HandleCardSearch)
	mux.HandleFunc("/cards/view/", app.HandleCardShow)
	mux.HandleFunc("/cards/resolve", app.HandleCardResolve)
	mux.HandleFunc("/cards/versions", app.HandleCardVersions)
	mux.HandleFunc("/commanders/search", app.HandleCommanderSearch)
	mux.HandleFunc("/rules", app.HandleRulesHome)

	registerCardCompatibilityRoutes(mux, app)
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

type runOptions struct {
	syncNow bool
}

func startCardSyncWorkers(database *sql.DB, enabled bool, runOnStart bool, options cards.CardBulkSyncOptions) {
	if !enabled {
		log.Printf("cards sync disabled (CARD_SYNC_ENABLED=false)")
		return
	}
	if !runOnStart {
		due, err := cards.CardSyncDue(context.Background(), database, 24*time.Hour)
		if err != nil {
			log.Printf("cards sync startup due-check failed: %v", err)
		} else if due {
			runOnStart = true
			log.Printf("cards sync: immediate startup sync required because local card data is missing or outdated")
		}
	}
	if options.MaxRows > 0 {
		log.Printf("cards sync running in limited mode (CARD_SYNC_MAX_ROWS=%d)", options.MaxRows)
	}
	cards.StartCardBulkSyncLoop(database, 24*time.Hour, runOnStart, log.Default(), options)
}

func run(opts runOptions) error {
	cfg := config.Load()
	database := db.Open(cfg.DatabaseURL)
	defer database.Close()

	if err := ensureTables(context.Background(), database); err != nil {
		return err
	}
	startCardSyncWorkers(database, cfg.CardSyncOn, cfg.CardSyncOnStart || opts.syncNow, cards.CardBulkSyncOptions{
		MaxRows: cfg.CardSyncMaxRows,
	})

	app := &web.App{
		DB:                  database,
		Renderer:            web.NewRenderer(),
		SessionCookieSecure: cfg.SessionCookieSecure,
	}

	mux := http.NewServeMux()
	registerRoutes(mux, app)

	handler := wrapMiddleware(app, mux)
	addr := ":" + cfg.Port
	log.Printf("Listening on %s...", addr)
	return http.ListenAndServe(addr, handler)
}

func main() {
	var opts runOptions
	flag.BoolVar(&opts.syncNow, "sync-now", false, "run a full Scryfall bulk sync immediately on startup")
	flag.Parse()

	if err := run(opts); err != nil {
		log.Fatal(err)
	}
}
