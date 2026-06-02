package main

// @title        Subscriptions API
// @version      1.0
// @description  REST-сервис для агрегации данных об онлайн-подписках.
// @host         localhost:8080
// @BasePath     /api/v1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "subscriptions-service/docs"
	"subscriptions-service/internal/config"
	"subscriptions-service/internal/handlers"
	"subscriptions-service/internal/reps"
	"subscriptions-service/internal/service"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339})

	cfg, err := config.Load()
	if err != nil {
		log.WithError(err).Fatal("failed to load config")
	}

	if lvl, err := logrus.ParseLevel(cfg.Log.Level); err == nil {
		log.SetLevel(lvl)
	}

	// Подключение к БД
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DB.DSN())
	if err != nil {
		log.WithError(err).Fatal("failed to connect to database")
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.WithError(err).Fatal("database ping failed")
	}
	log.WithFields(logrus.Fields{"host": cfg.DB.Host, "db": cfg.DB.Name}).Info("connected to database")

	// Миграции
	m, err := migrate.New("file://migrations", cfg.DB.DSN())
	if err != nil {
		log.WithError(err).Fatal("failed to init migrations")
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.WithError(err).Fatal("failed to run migrations")
	}
	log.Info("migrations applied")

	// Dependency injection
	repo := reps.NewSubscriptionRepository(pool)
	svc := service.NewSubscriptionService(repo, log)
	h := handlers.NewSubscriptionHandler(svc, log)

	// Роутер
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), requestLogger(log))

	v1 := router.Group("/api/v1")
	subs := v1.Group("/subscriptions")
	{
		subs.POST("", h.Create)
		subs.GET("", h.List)
		subs.GET("/cost", h.CalculateCost) // ВАЖНО: до /:id
		subs.GET("/:id", h.GetByID)
		subs.PATCH("/:id", h.Update)
		subs.DELETE("/:id", h.Delete)
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.App.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		log.Infof("server listening on port %s", cfg.App.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Fatal("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down...")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	srv.Shutdown(shutCtx)
	log.Info("server stopped")
}

func requestLogger(log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.WithFields(logrus.Fields{
			"method":  c.Request.Method,
			"path":    c.Request.URL.Path,
			"status":  c.Writer.Status(),
			"latency": time.Since(start).String(),
			"ip":      c.ClientIP(),
		}).Info("request")
	}
}
