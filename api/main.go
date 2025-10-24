package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type config struct {
	addr           string
	mongoURI       string
	mongoDatabase  string
	pingCollection string
	timeout        time.Duration
	timezone       string
	serverLog      *log.Logger
}

type server struct {
	logger   *log.Logger
	client   *mongo.Client
	database *mongo.Database
	pings    *mongo.Collection
	location *time.Location
}

func main() {
	cfg := loadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.mongoURI).SetServerAPIOptions(options.ServerAPI(options.ServerAPIVersion1))
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		cfg.serverLog.Fatalf("MongoDB 接続に失敗しました: %v", err)
	}

	srv := newServer(cfg, client)

	if err := srv.ensureSamplePing(context.Background()); err != nil {
		cfg.serverLog.Printf("サンプル ping ドキュメントの用意に失敗しました: %v", err)
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Get("/healthz", srv.healthHandler())
	router.Get("/api/ping", srv.pingHandler())

	httpServer := &http.Server{
		Addr:              cfg.addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		cfg.serverLog.Printf("HTTP サーバー起動: http://%s", cfg.addr)
		errChan <- httpServer.ListenAndServe()
	}()

	waitForShutdown(httpServer, errChan, srv)
}

func loadConfig() config {
	timeout := 10 * time.Second
	if v := os.Getenv("MONGO_CONNECT_TIMEOUT"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			timeout = parsed
		}
	}

	return config{
		addr:           envOrDefault("HTTP_ADDR", ":8080"),
		mongoURI:       envOrDefault("MONGO_URI", "mongodb://mongo:27017"),
		mongoDatabase:  envOrDefault("MONGO_DB", "makoto-club"),
		pingCollection: envOrDefault("PING_COLLECTION", "pings"),
		timeout:        timeout,
		timezone:       envOrDefault("TIMEZONE", "Asia/Tokyo"),
		serverLog:      log.New(os.Stdout, "[makoto-club-api] ", log.LstdFlags|log.Lshortfile),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newServer(cfg config, client *mongo.Client) *server {
	loc, err := time.LoadLocation(cfg.timezone)
	if err != nil {
		loc = time.FixedZone("JST", 9*60*60)
		cfg.serverLog.Printf("タイムゾーン %s の読み込みに失敗: %v, JST を使用します", cfg.timezone, err)
	}

	srv := &server{
		logger:   cfg.serverLog,
		client:   client,
		database: client.Database(cfg.mongoDatabase),
		location: loc,
	}
	srv.pings = srv.database.Collection(cfg.pingCollection)
	return srv
}

func (s *server) healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := s.client.Ping(ctx, readpref.Primary()); err != nil {
			s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "degraded",
				"error":  err.Error(),
			})
			return
		}

		now := time.Now().In(s.location)
		s.writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   now.Format(time.RFC3339),
		})
	}
}

type pingDocument struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Message   string             `json:"message" bson:"message"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
}

func (s *server) pingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		opts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})
		var doc pingDocument
		err := s.pings.FindOne(ctx, bson.D{}, opts).Decode(&doc)
		if errors.Is(err, mongo.ErrNoDocuments) {
			s.writeJSON(w, http.StatusNotFound, map[string]string{
				"status":  "not_found",
				"message": "ping コレクションにドキュメントが存在しません",
			})
			return
		}
		if err != nil {
			s.logger.Printf("ping コレクションのドキュメント取得に失敗: %v", err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "ping コレクションのドキュメント取得に失敗しました",
			})
			return
		}

		s.writeJSON(w, http.StatusOK, map[string]any{
			"message":   doc.Message,
			"createdAt": doc.CreatedAt.In(s.location),
			"id":        doc.ID.Hex(),
		})
	}
}

func (s *server) ensureSamplePing(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	count, err := s.pings.CountDocuments(ctx, bson.D{})
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	_, err = s.pings.InsertOne(ctx, bson.M{
		"message":   "pong",
		"createdAt": time.Now().In(s.location),
	})
	return err
}

func (s *server) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		s.logger.Printf("JSON エンコードに失敗: %v", err)
	}
}

func (s *server) shutdown(ctx context.Context) {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.client.Disconnect(shutdownCtx); err != nil {
		s.logger.Printf("MongoDB 切断時にエラー: %v", err)
	}
}

func waitForShutdown(httpServer *http.Server, errChan <-chan error, srv *server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			srv.logger.Fatalf("サーバーが異常終了: %v", err)
		}
	case sig := <-sigChan:
		srv.logger.Printf("シグナル %s を受信。サーバー停止処理を開始します。", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			srv.logger.Printf("サーバー停止時にエラー: %v", err)
		}
	}

	srv.shutdown(context.Background())
}
