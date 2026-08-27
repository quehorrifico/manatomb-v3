package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
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

	if err := web.EnsureFeatureTables(ctx, database); err != nil {
		return fmt.Errorf("ensure feature tables: %w", err)
	}

	return nil
}

// methodSwitch returns a handler that dispatches to the given GET / POST handlers
// and responds with 405 for unsupported methods.
func methodSwitch(get, post http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead:
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
		allowed := make([]string, 0, 3)
		if get != nil {
			allowed = append(allowed, http.MethodGet, http.MethodHead)
		}
		if post != nil {
			allowed = append(allowed, http.MethodPost)
		}
		w.Header().Set("Allow", strings.Join(allowed, ", "))
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
	mux.HandleFunc("/robots.txt", app.HandleRobotsTXT)
	mux.HandleFunc("/sitemap.xml", app.HandleSitemapXML)
	mux.HandleFunc("/healthz", app.HandleHealthz)
	mux.HandleFunc("/changelog", app.HandleChangelog)
	mux.HandleFunc("/privacy", app.HandlePrivacy)
	mux.HandleFunc("/terms", app.HandleTerms)
	mux.HandleFunc("/users/", app.HandleProfileShow)

	registerMethodRoutes(mux, []methodRoute{
		{pattern: "/logout", post: app.HandleLogout},
		{pattern: "/signup", get: app.HandleSignupShow, post: app.HandleSignupPost},
		{pattern: "/login", get: app.HandleLoginShow, post: app.HandleLoginPost},
		{pattern: "/forgot-password", get: app.HandleForgotPasswordShow, post: app.HandleForgotPasswordPost},
		{pattern: "/reset-password", get: app.HandleResetPasswordShow, post: app.HandleResetPasswordPost},
		{pattern: "/profile/avatar", post: app.HandleProfileAvatarPost},
		{pattern: "/profile/art", post: app.HandleProfileArtPost},
		{pattern: "/cards/favorites/printing", post: app.HandleCardPrintingFavoritePost},
		{pattern: "/games/guess-card", get: app.HandleGuessCardShow, post: app.HandleGuessCardPost},
		{pattern: "/games/spellify", get: app.HandleSpellifyShow, post: app.HandleSpellifyPost},
		{pattern: "/games/pack-opening", get: app.HandlePackOpeningShow, post: app.HandlePackOpeningPost},
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
		{pattern: "/decks/new/commander/more", get: app.HandleDeckNewCommanderMore},
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
	mux.HandleFunc("/cards/random", app.HandleRandomCard)
	mux.HandleFunc("/cards/view/", app.HandleCardShow)
	mux.HandleFunc("/cards/resolve", app.HandleCardResolve)
	mux.HandleFunc("/cards/versions", app.HandleCardVersions)
	mux.HandleFunc("/commanders/search", app.HandleCommanderSearch)
	mux.HandleFunc("/rules", app.HandleRulesHome)

	registerCardCompatibilityRoutes(mux, app)
}

// registerRoutes wires up all HTTP routes for the application.
func registerRoutes(mux *http.ServeMux, app *web.App) {
	mux.Handle("/assets/", web.AssetHandler())
	registerHomeAndAuthRoutes(mux, app)
	registerSettingsRoutes(mux, app)
	registerDeckRoutes(mux, app)
	registerCardAndRulesRoutes(mux, app)
}

func wrapMiddleware(app *web.App, handler http.Handler) http.Handler {
	// Middleware order matters; last wrapper here is the outermost handler.
	handler = app.WithNotFoundMiddleware(handler)
	handler = app.WithUserMiddleware(handler)
	handler = app.WithRateLimitMiddleware(handler)
	handler = app.WithRecoveryMiddleware(handler)
	handler = app.WithSecurityHeadersMiddleware(handler)
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
		Renderer:            web.NewRenderer(cfg.PublicBaseURL),
		SessionCookieSecure: cfg.SessionCookieSecure,
		PublicBaseURL:       cfg.PublicBaseURL,
		TrustedProxyHops:    cfg.TrustedProxyHops,
		PasswordResetMailer: web.NewSMTPPasswordResetMailer(web.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
		}),
	}

	mux := http.NewServeMux()
	registerRoutes(mux, app)

	handler := wrapMiddleware(app, mux)
	addr := ":" + cfg.Port
	log.Printf("Listening on %s...", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	shutdownSignal, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	shutdownResult := make(chan error, 1)
	go func() {
		<-shutdownSignal.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		shutdownResult <- server.Shutdown(shutdownContext)
	}()

	serveErr := server.ListenAndServe()
	if shutdownSignal.Err() != nil {
		if shutdownErr := <-shutdownResult; shutdownErr != nil {
			return fmt.Errorf("graceful server shutdown: %w", shutdownErr)
		}
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}
	return serveErr
}

func main() {
	var opts runOptions
	flag.BoolVar(&opts.syncNow, "sync-now", false, "run a full Scryfall bulk sync immediately on startup")
	flag.Parse()

	if err := run(opts); err != nil {
		log.Fatal(err)
	}
}
