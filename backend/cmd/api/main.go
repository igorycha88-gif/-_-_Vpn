package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"smarttraffic/internal/config"
	"smarttraffic/internal/handlers"
	"smarttraffic/internal/middleware"
	"smarttraffic/internal/repository"
	"smarttraffic/internal/services"
	"smarttraffic/migrations"

	"github.com/go-chi/chi/v5"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("ошибка загрузки конфигурации", "error", err)
		os.Exit(1)
	}

	db, err := repository.InitDB(cfg.DB.Path, migrations.Files)
	if err != nil {
		logger.Error("ошибка инициализации БД", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	peerRepo := repository.NewPeerRepository(db)
	routeRepo := repository.NewRouteRepository(db)
	presetRepo := repository.NewPresetRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	dnsRepo := repository.NewDNSRepository(db)
	authRepo := repository.NewAuthRepository(db)

	authSvc := services.NewAuthService(authRepo, &cfg.JWT, logger)
	wgSvc := services.NewWireGuardService(peerRepo, &cfg.WG, logger)
	singboxSvc := services.NewSingBoxService(routeRepo, dnsRepo, &cfg.SingBox, &cfg.WG, &cfg.Server, logger)
	routingSvc := services.NewRoutingService(routeRepo, presetRepo, logger)
	dnsSvc := services.NewDNSService(dnsRepo, logger)
	trafficSvc := services.NewTrafficService(trafficRepo, peerRepo, logger)

	if err := wgSvc.SyncAllPeers(context.Background()); err != nil {
		logger.Warn("не удалось синхронизировать пиры WG при запуске", "error", err)
	}

	authHandler := handlers.NewAuthHandler(authSvc, logger)
	peerHandler := handlers.NewPeerHandler(wgSvc, singboxSvc, logger)
	routeHandler := handlers.NewRouteHandler(routingSvc, singboxSvc, logger)
	presetHandler := handlers.NewPresetHandler(routingSvc, singboxSvc, presetRepo, logger)
	dnsHandler := handlers.NewDNSHandler(dnsSvc, logger)
	serverHandler := handlers.NewServerHandler(trafficSvc, logger)
	monitoringHandler := handlers.NewMonitoringHandler(trafficSvc, logger)

	rateLimiter := middleware.NewRateLimiter(1, time.Second, 5)

	r := chi.NewRouter()
	r.Use(middleware.CORS(cfg.CORS.AllowedOrigins))
	r.Use(middleware.Logging)

	r.Get("/health", serverHandler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", rateLimiter.Middleware(http.HandlerFunc(authHandler.Login)).ServeHTTP)
		r.Post("/auth/refresh", authHandler.Refresh)

		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(authSvc))

			r.Get("/auth/session", authHandler.Session)
			r.Post("/auth/logout", authHandler.Logout)
			r.Post("/auth/logout-all", authHandler.LogoutAll)

			r.Route("/wg/peers", func(r chi.Router) {
				r.Get("/", peerHandler.List)
				r.Post("/", peerHandler.Create)
				r.Post("/sync", peerHandler.Sync)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", peerHandler.Get)
					r.Delete("/", peerHandler.Delete)
					r.Get("/config", peerHandler.DownloadConfig)
					r.Get("/qr", peerHandler.GetQRCode)
					r.Get("/stats", peerHandler.GetStats)
					r.Put("/toggle", peerHandler.Toggle)
				})
			})

			r.Route("/routes", func(r chi.Router) {
				r.Get("/", routeHandler.List)
				r.Post("/", routeHandler.Create)
				r.Put("/reorder", routeHandler.Reorder)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", routeHandler.Get)
					r.Put("/", routeHandler.Update)
					r.Delete("/", routeHandler.Delete)
				})
			})

			r.Route("/presets", func(r chi.Router) {
				r.Get("/", presetHandler.List)
				r.Post("/{id}/apply", presetHandler.Apply)
			})

			r.Get("/dns/settings", dnsHandler.Get)
			r.Put("/dns/settings", dnsHandler.Update)

			r.Get("/servers/status", serverHandler.Status)
			r.Get("/servers/ru/stats", serverHandler.RUStats)
			r.Get("/servers/foreign/stats", serverHandler.ForeignStats)

			r.Get("/monitoring/traffic", monitoringHandler.Traffic)
			r.Get("/monitoring/logs", monitoringHandler.Logs)
			r.Get("/monitoring/alerts", monitoringHandler.Alerts)
			r.Get("/monitoring/stats", monitoringHandler.Stats)
		})
	})

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	logger.Info("запуск API сервера", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Error("ошибка запуска сервера", "error", err)
		os.Exit(1)
	}
}
