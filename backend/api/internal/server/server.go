package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	adminapp "github.com/sngm3741/makoto-club-services/api/internal/admin/application"
	"github.com/sngm3741/makoto-club-services/api/internal/config"
	mongodoc "github.com/sngm3741/makoto-club-services/api/internal/infrastructure/mongo"
	adminhttp "github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/admin"
	commonhttp "github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
	publichttp "github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/public"
	publicapp "github.com/sngm3741/makoto-club-services/api/internal/public/application"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Server は HTTP サーバーのライフサイクルを管理し、Public/Admin の各ハンドラへ依存注入するコンポジションルート。
// DDD の Interface 層に相当し、アプリケーションサービスをルータへ接続する責務を担う。
type Server struct {
	logger               *log.Logger
	client               *mongo.Client
	database             *mongo.Database
	pings                *mongo.Collection
	stores               *mongo.Collection
	reviews              *mongo.Collection
	surveyRepo           *mongodoc.SurveyRepository
	surveyCommandService publicapp.SurveyCommandService
	location             *time.Location
	helpfulCookieSecret  []byte
	helpfulCookieSecure  bool
	adminStoreService    adminapp.StoreService
	adminSurveyService   adminapp.SurveyService
	jwtConfigs           []config.JWTConfig
	jwtAudience          string
	httpClient           *http.Client
	messengerEndpoint    string
	messengerDestination string
	discordDestination   string
	slackDestination     string
	adminReviewBaseURL   string
	storeQueryService    publicapp.StoreQueryService
	surveyQueryService   publicapp.SurveyQueryService
	failedNotifications  *mongo.Collection
	addr                 string
	allowedOrigins       []string
}

type authenticatedUser = commonhttp.AuthenticatedUser

// Run はHTTPサーバーを起動し、Public/Adminのルーティングやミドルウェアを組み立てる。
// インフラ初期化に限定し、ドメインロジックをここに書かないことで層の責務を守る。
func (s *Server) Run() error {
	if err := s.ensureSamplePing(context.Background()); err != nil {
		s.logger.Printf("サンプル ping ドキュメントの用意に失敗しました: %v", err)
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(withCORS(s.allowedOrigins))

	router.Get("/healthz", s.healthHandler())
	router.Get("/ping", s.pingHandler())
	publicHandler := publichttp.NewHandler(publichttp.Config{
		Logger:               s.logger,
		StoreQueries:         s.storeQueryService,
		SurveyQueries:        s.surveyQueryService,
		SurveyCommands:       s.surveyCommandService,
		Stores:               s.stores,
		Reviews:              s.reviews,
		Location:             s.location,
		HelpfulCookieSecret:  s.helpfulCookieSecret,
		HelpfulCookieSecure:  s.helpfulCookieSecure,
		HTTPClient:           s.httpClient,
		MessengerEndpoint:    s.messengerEndpoint,
		MessengerDestination: s.messengerDestination,
		DiscordDestination:   s.discordDestination,
		SlackDestination:     s.slackDestination,
		FailedNotifications:  s.failedNotifications,
		AdminReviewBaseURL:   s.adminReviewBaseURL,
	})
	publicHandler.Register(router, s.authMiddleware)
	adminHandler := adminhttp.NewHandler(adminhttp.Config{
		Logger:        s.logger,
		StoreService:  s.adminStoreService,
		SurveyService: s.adminSurveyService,
	})
	router.Route("/admin", adminHandler.Register)

	httpServer := &http.Server{
		Addr:              s.addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		s.logger.Printf("HTTP サーバー起動: http://%s", s.addr)
		errChan <- httpServer.ListenAndServe()
	}()

	waitForShutdown(httpServer, errChan, s)
	return nil
}

// normaliseBaseURL は入力文字列をトリムして末尾スラッシュを削除したURLを返す。
func normaliseBaseURL(input string) string {
	trimmed := strings.TrimSpace(input)
	return strings.TrimRight(trimmed, "/")
}

// withCORS は許可されたオリジン情報をもとに CORS ヘッダーを付与するミドルウェアを返す。
func withCORS(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{})
	allowAll := false
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			allowAll = true
			continue
		}
		allowed[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" || (!allowAll && len(allowed) > 0 && !originAllowed(origin, allowed)) {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
			w.Header().Set("Access-Control-Max-Age", "300")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// originAllowed は指定された Origin が許可リストに含まれるか判定する。
func originAllowed(origin string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return true
	}
	_, ok := allowed[origin]
	return ok
}

// healthHandler は MongoDB への疎通確認を行い、監視系からのヘルスチェック要求に応える。
// ドメインの状態ではなくインフラ状態のみを返す設計。
func (s *Server) healthHandler() http.HandlerFunc {
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

		s.writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	}
}

// authMiddleware は Authorization ヘッダーから JWT を検証し、認証済みユーザーをコンテキストへ詰める。
// Public/Admin 双方のルートで利用するため Server に集約している。
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" {
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Authorization ヘッダーがありません"})
			return
		}

		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bearer トークンを指定してください"})
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
		if tokenString == "" {
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "アクセストークンが空です"})
			return
		}

		claims, err := s.parseAuthToken(tokenString)
		if err != nil {
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}

		user := authenticatedUser{
			ID:       claims.Subject,
			Name:     claims.Name,
			Username: claims.PreferredUsername,
			Picture:  claims.Picture,
		}

		ctx := commonhttp.ContextWithUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// parseAuthToken は複数の JWT 設定を順番に試し、署名検証と Issuer/Audience の整合性を確認する。
// いずれの設定にも一致しない場合は認証エラーを返す。
func (s *Server) parseAuthToken(tokenString string) (*authClaims, error) {
	if len(s.jwtConfigs) == 0 {
		return nil, fmt.Errorf("認証設定が構成されていません")
	}

	for _, cfg := range s.jwtConfigs {
		claims := &authClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
			}
			return cfg.Secret, nil
		}, jwt.WithLeeway(30*time.Second))

		if err != nil || !token.Valid {
			continue
		}

		if cfg.Issuer != "" && claims.Issuer != cfg.Issuer {
			continue
		}

		now := time.Now()
		if claims.ExpiresAt != nil && now.After(claims.ExpiresAt.Time) {
			continue
		}
		if claims.NotBefore != nil && now.Before(claims.NotBefore.Time) {
			continue
		}
		if claims.Subject == "" {
			continue
		}
		if s.jwtAudience != "" && !contains(claims.Audience, s.jwtAudience) {
			continue
		}

		return claims, nil
	}

	return nil, fmt.Errorf("アクセストークンが無効です")
}

// contains は Audience 等の検証で利用する単純な包含チェック。
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

type authClaims struct {
	jwt.RegisteredClaims
	Name              string `json:"name,omitempty"`
	Picture           string `json:"picture,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
}

type pingDocument struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Message   string             `json:"message" bson:"message"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
}

// pingHandler は `pings` コレクションから最新レコードを返す検証用エンドポイント。
// Seed されているか、アプリが Mongo にアクセスできるかを手軽に確認する用途。
func (s *Server) pingHandler() http.HandlerFunc {
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

// ensureSamplePing は pings コレクションに最低1件のドキュメントがある状態を保証する。
// ローカル環境でも /ping が 404 にならないよう起動時に呼び出す。
func (s *Server) ensureSamplePing(ctx context.Context) error {
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

// writeJSON は JSON レスポンスの共通書き込み処理。
// Content-Type 設定とエラーログ出力を一元化して重複を避ける。
func (s *Server) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		s.logger.Printf("JSON エンコードに失敗: %v", err)
	}
}

// shutdown は MongoDB クライアントをタイムアウト付きで切断し、プロセス終了時のリソースリークを防ぐ。
func (s *Server) shutdown(ctx context.Context) {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.client.Disconnect(shutdownCtx); err != nil {
		s.logger.Printf("MongoDB 切断時にエラー: %v", err)
	}
}

// waitForShutdown は ListenAndServe の終了と OS シグナルを監視し、graceful shutdown を実現する。
// アプリケーションの外側で扱うべき OS 依存の関心事をここへ閉じ込める。
func waitForShutdown(httpServer *http.Server, errChan <-chan error, srv *Server) {
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

// New は Config と Mongo クライアントを受け取り、アプリケーションサービスとハンドラを組み立てた Server を返す。
// 依存解決の起点となるファクトリとして機能する。
func New(cfg config.Config, client *mongo.Client) *Server {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.FixedZone("JST", 9*60*60)
		cfg.ServerLog.Printf("タイムゾーン %s の読み込みに失敗: %v, JST を使用します", cfg.Timezone, err)
	}

	endpoint := normaliseBaseURL(cfg.MessengerEndpoint)
	if endpoint == "" {
		endpoint = "http://messenger-gateway:3000"
	}

	srv := &Server{
		logger:               cfg.ServerLog,
		client:               client,
		database:             client.Database(cfg.MongoDatabase),
		location:             loc,
		helpfulCookieSecret:  cfg.HelpfulCookieSecret,
		helpfulCookieSecure:  cfg.HelpfulCookieSecure,
		jwtConfigs:           append([]config.JWTConfig(nil), cfg.JWTConfigs...),
		jwtAudience:          cfg.JWTAudience,
		httpClient:           &http.Client{Timeout: cfg.MessengerTimeout},
		messengerEndpoint:    endpoint,
		messengerDestination: cfg.MessengerDestination,
		discordDestination:   cfg.DiscordDestination,
		slackDestination:     strings.TrimSpace(cfg.SlackDestination),
		adminReviewBaseURL:   cfg.AdminReviewBaseURL,
		addr:                 cfg.Addr,
		allowedOrigins:       append([]string(nil), cfg.AllowedOrigins...),
	}
	srv.pings = srv.database.Collection(cfg.PingCollection)
	srv.stores = srv.database.Collection(cfg.StoreCollection)
	srv.reviews = srv.database.Collection(cfg.ReviewCollection)
	srv.failedNotifications = srv.database.Collection(cfg.FailedNotificationCollection)

	storeRepo := mongodoc.NewStoreRepository(srv.database, cfg.StoreCollection)
	srv.storeQueryService = publicapp.NewStoreQueryService(storeRepo)
	adminStoreRepo := mongodoc.NewAdminStoreRepository(srv.database, cfg.StoreCollection)
	srv.adminStoreService = adminapp.NewStoreService(adminStoreRepo)
	adminSurveyRepo := mongodoc.NewAdminSurveyRepository(srv.database, cfg.ReviewCollection, cfg.StoreCollection)
	srv.adminSurveyService = adminapp.NewSurveyService(adminSurveyRepo)

	surveyRepo := mongodoc.NewSurveyRepository(srv.database, cfg.ReviewCollection, cfg.StoreCollection, cfg.HelpfulVoteCollection)
	srv.surveyRepo = surveyRepo
	srv.surveyQueryService = publicapp.NewSurveyQueryService(surveyRepo)
	srv.surveyCommandService = publicapp.NewSurveyCommandService(surveyRepo)

	return srv
}
