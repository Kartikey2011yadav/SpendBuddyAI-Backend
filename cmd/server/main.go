package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/kartikeyyadav/spendbuddy/internal/auth"
	"github.com/kartikeyyadav/spendbuddy/internal/chat"
	"github.com/kartikeyyadav/spendbuddy/internal/delivery/http/handler"
	delivery "github.com/kartikeyyadav/spendbuddy/internal/delivery/http"
	"github.com/kartikeyyadav/spendbuddy/internal/expense"
	"github.com/kartikeyyadav/spendbuddy/internal/repository/postgres"
	"github.com/kartikeyyadav/spendbuddy/pkg/config"
	"github.com/kartikeyyadav/spendbuddy/pkg/database"
)

func main() {
	// Load .env (ignore error in production where vars come from the environment)
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	// ── Infrastructure ─────────────────────────────────────────────────────────
	db, err := database.NewPostgresPool(cfg.DB)
	if err != nil {
		slog.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	rdb, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		slog.Error("connect redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// ── Repositories ───────────────────────────────────────────────────────────
	userRepo    := postgres.NewUserRepository(db)
	groupRepo   := postgres.NewGroupRepository(db)
	msgRepo     := postgres.NewMessageRepository(db)
	expenseRepo := postgres.NewExpenseRepository(db)

	// ── Services ───────────────────────────────────────────────────────────────
	jwtSvc    := auth.NewJWTService(cfg.JWT)
	googleSvc := auth.NewGoogleAuthService(cfg.Google, userRepo, jwtSvc)
	otpSvc    := auth.NewOTPService(cfg.SMTP, rdb, userRepo, jwtSvc, newSMTPMailer(cfg.SMTP))
	expSvc    := expense.NewService(expenseRepo, groupRepo)

	// ── WebSocket Hub ──────────────────────────────────────────────────────────
	hub := chat.NewHub()
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()
	go hub.Run(hubCtx)

	// ── Handlers ───────────────────────────────────────────────────────────────
	authH    := handler.NewAuthHandler(googleSvc, otpSvc, jwtSvc)
	chatH    := handler.NewChatHandler(hub, msgRepo, groupRepo)
	expH     := handler.NewExpenseHandler(expSvc, groupRepo, hub)

	// ── Router ─────────────────────────────────────────────────────────────────
	e := delivery.NewRouter(jwtSvc, authH, chatH, expH)

	// ── Graceful shutdown ──────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      e,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "port", cfg.App.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}
