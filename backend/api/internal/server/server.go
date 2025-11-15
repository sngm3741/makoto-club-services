package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	adminapp "github.com/sngm3741/makoto-club-services/api/internal/admin/application"
	admindomain "github.com/sngm3741/makoto-club-services/api/internal/admin/domain"
	"github.com/sngm3741/makoto-club-services/api/internal/config"
	mongodoc "github.com/sngm3741/makoto-club-services/api/internal/infrastructure/mongo"
	adminhttp "github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/admin"
	commonhttp "github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
	publichttp "github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/public"
	publicapp "github.com/sngm3741/makoto-club-services/api/internal/public/application"
	publicdomain "github.com/sngm3741/makoto-club-services/api/internal/public/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type reviewSummaryResponse = publicdomain.SurveySummary
type reviewDetailResponse = publicdomain.SurveyDetail

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
	jwtConfigs           []config.JWTConfig
	jwtAudience          string
	httpClient           *http.Client
	messengerEndpoint    string
	messengerDestination string
	discordDestination   string
	adminReviewBaseURL   string
	mediaBaseURL         string
	storeQueryService    publicapp.StoreQueryService
	surveyQueryService   publicapp.SurveyQueryService
	addr                 string
	allowedOrigins       []string
}

var jstLocation = time.FixedZone("JST", 9*60*60)

const (
	helpfulCookieName        = "mc_helpful_voter"
	helpfulCookieTTL         = 180 * 24 * time.Hour
	helpfulCookieMaxAge      = int(helpfulCookieTTL / time.Second)
	maxStorePhotoCount       = 10
	maxSurveyPhotoCount      = 10
	maxStoreDescriptionRunes = 2000
)

var (
	allowedStoreTags         = []string{"個室", "半個室", "裏", "講習無", "店泊可", "雑費無料"}
	allowedEmploymentTypes   = []string{"出稼ぎ", "在籍"}
	allowedStoreTagSet       = makeStringSet(allowedStoreTags)
	allowedEmploymentTypeSet = makeStringSet(allowedEmploymentTypes)
)

func makeStringSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		set[item] = struct{}{}
	}
	return set
}

type authenticatedUser = commonhttp.AuthenticatedUser

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
		AdminReviewBaseURL:   s.adminReviewBaseURL,
	})
	publicHandler.Register(router, s.authMiddleware)
	adminHandler := adminhttp.NewHandler(adminhttp.Config{
		Logger:       s.logger,
		StoreService: s.adminStoreService,
		Stores:       s.stores,
		Reviews:      s.reviews,
		Location:     s.location,
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

func normaliseBaseURL(input string) string {
	trimmed := strings.TrimSpace(input)
	return strings.TrimRight(trimmed, "/")
}

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

func objectIDHex(id primitive.ObjectID) string {
	if id.IsZero() {
		return ""
	}
	return id.Hex()
}

func originAllowed(origin string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return true
	}
	_, ok := allowed[origin]
	return ok
}

func (s *Server) mediaURL(storedFilename string) string {
	filename := strings.TrimSpace(storedFilename)
	if filename == "" {
		return ""
	}
	base := strings.TrimSpace(s.mediaBaseURL)
	if base == "" {
		return filename
	}
	base = strings.TrimSuffix(base, "/")
	filename = strings.TrimPrefix(filename, "/")
	return fmt.Sprintf("%s/%s", base, filename)
}

func intPtrValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func intPtr(v int) *int {
	return &v
}

func (s *Server) findStoreIDs(ctx context.Context, prefecture, name string) ([]primitive.ObjectID, error) {
	filter := bson.M{}
	if prefecture != "" {
		filter["prefecture"] = prefecture
	}
	if name != "" {
		filter["name"] = bson.M{"$regex": name, "$options": "i"}
	}

	cursor, err := s.stores.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var ids []primitive.ObjectID
	for cursor.Next(ctx) {
		var doc struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		ids = append(ids, doc.ID)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *Server) loadStoresMap(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]mongodoc.StoreDocument, error) {
	result := make(map[primitive.ObjectID]mongodoc.StoreDocument, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	cursor, err := s.stores.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc mongodoc.StoreDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		result[doc.ID] = doc
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Server) getStoreByID(ctx context.Context, id primitive.ObjectID) (mongodoc.StoreDocument, error) {
	var store mongodoc.StoreDocument
	err := s.stores.FindOne(ctx, bson.M{"_id": id}).Decode(&store)
	return store, err
}

func (s *Server) findOrCreateStore(ctx context.Context, name, branch, prefecture, category string) (mongodoc.StoreDocument, error) {
	name = strings.TrimSpace(name)
	branch = strings.TrimSpace(branch)
	prefecture = strings.TrimSpace(prefecture)
	category = canonicalIndustryCode(category)
	if name == "" {
		return mongodoc.StoreDocument{}, errors.New("店舗名が指定されていません")
	}

	filter := bson.M{"name": name}
	if branch != "" {
		filter["branchName"] = branch
	}
	if prefecture != "" {
		filter["prefecture"] = prefecture
	}

	var store mongodoc.StoreDocument
	err := s.stores.FindOne(ctx, filter).Decode(&store)
	if err == nil {
		return store, nil
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return mongodoc.StoreDocument{}, err
	}

	now := time.Now().In(s.location)
	newID := primitive.NewObjectID()
	doc := bson.M{
		"_id":       newID,
		"name":      name,
		"createdAt": now,
		"updatedAt": now,
		"stats": bson.M{
			"reviewCount": 0,
		},
	}
	if branch != "" {
		doc["branchName"] = branch
	}
	if prefecture != "" {
		doc["prefecture"] = prefecture
	}
	if category != "" {
		doc["industries"] = bson.A{category}
	}

	if _, err := s.stores.InsertOne(ctx, doc); err != nil {
		return mongodoc.StoreDocument{}, err
	}

	return s.getStoreByID(ctx, newID)
}

func (s *Server) recalculateStoreStats(ctx context.Context, storeID primitive.ObjectID) error {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"storeId": storeID,
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":            nil,
			"reviewCount":    bson.M{"$sum": 1},
			"avgRating":      bson.M{"$avg": "$rating"},
			"avgEarning":     bson.M{"$avg": "$averageEarning"},
			"avgWaitTime":    bson.M{"$avg": "$waitTimeHours"},
			"lastReviewedAt": bson.M{"$max": "$createdAt"},
		}}},
	}

	cursor, err := s.reviews.Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	update := bson.M{
		"stats.reviewCount":    0,
		"stats.avgRating":      nil,
		"stats.avgEarning":     nil,
		"stats.avgWaitTime":    nil,
		"stats.lastReviewedAt": nil,
		"updatedAt":            time.Now().In(s.location),
	}

	if cursor.Next(ctx) {
		var agg struct {
			ReviewCount    int        `bson:"reviewCount"`
			AvgRating      *float64   `bson:"avgRating"`
			AvgEarning     *float64   `bson:"avgEarning"`
			AvgWaitTime    *float64   `bson:"avgWaitTime"`
			LastReviewedAt *time.Time `bson:"lastReviewedAt"`
		}
		if err := cursor.Decode(&agg); err != nil {
			return err
		}
		update["stats.reviewCount"] = agg.ReviewCount
		update["stats.avgRating"] = agg.AvgRating
		update["stats.avgEarning"] = agg.AvgEarning
		update["stats.avgWaitTime"] = agg.AvgWaitTime
		update["stats.lastReviewedAt"] = agg.LastReviewedAt
	}
	if err := cursor.Err(); err != nil {
		return err
	}

	_, err = s.stores.UpdateByID(ctx, storeID, bson.M{"$set": update})
	return err
}

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
		adminReviewBaseURL:   cfg.AdminReviewBaseURL,
		mediaBaseURL:         strings.TrimSuffix(strings.TrimSpace(cfg.MediaBaseURL), "/"),
		addr:                 cfg.Addr,
		allowedOrigins:       append([]string(nil), cfg.AllowedOrigins...),
	}
	srv.pings = srv.database.Collection(cfg.PingCollection)
	srv.stores = srv.database.Collection(cfg.StoreCollection)
	srv.reviews = srv.database.Collection(cfg.ReviewCollection)

	storeRepo := mongodoc.NewStoreRepository(srv.database, cfg.StoreCollection)
	srv.storeQueryService = publicapp.NewStoreQueryService(storeRepo)
	adminStoreRepo := mongodoc.NewAdminStoreRepository(srv.database, cfg.StoreCollection)
	srv.adminStoreService = adminapp.NewStoreService(adminStoreRepo)

	surveyRepo := mongodoc.NewSurveyRepository(srv.database, cfg.ReviewCollection, cfg.StoreCollection, cfg.HelpfulVoteCollection)
	srv.surveyRepo = surveyRepo
	srv.surveyQueryService = publicapp.NewSurveyQueryService(surveyRepo)
	srv.surveyCommandService = publicapp.NewSurveyCommandService(surveyRepo)

	return srv
}

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

func authenticatedUserFromContext(ctx context.Context) (authenticatedUser, bool) {
	return commonhttp.UserFromContext(ctx)
}

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

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func canonicalIndustryCode(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	lower := strings.ToLower(trimmed)
	switch lower {
	case "deriheru", "delivery_health":
		return "デリヘル"
	case "hoteheru", "hotel_health":
		return "ホテヘル"
	case "hakoheru", "hako_heru", "hako-health":
		return "箱ヘル"
	case "sopu", "soap":
		return "ソープ"
	case "dc":
		return "DC"
	case "huesu", "fuesu":
		return "風エス"
	case "menesu", "mensu", "mens_es":
		return "メンエス"
	}

	switch trimmed {
	case "デリヘル", "ホテヘル", "箱ヘル", "ソープ", "DC", "風エス", "メンエス":
		return trimmed
	}

	return trimmed
}

func canonicalIndustryCodes(codes []string) []string {
	result := make([]string, 0, len(codes))
	seen := make(map[string]struct{})
	for _, code := range codes {
		canonical := canonicalIndustryCode(code)
		if canonical == "" {
			continue
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	return result
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

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		s.logger.Printf("JSON エンコードに失敗: %v", err)
	}
}

func (s *Server) shutdown(ctx context.Context) {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.client.Disconnect(shutdownCtx); err != nil {
		s.logger.Printf("MongoDB 切断時にエラー: %v", err)
	}
}

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

type reviewListResponse struct {
	Items []reviewSummaryResponse `json:"items"`
	Page  int                     `json:"page"`
	Limit int                     `json:"limit"`
	Total int                     `json:"total"`
}

type adminReviewResponse struct {
	ID              string                     `json:"id"`
	StoreID         string                     `json:"storeId"`
	StoreName       string                     `json:"storeName"`
	BranchName      string                     `json:"branchName,omitempty"`
	Prefecture      string                     `json:"prefecture"`
	Area            string                     `json:"area,omitempty"`
	Category        string                     `json:"category"`
	Industries      []string                   `json:"industries,omitempty"`
	Genre           string                     `json:"genre,omitempty"`
	VisitedAt       string                     `json:"visitedAt"`
	Age             int                        `json:"age"`
	SpecScore       int                        `json:"specScore"`
	WaitTimeMinutes int                        `json:"waitTimeMinutes"`
	AverageEarning  int                        `json:"averageEarning"`
	EmploymentType  string                     `json:"employmentType,omitempty"`
	Rating          float64                    `json:"rating"`
	Comment         string                     `json:"comment,omitempty"`
	CustomerNote    string                     `json:"customerNote,omitempty"`
	StaffNote       string                     `json:"staffNote,omitempty"`
	EnvironmentNote string                     `json:"environmentNote,omitempty"`
	Tags            []string                   `json:"tags,omitempty"`
	ContactEmail    string                     `json:"contactEmail,omitempty"`
	Photos          []adminSurveyPhotoResponse `json:"photos,omitempty"`
	HelpfulCount    int                        `json:"helpfulCount"`
	CreatedAt       time.Time                  `json:"createdAt"`
	UpdatedAt       time.Time                  `json:"updatedAt"`
}

type adminReviewListResponse struct {
	Items []adminReviewResponse `json:"items"`
}

type adminStoreResponse struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	BranchName      string          `json:"branchName,omitempty"`
	GroupName       string          `json:"groupName,omitempty"`
	Prefecture      string          `json:"prefecture,omitempty"`
	Area            string          `json:"area,omitempty"`
	Genre           string          `json:"genre,omitempty"`
	Industries      []string        `json:"industries,omitempty"`
	EmploymentTypes []string        `json:"employmentTypes,omitempty"`
	BusinessHours   string          `json:"businessHours,omitempty"`
	PricePerHour    int             `json:"pricePerHour,omitempty"`
	PriceRange      string          `json:"priceRange,omitempty"`
	AverageEarning  int             `json:"averageEarning,omitempty"`
	Tags            []string        `json:"tags,omitempty"`
	HomepageURL     string          `json:"homepageUrl,omitempty"`
	SNS             storeSNSPayload `json:"sns"`
	PhotoURLs       []string        `json:"photoUrls,omitempty"`
	Description     string          `json:"description,omitempty"`
	ReviewCount     int             `json:"reviewCount"`
	LastReviewedAt  *time.Time      `json:"lastReviewedAt,omitempty"`
}

type storeSNSPayload struct {
	Twitter   string `json:"twitter,omitempty"`
	Line      string `json:"line,omitempty"`
	Instagram string `json:"instagram,omitempty"`
	TikTok    string `json:"tiktok,omitempty"`
	Official  string `json:"official,omitempty"`
}

type adminStoreSNSPayload = storeSNSPayload

type adminSurveyPhotoResponse struct {
	ID          string    `json:"id"`
	StoredPath  string    `json:"storedPath,omitempty"`
	PublicURL   string    `json:"publicUrl,omitempty"`
	ContentType string    `json:"contentType,omitempty"`
	UploadedAt  time.Time `json:"uploadedAt"`
}

func adminStoreDomainToResponse(store admindomain.Store) adminStoreResponse {
	return adminStoreResponse{
		ID:              store.ID,
		Name:            store.Name,
		BranchName:      strings.TrimSpace(store.BranchName),
		GroupName:       store.GroupName,
		Prefecture:      store.Prefecture,
		Area:            store.Area,
		Genre:           store.Genre,
		Industries:      canonicalIndustryCodes(store.Industries),
		EmploymentTypes: append([]string{}, store.EmploymentTypes...),
		BusinessHours:   store.BusinessHours,
		PricePerHour:    store.PricePerHour,
		PriceRange:      store.PriceRange,
		AverageEarning:  store.AverageEarning,
		Tags:            append([]string{}, store.Tags...),
		HomepageURL:     store.HomepageURL,
		SNS: adminStoreSNSPayload{
			Twitter:   store.SNS.Twitter,
			Line:      store.SNS.Line,
			Instagram: store.SNS.Instagram,
			TikTok:    store.SNS.TikTok,
			Official:  store.SNS.Official,
		},
		PhotoURLs:      append([]string{}, store.PhotoURLs...),
		Description:    store.Description,
		ReviewCount:    store.ReviewCount,
		LastReviewedAt: store.LastReviewedAt,
	}
}

type createReviewRequest struct {
	StoreName       string               `json:"storeName"`
	BranchName      string               `json:"branchName"`
	Prefecture      string               `json:"prefecture"`
	Industries      []string             `json:"industries"`
	VisitedAt       string               `json:"visitedAt"`
	Age             int                  `json:"age"`
	SpecScore       int                  `json:"specScore"`
	WaitTimeHours   int                  `json:"waitTimeHours"`
	AverageEarning  int                  `json:"averageEarning"`
	Comment         string               `json:"comment"`
	Rating          float64              `json:"rating"`
	ContactEmail    string               `json:"contactEmail,omitempty"`
	CustomerNote    string               `json:"customerNote,omitempty"`
	StaffNote       string               `json:"staffNote,omitempty"`
	EnvironmentNote string               `json:"environmentNote,omitempty"`
	Tags            []string             `json:"tags"`
	Photos          []reviewPhotoPayload `json:"photos"`
}

type createReviewResponse struct {
	Status string                `json:"status"`
	Review reviewSummaryResponse `json:"review"`
	Detail reviewDetailResponse  `json:"detail"`
}

const maxReviewRequestBody = 1 << 20

type reviewMetrics struct {
	VisitedAt      string
	Age            int
	SpecScore      int
	WaitTimeHours  int
	AverageEarning int
	Comment        string
	Rating         float64
	ContactEmail   string
}

type reviewPhotoPayload struct {
	ID          string `json:"id"`
	StoredPath  string `json:"storedPath"`
	PublicURL   string `json:"publicUrl"`
	ContentType string `json:"contentType"`
}

func (m *reviewMetrics) normalize() error {
	m.VisitedAt = strings.TrimSpace(m.VisitedAt)
	if m.VisitedAt == "" {
		return errors.New("働いた時期を指定してください")
	}
	if m.Age < 18 {
		return errors.New("年齢は18歳以上で入力してください")
	}
	if m.Age > 60 {
		m.Age = 60
	}
	if m.SpecScore < 60 {
		return errors.New("スペックは60以上で入力してください")
	}
	if m.SpecScore > 140 {
		m.SpecScore = 140
	}
	if m.WaitTimeHours < 1 {
		return errors.New("待機時間は1時間以上で入力してください")
	}
	if m.WaitTimeHours > 24 {
		m.WaitTimeHours = 24
	}
	if m.AverageEarning < 0 {
		return errors.New("平均稼ぎは0以上で入力してください")
	}
	if m.AverageEarning > 20 {
		m.AverageEarning = 20
	}
	if m.Rating < 0 || m.Rating > 5 {
		return errors.New("総評は0〜5の範囲で入力してください")
	}
	m.Rating = math.Round(m.Rating*2) / 2
	comment := strings.TrimSpace(m.Comment)
	if len([]rune(comment)) > 2000 {
		return errors.New("感想は2000文字以内で入力してください")
	}
	m.Comment = comment

	email, err := normalizeEmail(m.ContactEmail)
	if err != nil {
		return err
	}
	m.ContactEmail = email
	return nil
}

func (req *createReviewRequest) validate() error {
	if strings.TrimSpace(req.StoreName) == "" {
		return errors.New("店舗名は必須です")
	}
	if strings.TrimSpace(req.Prefecture) == "" {
		return errors.New("都道府県は必須です")
	}
	if len(req.Industries) == 0 {
		return errors.New("業種は1件以上指定してください")
	}
	if len(req.Photos) > maxSurveyPhotoCount {
		return fmt.Errorf("写真は最大%d枚までです", maxSurveyPhotoCount)
	}
	metrics := reviewMetrics{
		VisitedAt:      req.VisitedAt,
		Age:            req.Age,
		SpecScore:      req.SpecScore,
		WaitTimeHours:  req.WaitTimeHours,
		AverageEarning: req.AverageEarning,
		Comment:        req.Comment,
		Rating:         req.Rating,
		ContactEmail:   req.ContactEmail,
	}
	if err := metrics.normalize(); err != nil {
		return err
	}
	req.VisitedAt = metrics.VisitedAt
	req.Age = metrics.Age
	req.SpecScore = metrics.SpecScore
	req.WaitTimeHours = metrics.WaitTimeHours
	req.AverageEarning = metrics.AverageEarning
	req.Comment = metrics.Comment
	req.Rating = metrics.Rating
	req.ContactEmail = metrics.ContactEmail
	req.BranchName = strings.TrimSpace(req.BranchName)
	return nil
}

func formatSurveyPeriod(visited string) (string, error) {
	value := strings.TrimSpace(visited)
	if value == "" {
		return "", errors.New("働いた時期を指定してください")
	}

	t, err := time.Parse("2006-01", value)
	if err != nil {
		return "", fmt.Errorf("働いた時期の形式が不正です: %w", err)
	}

	return fmt.Sprintf("%d年%d月", t.Year(), int(t.Month())), nil
}

func formatWaitTimeLabel(hours int) string {
	return fmt.Sprintf("%d時間", hours)
}

func formatAverageEarningLabel(value int) string {
	if value >= 20 {
		return "20万円以上"
	}
	return fmt.Sprintf("%d万円", value)
}

func formatVisitedDisplay(visited string) string {
	t, err := time.Parse("2006-01", visited)
	if err != nil {
		return visited
	}
	return fmt.Sprintf("%d年%d月", t.Year(), int(t.Month()))
}

type storeSummaryResponse struct {
	ID                  string   `json:"id"`
	StoreName           string   `json:"storeName"`
	BranchName          string   `json:"branchName,omitempty"`
	Prefecture          string   `json:"prefecture"`
	Industries          []string `json:"industries,omitempty"`
	AverageRating       float64  `json:"averageRating"`
	AverageEarning      int      `json:"averageEarning"`
	AverageEarningLabel string   `json:"averageEarningLabel,omitempty"`
	WaitTimeHours       int      `json:"waitTimeHours"`
	WaitTimeLabel       string   `json:"waitTimeLabel,omitempty"`
	ReviewCount         int      `json:"reviewCount"`
	Tags                []string `json:"tags,omitempty"`
	PhotoURLs           []string `json:"photoUrls,omitempty"`
}

type storeDetailResponse struct {
	ID                  string          `json:"id"`
	StoreName           string          `json:"storeName"`
	BranchName          string          `json:"branchName,omitempty"`
	Prefecture          string          `json:"prefecture,omitempty"`
	Area                string          `json:"area,omitempty"`
	Genre               string          `json:"genre,omitempty"`
	BusinessHours       string          `json:"businessHours,omitempty"`
	PriceRange          string          `json:"priceRange,omitempty"`
	Industries          []string        `json:"industries,omitempty"`
	EmploymentTypes     []string        `json:"employmentTypes,omitempty"`
	PricePerHour        int             `json:"pricePerHour,omitempty"`
	AverageRating       float64         `json:"averageRating"`
	AverageEarning      int             `json:"averageEarning"`
	AverageEarningLabel string          `json:"averageEarningLabel,omitempty"`
	WaitTimeHours       int             `json:"waitTimeHours"`
	WaitTimeLabel       string          `json:"waitTimeLabel,omitempty"`
	ReviewCount         int             `json:"reviewCount"`
	LastReviewedAt      *time.Time      `json:"lastReviewedAt,omitempty"`
	UpdatedAt           *time.Time      `json:"updatedAt,omitempty"`
	Tags                []string        `json:"tags,omitempty"`
	PhotoURLs           []string        `json:"photoUrls,omitempty"`
	HomepageURL         string          `json:"homepageUrl,omitempty"`
	SNS                 storeSNSPayload `json:"sns"`
	Description         string          `json:"description,omitempty"`
}

type storeListResponse struct {
	Items []storeSummaryResponse `json:"items"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
	Total int                    `json:"total"`
}

func buildStoreSummaryResponse(store publicdomain.Store, categoryOverride string) storeSummaryResponse {
	avgEarning := 0
	avgEarningLabel := "-"
	if store.Stats.AvgEarning != nil {
		avgEarning = int(math.Round(*store.Stats.AvgEarning))
		avgEarningLabel = formatAverageEarningLabel(avgEarning)
	}

	waitHours := 0
	waitLabel := "-"
	if store.Stats.AvgWaitTime != nil {
		waitHours = int(math.Round(*store.Stats.AvgWaitTime))
		waitLabel = formatWaitTimeLabel(waitHours)
	}

	avgRating := 0.0
	if store.Stats.AvgRating != nil {
		avgRating = math.Round(*store.Stats.AvgRating*10) / 10
	}

	return storeSummaryResponse{
		ID:                  store.ID,
		StoreName:           store.Name,
		BranchName:          strings.TrimSpace(store.BranchName),
		Prefecture:          store.Prefecture,
		Industries:          canonicalIndustryCodes(store.Industries),
		AverageRating:       avgRating,
		AverageEarning:      avgEarning,
		AverageEarningLabel: avgEarningLabel,
		WaitTimeHours:       waitHours,
		WaitTimeLabel:       waitLabel,
		ReviewCount:         store.Stats.ReviewCount,
		Tags:                append([]string{}, store.Tags...),
		PhotoURLs:           append([]string{}, store.PhotoURLs...),
	}
}

func storeDomainToDetailResponse(store publicdomain.Store) storeDetailResponse {
	industries := canonicalIndustryCodes(store.Industries)

	avgRating := 0.0
	if store.Stats.AvgRating != nil {
		avgRating = math.Round(*store.Stats.AvgRating*10) / 10
	}

	avgEarning := 0
	avgEarningLabel := "-"
	if store.Stats.AvgEarning != nil {
		avgEarning = int(math.Round(*store.Stats.AvgEarning))
		avgEarningLabel = formatAverageEarningLabel(avgEarning)
	}

	waitHours := 0
	waitLabel := "-"
	if store.Stats.AvgWaitTime != nil {
		waitHours = int(math.Round(*store.Stats.AvgWaitTime))
		waitLabel = formatWaitTimeLabel(waitHours)
	}

	var updatedAt *time.Time
	if !store.UpdatedAt.IsZero() {
		t := store.UpdatedAt
		updatedAt = &t
	}

	return storeDetailResponse{
		ID:                  store.ID,
		StoreName:           store.Name,
		BranchName:          strings.TrimSpace(store.BranchName),
		Prefecture:          store.Prefecture,
		Area:                store.Area,
		Genre:               store.Genre,
		BusinessHours:       store.BusinessHours,
		PriceRange:          store.PriceRange,
		Industries:          industries,
		EmploymentTypes:     append([]string{}, store.EmploymentTypes...),
		PricePerHour:        store.PricePerHour,
		AverageRating:       avgRating,
		AverageEarning:      avgEarning,
		AverageEarningLabel: avgEarningLabel,
		WaitTimeHours:       waitHours,
		WaitTimeLabel:       waitLabel,
		ReviewCount:         store.Stats.ReviewCount,
		LastReviewedAt:      store.Stats.LastReviewedAt,
		UpdatedAt:           updatedAt,
		Tags:                append([]string{}, store.Tags...),
		PhotoURLs:           append([]string{}, store.PhotoURLs...),
		HomepageURL:         store.HomepageURL,
		SNS: storeSNSPayload{
			Twitter:   store.SNS.Twitter,
			Line:      store.SNS.Line,
			Instagram: store.SNS.Instagram,
			TikTok:    store.SNS.TikTok,
			Official:  store.SNS.Official,
		},
		Description: store.Description,
	}
}

func parseInt(value string) (int, bool) {
	if strings.TrimSpace(value) == "" {
		return 0, false
	}
	num, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return num, true
}

func parsePositiveInt(value string, fallback int) (int, bool) {
	num, ok := parseInt(value)
	if !ok || num <= 0 {
		return fallback, false
	}
	return num, true
}

func (s *Server) ensureHelpfulVoterID(w http.ResponseWriter, r *http.Request) (string, error) {
	if len(s.helpfulCookieSecret) == 0 {
		return "", errors.New("helpful voter secret not configured")
	}
	if cookie, err := r.Cookie(helpfulCookieName); err == nil {
		if voterID, issuedAt, ok := s.parseHelpfulCookie(cookie.Value); ok && time.Since(issuedAt) < helpfulCookieTTL {
			return voterID, nil
		}
	}
	voterID := primitive.NewObjectID().Hex()
	s.issueHelpfulCookie(w, voterID)
	return voterID, nil
}

func (s *Server) issueHelpfulCookie(w http.ResponseWriter, voterID string) {
	value := s.signHelpfulCookie(voterID, time.Now().UTC())
	http.SetCookie(w, &http.Cookie{
		Name:     helpfulCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.helpfulCookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   helpfulCookieMaxAge,
	})
}

func (s *Server) signHelpfulCookie(voterID string, issuedAt time.Time) string {
	payload := fmt.Sprintf("v=%s&ts=%d", voterID, issuedAt.Unix())
	mac := hmac.New(sha256.New, s.helpfulCookieSecret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "&sig=" + sig
}

func (s *Server) parseHelpfulCookie(raw string) (string, time.Time, bool) {
	parts := strings.Split(raw, "&")
	if len(parts) < 3 {
		return "", time.Time{}, false
	}
	values := make(map[string]string, len(parts))
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		values[kv[0]] = kv[1]
	}
	voterID := values["v"]
	tsValue := values["ts"]
	sigValue := values["sig"]
	if voterID == "" || tsValue == "" || sigValue == "" {
		return "", time.Time{}, false
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(sigValue)
	if err != nil {
		return "", time.Time{}, false
	}
	payload := fmt.Sprintf("v=%s&ts=%s", voterID, tsValue)
	mac := hmac.New(sha256.New, s.helpfulCookieSecret)
	mac.Write([]byte(payload))
	if !hmac.Equal(sigBytes, mac.Sum(nil)) {
		return "", time.Time{}, false
	}
	tsInt, err := strconv.ParseInt(tsValue, 10, 64)
	if err != nil {
		return "", time.Time{}, false
	}
	return voterID, time.Unix(tsInt, 0).UTC(), true
}

var numberPattern = regexp.MustCompile(`\d+(?:\.\d+)?`)

func parseFirstNumber(input string) (float64, bool) {
	match := numberPattern.FindString(input)
	if match == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func normalizeEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > 254 {
		return "", errors.New("メールアドレスは254文字以内で入力してください")
	}
	if _, err := mail.ParseAddress(trimmed); err != nil {
		return "", errors.New("メールアドレスの形式が正しくありません")
	}
	return trimmed, nil
}

func (s *Server) notifyReviewReceipt(ctx context.Context, user authenticatedUser, summary reviewSummaryResponse, comment string) {
	if ctx == nil {
		ctx = context.Background()
	}

	if userID := strings.TrimSpace(user.ID); userID != "" {
		message := buildReceiptMessage(summary, comment)
		if err := s.sendLineMessage(ctx, userID, message); err != nil && s.logger != nil {
			s.logger.Printf("LINE通知の送信に失敗: %v", err)
		}
	}

	if strings.TrimSpace(s.discordDestination) != "" {
		discordMessage := buildDiscordReviewMessage(s.adminReviewBaseURL, user, summary, comment)
		if discordMessage != "" {
			identifier := summary.ID
			if identifier == "" {
				identifier = strings.TrimSpace(user.Username)
			}
			if identifier == "" {
				identifier = user.ID
			}
			if identifier == "" {
				identifier = "discord"
			}
			if err := s.sendDiscordMessage(ctx, identifier, discordMessage); err != nil && s.logger != nil {
				s.logger.Printf("Discord通知の送信に失敗: %v", err)
			}
		}
	}
}

func reviewerDisplayName(user authenticatedUser) string {
	name := strings.TrimSpace(user.Name)
	if name != "" {
		return name
	}
	return "匿名店舗アンケート"
}

func buildReceiptMessage(summary reviewSummaryResponse, comment string) string {
	sections := [][]string{}

	addSection := func(title, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		sections = append(sections, []string{
			fmt.Sprintf("**%s**", title),
			"> " + value,
		})
	}

	addSection("店舗名", summary.StoreName)
	addSection("支店名", summary.BranchName)
	addSection("都道府県", summary.Prefecture)
	addSection("訪問時期", formatVisitedDisplay(summary.VisitedAt))
	if len(summary.Industries) > 0 {
		addSection("業種", strings.Join(summary.Industries, " / "))
	}
	if len(summary.Industries) > 0 {
		addSection("業種", strings.Join(summary.Industries, " / "))
	}
	if len(summary.Industries) > 0 {
		addSection("業種", strings.Join(summary.Industries, " / "))
	}
	if summary.AverageEarning > 0 {
		addSection("平均稼ぎ", fmt.Sprintf("%d万円", summary.AverageEarning))
	}
	if summary.WaitTimeHours > 0 {
		addSection("待機時間", fmt.Sprintf("%d時間", summary.WaitTimeHours))
	}
	if summary.Age > 0 {
		addSection("年齢", fmt.Sprintf("%d歳", summary.Age))
	}
	if summary.SpecScore > 0 {
		addSection("スペック", fmt.Sprintf("%d", summary.SpecScore))
	}
	if len(summary.Tags) > 0 {
		addSection("タグ", strings.Join(summary.Tags, ", "))
	}
	if len(summary.Tags) > 0 {
		addSection("タグ", strings.Join(summary.Tags, ", "))
	}
	if trimmedComment := strings.TrimSpace(comment); trimmedComment != "" {
		addSection("客層・スタッフ・環境等", trimmedComment)
	}
	if summary.Rating > 0 {
		addSection("満足度", formatRatingValue(summary.Rating))
	}

	lines := []string{
		"アンケートを受け取りました。ご協力ありがとうございます！",
		"",
	}
	for _, section := range sections {
		lines = append(lines, section...)
		lines = append(lines, "")
	}
	lines = append(lines, "内容の確認が終わり次第PayPay1000円分のリンクをお送りします。")

	return strings.Join(lines, "\n")
}

func formatDiscordTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006-01",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.In(jstLocation).Format("2006-01-02 15:04")
		}
	}

	return value
}

func formatRatingValue(value float64) string {
	if value <= 0 {
		return "0"
	}
	formatted := strconv.FormatFloat(value, 'f', 1, 64)
	formatted = strings.TrimSuffix(formatted, "0")
	formatted = strings.TrimSuffix(formatted, ".")
	return formatted
}

func buildDiscordReviewMessage(adminBaseURL string, user authenticatedUser, summary reviewSummaryResponse, comment string) string {
	sections := [][]string{}

	addSection := func(title, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		sections = append(sections, []string{
			fmt.Sprintf("**%s**", title),
			"> " + value,
		})
	}

	addSection("店舗名", summary.StoreName)
	addSection("支店名", summary.BranchName)
	addSection("都道府県", summary.Prefecture)
	addSection("訪問時期", formatVisitedDisplay(summary.VisitedAt))
	if summary.AverageEarning > 0 {
		addSection("平均稼ぎ", fmt.Sprintf("%d万円", summary.AverageEarning))
	}
	if summary.WaitTimeHours > 0 {
		addSection("待機時間", fmt.Sprintf("%d時間", summary.WaitTimeHours))
	}
	if summary.Age > 0 {
		addSection("年齢", fmt.Sprintf("%d歳", summary.Age))
	}
	if summary.SpecScore > 0 {
		addSection("スペック", fmt.Sprintf("%d", summary.SpecScore))
	}
	if trimmed := strings.TrimSpace(comment); trimmed != "" {
		addSection("客層・スタッフ・環境等", trimmed)
	}
	if summary.Rating > 0 {
		addSection("満足度", formatRatingValue(summary.Rating))
	}

	lines := []string{
		"📝 **アンケートが投稿されました**",
	}

	if postedAt := formatDiscordTimestamp(summary.CreatedAt); postedAt != "" {
		lines = append(lines, fmt.Sprintf("🕐 投稿日時: %s", postedAt))
	}

	if username := strings.TrimSpace(user.Username); username != "" {
		escaped := url.PathEscape(username)
		lines = append(lines, fmt.Sprintf("👤 投稿者: [@%s](https://twitter.com/%s)", username, escaped))
	} else {
		lines = append(lines, "👤投稿者: (未設定)")
	}

	lines = append(lines, "", "**【内容】**")
	for _, section := range sections {
		lines = append(lines, section...)
		lines = append(lines, "")
	}

	if trimmed := strings.TrimSpace(adminBaseURL); trimmed != "" {
		link := strings.TrimSuffix(trimmed, "/")
		if summary.ID != "" {
			link = link + "/" + summary.ID
		}
		lines = append(lines, fmt.Sprintf("🔗 [管理画面](%s)", link))
	}

	lines = append(lines, "", "内容を確認のうえ、PayPay 送付対応を進めてください。")

	return strings.Join(lines, "\n")
}

func (s *Server) sendMessengerMessage(ctx context.Context, destination, userID, text string) error {
	if s.httpClient == nil || s.messengerEndpoint == "" {
		return errors.New("メッセンジャー送信の設定がされていません")
	}

	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return errors.New("メッセンジャー送信先ユーザーIDが空です")
	}

	bodyText := strings.TrimSpace(text)
	if bodyText == "" {
		return errors.New("メッセンジャー送信本文が空です")
	}

	payload := map[string]any{
		"userId": trimmedUserID,
		"text":   bodyText,
	}
	if dest := strings.TrimSpace(destination); dest != "" {
		payload["destination"] = dest
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("メッセンジャー送信用ペイロードの作成に失敗: %w", err)
	}

	timeout := s.httpClient.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpoint := s.messengerEndpoint + "/messages"
	req, err := http.NewRequestWithContext(ctxWithTimeout, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("メッセンジャー送信リクエストの作成に失敗: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("メッセンジャー送信リクエストに失敗: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		message, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
		return fmt.Errorf("メッセンジャー送信でエラーが発生: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(message)))
	}

	return nil
}

func (s *Server) sendLineMessage(ctx context.Context, userID, text string) error {
	return s.sendMessengerMessage(ctx, s.messengerDestination, userID, text)
}

func (s *Server) sendDiscordMessage(ctx context.Context, userID, text string) error {
	dest := strings.TrimSpace(s.discordDestination)
	if dest == "" {
		return nil
	}
	return s.sendMessengerMessage(ctx, dest, userID, text)
}

func (s *Server) reviewCreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUserFromContext(r.Context())
		if !ok {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "認証情報を取得できませんでした"})
			return
		}

		defer r.Body.Close()

		var req createReviewRequest
		decoder := json.NewDecoder(io.LimitReader(r.Body, maxReviewRequestBody))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("リクエストの形式が不正です: %v", err),
			})
			return
		}

		if err := req.validate(); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		period, err := formatSurveyPeriod(req.VisitedAt)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		industries, err := normalizeIndustryList(req.Industries)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		category := ""
		if len(industries) > 0 {
			category = industries[0]
		}
		comment := strings.TrimSpace(req.Comment)

		storeName := strings.TrimSpace(req.StoreName)
		branchName := strings.TrimSpace(req.BranchName)
		prefecture := strings.TrimSpace(req.Prefecture)

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		store, err := s.findOrCreateStore(ctx, storeName, branchName, prefecture, category)
		if err != nil {
			s.logger.Printf("店舗の取得/作成に失敗: %v", err)
			http.Error(w, "店舗情報の処理に失敗しました", http.StatusInternalServerError)
			return
		}

		tags, err := normalizeStoreTags(req.Tags)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		photos, err := normalizeReviewPhotos(req.Photos, maxSurveyPhotoCount)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		cmd := publicapp.SubmitSurveyCommand{
			StoreID:         store.ID.Hex(),
			StoreName:       store.Name,
			BranchName:      strings.TrimSpace(store.BranchName),
			Prefecture:      store.Prefecture,
			Area:            store.Area,
			Industries:      industries,
			Period:          period,
			Age:             intPtr(req.Age),
			SpecScore:       intPtr(req.SpecScore),
			WaitTime:        intPtr(req.WaitTimeHours),
			AverageEarning:  intPtr(req.AverageEarning),
			EmploymentType:  "",
			CustomerNote:    strings.TrimSpace(req.CustomerNote),
			StaffNote:       strings.TrimSpace(req.StaffNote),
			EnvironmentNote: strings.TrimSpace(req.EnvironmentNote),
			Comment:         comment,
			Rating:          req.Rating,
			ContactEmail:    req.ContactEmail,
			Tags:            tags,
			Photos:          photos,
		}

		createdSurvey, err := s.surveyCommandService.Submit(ctx, cmd)
		if err != nil {
			s.logger.Printf("レビューの保存に失敗: %v", err)
			http.Error(w, "レビューの保存に失敗しました", http.StatusInternalServerError)
			return
		}

		if category != "" {
			_, err := s.stores.UpdateByID(ctx, store.ID, bson.M{"$addToSet": bson.M{"industries": category}})
			if err != nil {
				s.logger.Printf("店舗業種の更新に失敗: %v", err)
			}
		}

		if err := s.recalculateStoreStats(ctx, store.ID); err != nil {
			s.logger.Printf("店舗統計の更新に失敗: %v", err)
		}
		if refreshed, err := s.getStoreByID(ctx, store.ID); err == nil {
			store = refreshed
		}

		createdSurvey.StoreName = store.Name
		createdSurvey.BranchName = strings.TrimSpace(store.BranchName)
		createdSurvey.Prefecture = store.Prefecture
		createdSurvey.Area = store.Area

		summary := buildReviewSummaryFromDomain(*createdSurvey)
		detail := buildReviewDetailFromDomain(*createdSurvey, reviewerDisplayName(user), user.Picture)

		go s.notifyReviewReceipt(context.Background(), user, summary, comment)

		s.writeJSON(w, http.StatusCreated, createReviewResponse{
			Status: "ok",
			Review: summary,
			Detail: detail,
		})
	}
}

type reviewQueryParams struct {
	Prefecture string
	Category   string
	StoreName  string
	StoreID    primitive.ObjectID
	Sort       string
	Page       int
	Limit      int
}

func (s *Server) collectReviews(ctx context.Context, params reviewQueryParams) ([]reviewSummaryResponse, error) {
	filter := publicapp.SurveyFilter{
		Prefecture: params.Prefecture,
		Genre:      params.Category,
		StoreName:  params.StoreName,
	}
	if params.StoreID != primitive.NilObjectID {
		filter.StoreID = params.StoreID.Hex()
	}

	paging := publicapp.Paging{
		Page:  params.Page,
		Limit: params.Limit,
		Sort:  params.Sort,
	}

	surveys, err := s.surveyQueryService.List(ctx, filter, paging)
	if err != nil {
		return nil, err
	}

	summaries := make([]reviewSummaryResponse, 0, len(surveys))
	for _, survey := range surveys {
		summaries = append(summaries, buildReviewSummaryFromDomain(survey))
	}
	return summaries, nil
}

func buildReviewSummaryFromDomain(survey publicdomain.Survey) reviewSummaryResponse {
	industries := canonicalIndustryCodes(survey.Industries)

	visitedAt, createdAt := deriveDates(survey.Period)
	if !survey.CreatedAt.IsZero() {
		createdAt = survey.CreatedAt.Format(time.RFC3339)
	}

	spec := intPtrValue(survey.SpecScore)
	wait := intPtrValue(survey.WaitTime)
	earning := intPtrValue(survey.AverageEarning)
	helpful := survey.HelpfulCount
	if helpful == 0 {
		helpful = deriveHelpfulCount(survey.CreatedAt, spec)
	}

	waitLabel := ""
	if wait > 0 {
		waitLabel = formatWaitTimeLabel(wait)
	}
	earningLabel := ""
	if earning > 0 {
		earningLabel = formatAverageEarningLabel(earning)
	}

	excerpt := buildExcerpt(survey.Comment, survey.StoreName, earningLabel, waitLabel)

	return reviewSummaryResponse{
		ID:             survey.ID,
		StoreID:        survey.StoreID,
		StoreName:      survey.StoreName,
		BranchName:     strings.TrimSpace(survey.BranchName),
		Prefecture:     survey.Prefecture,
		Industries:     industries,
		VisitedAt:      visitedAt,
		Age:            intPtrValue(survey.Age),
		SpecScore:      spec,
		WaitTimeHours:  wait,
		AverageEarning: earning,
		Rating:         survey.Rating,
		CreatedAt:      createdAt,
		HelpfulCount:   helpful,
		Excerpt:        excerpt,
		Tags:           append([]string{}, survey.Tags...),
		Photos:         append([]publicdomain.SurveyPhoto{}, survey.Photos...),
	}
}

func buildReviewDetailFromDomain(survey publicdomain.Survey, authorName, authorAvatar string) reviewDetailResponse {
	summary := buildReviewSummaryFromDomain(survey)
	description := strings.TrimSpace(survey.Comment)
	if description == "" {
		description = buildFallbackDescription(summary)
	}
	return reviewDetailResponse{
		SurveySummary:     summary,
		Description:       description,
		AuthorDisplayName: authorName,
		AuthorAvatarURL:   authorAvatar,
		CustomerNote:      strings.TrimSpace(survey.CustomerNote),
		StaffNote:         strings.TrimSpace(survey.StaffNote),
		EnvironmentNote:   strings.TrimSpace(survey.EnvironmentNote),
		Comment:           strings.TrimSpace(survey.Comment),
	}
}

func deriveDates(period string) (visited string, created string) {
	period = strings.TrimSpace(period)
	if period == "" {
		now := time.Now()
		return now.Format("2006-01"), now.Format("2006-01-02")
	}

	replacer := strings.NewReplacer("年", "-", "月", "-01")
	normalized := replacer.Replace(period)
	t, err := time.Parse("2006-01-02", normalized)
	if err != nil {
		now := time.Now()
		return now.Format("2006-01"), now.Format("2006-01-02")
	}
	return t.Format("2006-01"), t.Format("2006-01-02")
}

func buildAdminReviewResponse(review mongodoc.ReviewDocument, store mongodoc.StoreDocument) adminReviewResponse {
	category := ""
	if len(review.Industries) > 0 {
		category = canonicalIndustryCode(review.Industries[0])
	} else if len(store.Industries) > 0 {
		category = canonicalIndustryCode(store.Industries[0])
	}
	if category == "" {
		category = "デリヘル"
	}

	industries := canonicalIndustryCodes(review.Industries)
	if len(industries) == 0 {
		industries = canonicalIndustryCodes(store.Industries)
	}

	genre := review.Genre
	if genre == "" {
		genre = store.Genre
	}

	visitedAt, _ := deriveDates(review.Period)

	waitMinutes := 0
	if review.WaitTimeMinutes != nil {
		waitMinutes = *review.WaitTimeMinutes
	}

	photos := convertSurveyPhotosForAdmin(review.Photos)

	return adminReviewResponse{
		ID:              review.ID.Hex(),
		StoreID:         review.StoreID.Hex(),
		StoreName:       store.Name,
		BranchName:      strings.TrimSpace(store.BranchName),
		Prefecture:      store.Prefecture,
		Area:            store.Area,
		Category:        category,
		Industries:      industries,
		Genre:           genre,
		VisitedAt:       visitedAt,
		Age:             intPtrValue(review.Age),
		SpecScore:       intPtrValue(review.SpecScore),
		WaitTimeMinutes: waitMinutes,
		AverageEarning:  intPtrValue(review.AverageEarning),
		EmploymentType:  review.EmploymentType,
		Rating:          review.Rating,
		Comment:         strings.TrimSpace(review.Comment),
		CustomerNote:    strings.TrimSpace(review.CustomerNote),
		StaffNote:       strings.TrimSpace(review.StaffNote),
		EnvironmentNote: strings.TrimSpace(review.EnvironmentNote),
		Tags:            append([]string{}, review.Tags...),
		ContactEmail:    review.ContactEmail,
		Photos:          photos,
		HelpfulCount:    review.HelpfulCount,
		CreatedAt:       review.CreatedAt,
		UpdatedAt:       review.UpdatedAt,
	}
}

func convertSurveyPhotosForAdmin(docs []mongodoc.SurveyPhotoDocument) []adminSurveyPhotoResponse {
	if len(docs) == 0 {
		return nil
	}
	result := make([]adminSurveyPhotoResponse, 0, len(docs))
	for _, doc := range docs {
		result = append(result, adminSurveyPhotoResponse{
			ID:          doc.ID,
			StoredPath:  doc.StoredPath,
			PublicURL:   doc.PublicURL,
			ContentType: doc.ContentType,
			UploadedAt:  doc.UploadedAt,
		})
	}
	return result
}

func deriveHelpfulCount(createdAt time.Time, spec int) int {
	base := int(createdAt.Unix()%10) + spec
	if base < 5 {
		base = 5
	}
	return base % 40
}

type updateReviewContentRequest struct {
	StoreID        *string  `json:"storeId"`
	StoreName      *string  `json:"storeName"`
	BranchName     *string  `json:"branchName"`
	Prefecture     *string  `json:"prefecture"`
	Category       *string  `json:"category"`
	VisitedAt      *string  `json:"visitedAt"`
	Age            *int     `json:"age"`
	SpecScore      *int     `json:"specScore"`
	WaitTimeHours  *int     `json:"waitTimeHours"`
	AverageEarning *int     `json:"averageEarning"`
	Comment        *string  `json:"comment"`
	Rating         *float64 `json:"rating"`
	ContactEmail   *string  `json:"contactEmail"`
}

func (s *Server) adminReviewListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		storeIDParam := strings.TrimSpace(query.Get("storeId"))
		filter := bson.M{}
		if storeIDParam != "" {
			storeID, err := primitive.ObjectIDFromHex(storeIDParam)
			if err != nil {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "店舗IDの形式が不正です"})
				return
			}
			filter["storeId"] = storeID
		}

		opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		cursor, err := s.reviews.Find(ctx, filter, opts)
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "アンケート一覧の取得に失敗しました"})
			return
		}
		defer cursor.Close(ctx)

		var reviews []mongodoc.ReviewDocument
		storeIDSet := make(map[primitive.ObjectID]struct{})
		for cursor.Next(ctx) {
			var doc mongodoc.ReviewDocument
			if err := cursor.Decode(&doc); err != nil {
				s.logger.Printf("管理リスト用レビューのデコードに失敗: %v", err)
				continue
			}
			reviews = append(reviews, doc)
			storeIDSet[doc.StoreID] = struct{}{}
		}

		if err := cursor.Err(); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "アンケート一覧の取得に失敗しました"})
			return
		}

		storeIDs := make([]primitive.ObjectID, 0, len(storeIDSet))
		for id := range storeIDSet {
			storeIDs = append(storeIDs, id)
		}

		storeMap, err := s.loadStoresMap(ctx, storeIDs)
		if err != nil {
			s.logger.Printf("管理リスト用店舗の取得に失敗: %v", err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		items := make([]adminReviewResponse, 0, len(reviews))
		for _, review := range reviews {
			store, ok := storeMap[review.StoreID]
			if !ok {
				if fetched, fetchErr := s.getStoreByID(ctx, review.StoreID); fetchErr == nil {
					store = fetched
				} else {
					s.logger.Printf("管理リスト用店舗が見つかりません reviewId=%s storeId=%s err=%v", review.ID.Hex(), review.StoreID.Hex(), fetchErr)
				}
			}
			items = append(items, buildAdminReviewResponse(review, store))
		}

		s.logger.Printf("admin review list: storeId=%q count=%d", storeIDParam, len(items))
		s.writeJSON(w, http.StatusOK, adminReviewListResponse{Items: items})
	}
}

func (s *Server) adminReviewDetailHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		s.logger.Printf("admin review detail request id=%q", idParam)
		objectID, err := primitive.ObjectIDFromHex(idParam)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "レビューIDの形式が不正です"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var review mongodoc.ReviewDocument
		if err := s.reviews.FindOne(ctx, bson.M{"_id": objectID}).Decode(&review); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				s.logger.Printf("admin review detail not found id=%q", idParam)
				s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "レビューが見つかりません"})
				return
			}
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "レビューの取得に失敗しました"})
			return
		}

		store, err := s.getStoreByID(ctx, review.StoreID)
		if err != nil {
			s.logger.Printf("admin review detail store fetch failed id=%q storeId=%s err=%v", idParam, review.StoreID.Hex(), err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		s.logger.Printf("admin review detail success id=%q", idParam)

		s.writeJSON(w, http.StatusOK, buildAdminReviewResponse(review, store))
	}
}

func (s *Server) adminReviewUpdateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		s.logger.Printf("admin review content update request id=%q", idParam)
		objectID, err := primitive.ObjectIDFromHex(idParam)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "レビューIDの形式が不正です"})
			return
		}

		var req updateReviewContentRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, maxReviewRequestBody)).Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "リクエストの形式が不正です"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var existing mongodoc.ReviewDocument
		if err := s.reviews.FindOne(ctx, bson.M{"_id": objectID}).Decode(&existing); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				s.logger.Printf("admin review content update not found id=%q", idParam)
				s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "レビューが見つかりません"})
				return
			}
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "レビューの取得に失敗しました"})
			return
		}

		reviewUpdate := bson.M{}
		storeUpdate := bson.M{}
		now := time.Now().In(s.location)
		var addIndustry string
		targetStoreID := existing.StoreID
		storeChanged := false

		if req.StoreID != nil {
			storeIDHex := strings.TrimSpace(*req.StoreID)
			if storeIDHex == "" {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "店舗IDが指定されていません"})
				return
			}
			newStoreID, err := primitive.ObjectIDFromHex(storeIDHex)
			if err != nil {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "店舗IDの形式が不正です"})
				return
			}
			if newStoreID != existing.StoreID {
				if err := s.stores.FindOne(ctx, bson.M{"_id": newStoreID}).Err(); err != nil {
					if errors.Is(err, mongo.ErrNoDocuments) {
						s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "指定された店舗が存在しません"})
						return
					}
					s.logger.Printf("admin review content update store lookup failed id=%q storeId=%s err=%v", idParam, storeIDHex, err)
					s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
					return
				}
				reviewUpdate["storeId"] = newStoreID
				targetStoreID = newStoreID
				storeChanged = true
			} else {
				targetStoreID = existing.StoreID
			}
		}

		if req.StoreName != nil {
			name := strings.TrimSpace(*req.StoreName)
			if name == "" {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "店舗名は必須です"})
				return
			}
			storeUpdate["name"] = name
		}
		if req.BranchName != nil {
			storeUpdate["branchName"] = strings.TrimSpace(*req.BranchName)
		}
		if req.Prefecture != nil {
			storeUpdate["prefecture"] = strings.TrimSpace(*req.Prefecture)
		}
		if req.Category != nil {
			category := canonicalIndustryCode(*req.Category)
			reviewUpdate["industryCode"] = category
			if category != "" {
				addIndustry = category
			}
		}
		if req.VisitedAt != nil {
			period, err := formatSurveyPeriod(strings.TrimSpace(*req.VisitedAt))
			if err != nil {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			reviewUpdate["period"] = period
		}
		if req.Age != nil {
			age := *req.Age
			if age < 18 {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "年齢は18歳以上で入力してください"})
				return
			}
			if age > 60 {
				age = 60
			}
			reviewUpdate["age"] = age
		}
		if req.SpecScore != nil {
			spec := *req.SpecScore
			if spec < 60 {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "スペックは60以上で入力してください"})
				return
			}
			if spec > 140 {
				spec = 140
			}
			reviewUpdate["specScore"] = spec
		}
		if req.WaitTimeHours != nil {
			wait := *req.WaitTimeHours
			if wait < 1 {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "待機時間は1時間以上で入力してください"})
				return
			}
			if wait > 24 {
				wait = 24
			}
			reviewUpdate["waitTimeHours"] = wait
		}
		if req.AverageEarning != nil {
			earning := *req.AverageEarning
			if earning < 0 {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "平均稼ぎは0以上で入力してください"})
				return
			}
			if earning > 20 {
				earning = 20
			}
			reviewUpdate["averageEarning"] = earning
		}
		if req.Comment != nil {
			comment := strings.TrimSpace(*req.Comment)
			if len([]rune(comment)) > 2000 {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "感想は2000文字以内で入力してください"})
				return
			}
			reviewUpdate["comment"] = comment
		}
		if req.Rating != nil {
			rating := *req.Rating
			if rating < 0 || rating > 5 {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "総評は0〜5の範囲で入力してください"})
				return
			}
			reviewUpdate["rating"] = math.Round(rating*2) / 2
		}
		if req.ContactEmail != nil {
			email, err := normalizeEmail(*req.ContactEmail)
			if err != nil {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			reviewUpdate["contactEmail"] = email
		}

		if len(storeUpdate) == 0 && len(reviewUpdate) == 0 && addIndustry == "" {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "更新内容が指定されていません"})
			return
		}

		if len(storeUpdate) > 0 && !targetStoreID.IsZero() {
			storeUpdate["updatedAt"] = now
			if _, err := s.stores.UpdateByID(ctx, targetStoreID, bson.M{"$set": storeUpdate}); err != nil {
				s.logger.Printf("admin review content update store update failed id=%q storeId=%s err=%v", idParam, targetStoreID.Hex(), err)
				s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の更新に失敗しました"})
				return
			}
		}
		if addIndustry != "" && !targetStoreID.IsZero() {
			if _, err := s.stores.UpdateByID(ctx, targetStoreID, bson.M{"$addToSet": bson.M{"industries": addIndustry}}); err != nil {
				s.logger.Printf("admin review content update industry append failed id=%q storeId=%s err=%v", idParam, targetStoreID.Hex(), err)
			}
		}

		var updated mongodoc.ReviewDocument
		if len(reviewUpdate) > 0 {
			reviewUpdate["updatedAt"] = now
			result := s.reviews.FindOneAndUpdate(ctx, bson.M{"_id": objectID}, bson.M{"$set": reviewUpdate}, options.FindOneAndUpdate().SetReturnDocument(options.After))
			if err := result.Decode(&updated); err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					s.logger.Printf("admin review content update disappeared id=%q", idParam)
					s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "レビューが見つかりません"})
					return
				}
				s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "レビューの更新に失敗しました"})
				return
			}
		} else {
			updated = existing
		}

		if err := s.recalculateStoreStats(ctx, updated.StoreID); err != nil {
			s.logger.Printf("admin review content update stats recalculation failed id=%q err=%v", idParam, err)
		}
		if storeChanged && existing.StoreID != updated.StoreID && !existing.StoreID.IsZero() {
			if err := s.recalculateStoreStats(ctx, existing.StoreID); err != nil {
				s.logger.Printf("admin review content update old store stats recalculation failed id=%q storeId=%s err=%v", idParam, existing.StoreID.Hex(), err)
			}
		}

		store, err := s.getStoreByID(ctx, updated.StoreID)
		if err != nil {
			s.logger.Printf("admin review content update store fetch failed id=%q storeId=%s err=%v", idParam, updated.StoreID.Hex(), err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		s.logger.Printf("admin review content update success id=%q storeId=%s", idParam, updated.StoreID.Hex())

		s.writeJSON(w, http.StatusOK, buildAdminReviewResponse(updated, store))
	}
}

func (s *Server) adminStoreSearchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryValues := r.URL.Query()
		prefecture := strings.TrimSpace(queryValues.Get("prefecture"))
		industry := strings.TrimSpace(queryValues.Get("industry"))
		keyword := strings.TrimSpace(queryValues.Get("q"))
		limit, _ := parsePositiveInt(queryValues.Get("limit"), 20)
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}

		filter := adminapp.StoreFilter{Prefecture: prefecture, Genre: industry, Keyword: keyword}
		paging := adminapp.Paging{Limit: limit}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		stores, err := s.adminStoreService.List(ctx, filter, paging)
		if err != nil {
			s.logger.Printf("admin store search failed: %v", err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗候補の取得に失敗しました"})
			return
		}

		items := make([]adminStoreResponse, 0, len(stores))
		for _, store := range stores {
			items = append(items, adminStoreDomainToResponse(store))
		}

		s.writeJSON(w, http.StatusOK, map[string]any{
			"items": items,
		})
	}
}

func (s *Server) adminStoreDetailHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		if idParam == "" {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "店舗IDが指定されていません"})
			return
		}
		objectID, err := primitive.ObjectIDFromHex(idParam)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "店舗IDの形式が不正です"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		store, err := s.adminStoreService.Detail(ctx, objectID.Hex())
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "店舗が見つかりません"})
				return
			}
			s.logger.Printf("admin store detail fetch failed id=%s err=%v", idParam, err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		s.writeJSON(w, http.StatusOK, adminStoreDomainToResponse(*store))
	}
}

type adminStoreCreateRequest struct {
	Name            string               `json:"name"`
	BranchName      string               `json:"branchName"`
	GroupName       string               `json:"groupName"`
	Prefecture      string               `json:"prefecture"`
	Area            string               `json:"area"`
	Genre           string               `json:"genre"`
	Industries      []string             `json:"industries"`
	EmploymentTypes []string             `json:"employmentTypes"`
	PricePerHour    int                  `json:"pricePerHour"`
	PriceRange      string               `json:"priceRange"`
	AverageEarning  int                  `json:"averageEarning"`
	BusinessHours   string               `json:"businessHours"`
	Tags            []string             `json:"tags"`
	HomepageURL     string               `json:"homepageUrl"`
	SNS             adminStoreSNSPayload `json:"sns"`
	PhotoURLs       []string             `json:"photoUrls"`
	Description     string               `json:"description"`
}

type adminStoreCreateResponse struct {
	Store   adminStoreResponse `json:"store"`
	Created bool               `json:"created"`
}

func (req adminStoreCreateRequest) toCommand() (adminapp.UpsertStoreCommand, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return adminapp.UpsertStoreCommand{}, errors.New("店舗名は必須です")
	}
	prefecture := strings.TrimSpace(req.Prefecture)
	if prefecture == "" {
		return adminapp.UpsertStoreCommand{}, errors.New("都道府県は必須です")
	}

	industries, err := normalizeIndustryList(req.Industries)
	if err != nil {
		return adminapp.UpsertStoreCommand{}, err
	}

	employmentTypes, err := normalizeEmploymentTypes(req.EmploymentTypes)
	if err != nil {
		return adminapp.UpsertStoreCommand{}, err
	}

	tags, err := normalizeStoreTags(req.Tags)
	if err != nil {
		return adminapp.UpsertStoreCommand{}, err
	}

	photos, err := normalizePhotoURLs(req.PhotoURLs, maxStorePhotoCount)
	if err != nil {
		return adminapp.UpsertStoreCommand{}, err
	}

	description := strings.TrimSpace(req.Description)
	if utf8.RuneCountInString(description) > maxStoreDescriptionRunes {
		return adminapp.UpsertStoreCommand{}, fmt.Errorf("店舗説明は最大%d文字までです", maxStoreDescriptionRunes)
	}

	if req.PricePerHour < 0 {
		return adminapp.UpsertStoreCommand{}, errors.New("単価は0以上で入力してください")
	}
	if req.AverageEarning < 0 {
		return adminapp.UpsertStoreCommand{}, errors.New("平均稼ぎは0以上で入力してください")
	}

	return adminapp.UpsertStoreCommand{
		Name:            name,
		BranchName:      strings.TrimSpace(req.BranchName),
		GroupName:       strings.TrimSpace(req.GroupName),
		Prefecture:      prefecture,
		Area:            strings.TrimSpace(req.Area),
		Genre:           strings.TrimSpace(req.Genre),
		Industries:      industries,
		EmploymentTypes: employmentTypes,
		PricePerHour:    req.PricePerHour,
		PriceRange:      strings.TrimSpace(req.PriceRange),
		AverageEarning:  req.AverageEarning,
		BusinessHours:   strings.TrimSpace(req.BusinessHours),
		Tags:            tags,
		HomepageURL:     strings.TrimSpace(req.HomepageURL),
		SNS: adminapp.StoreSNSCommand{
			Twitter:   strings.TrimSpace(req.SNS.Twitter),
			Line:      strings.TrimSpace(req.SNS.Line),
			Instagram: strings.TrimSpace(req.SNS.Instagram),
			TikTok:    strings.TrimSpace(req.SNS.TikTok),
			Official:  strings.TrimSpace(req.SNS.Official),
		},
		PhotoURLs:   photos,
		Description: description,
	}, nil
}

func normalizeIndustryList(values []string) ([]string, error) {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))

	appendIndustry := func(raw string) error {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil
		}
		code := canonicalIndustryCode(raw)
		if code == "" {
			return fmt.Errorf("無効な業種です: %s", raw)
		}
		if _, ok := seen[code]; ok {
			return nil
		}
		seen[code] = struct{}{}
		result = append(result, code)
		return nil
	}

	for _, raw := range values {
		if err := appendIndustry(raw); err != nil {
			return nil, err
		}
	}

	if len(result) == 0 {
		return nil, errors.New("業種は1件以上指定してください")
	}

	return result, nil
}

func normalizeEmploymentTypes(types []string) ([]string, error) {
	if len(types) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, len(types))
	for _, raw := range types {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := allowedEmploymentTypeSet[value]; !ok {
			return nil, fmt.Errorf("無効な勤務形態です: %s", raw)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func normalizeStoreTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if _, ok := allowedStoreTagSet[tag]; !ok {
			return nil, fmt.Errorf("無効なタグです: %s", raw)
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result, nil
}

const maxURLLength = 2048

func normalizePhotoURLs(urls []string, max int) ([]string, error) {
	if len(urls) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, len(urls))
	for _, raw := range urls {
		urlStr := strings.TrimSpace(raw)
		if urlStr == "" {
			continue
		}
		if len(urlStr) > maxURLLength {
			return nil, fmt.Errorf("URLが長すぎます: %s", urlStr)
		}
		if _, ok := seen[urlStr]; ok {
			continue
		}
		seen[urlStr] = struct{}{}
		result = append(result, urlStr)
		if len(result) > max {
			return nil, fmt.Errorf("写真URLは最大%d件までです", max)
		}
	}
	return result, nil
}

func normalizeReviewPhotos(payloads []reviewPhotoPayload, max int) ([]publicdomain.SurveyPhoto, error) {
	if len(payloads) == 0 {
		return nil, nil
	}
	result := make([]publicdomain.SurveyPhoto, 0, len(payloads))
	for _, payload := range payloads {
		id := strings.TrimSpace(payload.ID)
		publicURL := strings.TrimSpace(payload.PublicURL)
		if id == "" {
			return nil, errors.New("写真IDは必須です")
		}
		if publicURL == "" {
			return nil, fmt.Errorf("写真 %s の公開URLを指定してください", id)
		}
		storedPath := strings.TrimSpace(payload.StoredPath)
		if storedPath == "" {
			storedPath = id
		}
		result = append(result, publicdomain.SurveyPhoto{
			ID:          id,
			StoredPath:  storedPath,
			PublicURL:   publicURL,
			ContentType: strings.TrimSpace(payload.ContentType),
			UploadedAt:  time.Now().UTC(),
		})
		if len(result) > max {
			return nil, fmt.Errorf("写真は最大%d枚までです", max)
		}
	}
	return result, nil
}

type adminStoreReviewCreateRequest struct {
	VisitedAt      string  `json:"visitedAt"`
	Age            int     `json:"age"`
	SpecScore      int     `json:"specScore"`
	WaitTimeHours  int     `json:"waitTimeHours"`
	AverageEarning int     `json:"averageEarning"`
	Comment        string  `json:"comment"`
	Rating         float64 `json:"rating"`
	IndustryCode   string  `json:"industryCode"`
	ContactEmail   string  `json:"contactEmail,omitempty"`
}

func (s *Server) adminStoreCreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req adminStoreCreateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, maxReviewRequestBody)).Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "リクエストの形式が不正です"})
			return
		}

		cmd, err := req.toCommand()
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		store, err := s.adminStoreService.Create(ctx, cmd)
		if err != nil {
			s.logger.Printf("admin store create failed: %v", err)
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		s.writeJSON(w, http.StatusCreated, adminStoreCreateResponse{
			Store:   adminStoreDomainToResponse(*store),
			Created: true,
		})
	}
}

func (s *Server) adminStoreReviewCreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		storeIDParam := strings.TrimSpace(chi.URLParam(r, "id"))
		if storeIDParam == "" {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "店舗IDが指定されていません"})
			return
		}
		storeID, err := primitive.ObjectIDFromHex(storeIDParam)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "店舗IDの形式が不正です"})
			return
		}

		var req adminStoreReviewCreateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, maxReviewRequestBody)).Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "リクエストの形式が不正です"})
			return
		}

		metrics := reviewMetrics{
			VisitedAt:      req.VisitedAt,
			Age:            req.Age,
			SpecScore:      req.SpecScore,
			WaitTimeHours:  req.WaitTimeHours,
			AverageEarning: req.AverageEarning,
			Comment:        req.Comment,
			Rating:         req.Rating,
			ContactEmail:   req.ContactEmail,
		}
		if err := metrics.normalize(); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		period, err := formatSurveyPeriod(metrics.VisitedAt)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		store, err := s.getStoreByID(ctx, storeID)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "指定された店舗が見つかりません"})
				return
			}
			s.logger.Printf("admin store review create store fetch failed id=%s err=%v", storeID.Hex(), err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		category := canonicalIndustryCode(req.IndustryCode)
		if category == "" && len(store.Industries) > 0 {
			category = canonicalIndustryCode(store.Industries[0])
		}
		if category == "" {
			category = "デリヘル"
		}

		waitMinutes := metrics.WaitTimeHours * 60

		now := time.Now().In(s.location)
		reviewDoc := mongodoc.ReviewDocument{
			ID:              primitive.NewObjectID(),
			StoreID:         storeID,
			StoreName:       store.Name,
			BranchName:      store.BranchName,
			Prefecture:      store.Prefecture,
			Area:            store.Area,
			Industries:      []string{category},
			Genre:           store.Genre,
			Period:          period,
			Age:             intPtr(metrics.Age),
			SpecScore:       intPtr(metrics.SpecScore),
			WaitTimeMinutes: intPtr(waitMinutes),
			AverageEarning:  intPtr(metrics.AverageEarning),
			EmploymentType:  "",
			CustomerNote:    metrics.Comment,
			StaffNote:       "",
			EnvironmentNote: "",
			Rating:          metrics.Rating,
			Comment:         metrics.Comment,
			ContactEmail:    metrics.ContactEmail,
			Photos:          nil,
			Tags:            nil,
			HelpfulCount:    0,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if _, err := s.reviews.InsertOne(ctx, reviewDoc); err != nil {
			s.logger.Printf("admin store review create insert failed storeId=%s err=%v", storeID.Hex(), err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "レビューの保存に失敗しました"})
			return
		}

		if category != "" && !containsString(store.Industries, category) {
			if _, err := s.stores.UpdateByID(ctx, storeID, bson.M{"$addToSet": bson.M{"industries": category}}); err != nil {
				s.logger.Printf("admin store review create industry add failed storeId=%s err=%v", storeID.Hex(), err)
			} else {
				store, _ = s.getStoreByID(ctx, storeID)
			}
		}

		if err := s.recalculateStoreStats(ctx, storeID); err != nil {
			s.logger.Printf("admin store review create stats failed storeId=%s err=%v", storeID.Hex(), err)
		} else if refreshed, err := s.getStoreByID(ctx, storeID); err == nil {
			store = refreshed
		}

		s.writeJSON(w, http.StatusCreated, buildAdminReviewResponse(reviewDoc, store))
	}
}

func storeDocumentToAdminResponse(doc mongodoc.StoreDocument) adminStoreResponse {
	return adminStoreResponse{
		ID:              doc.ID.Hex(),
		Name:            doc.Name,
		BranchName:      strings.TrimSpace(doc.BranchName),
		GroupName:       doc.GroupName,
		Prefecture:      doc.Prefecture,
		Area:            doc.Area,
		Genre:           doc.Genre,
		Industries:      canonicalIndustryCodes(doc.Industries),
		EmploymentTypes: append([]string{}, doc.EmploymentTypes...),
		BusinessHours:   doc.BusinessHours,
		PricePerHour:    doc.PricePerHour,
		PriceRange:      doc.PriceRange,
		AverageEarning:  doc.AverageEarning,
		Tags:            append([]string{}, doc.Tags...),
		HomepageURL:     doc.HomepageURL,
		SNS: adminStoreSNSPayload{
			Twitter:   doc.SNS.Twitter,
			Line:      doc.SNS.Line,
			Instagram: doc.SNS.Instagram,
			TikTok:    doc.SNS.TikTok,
			Official:  doc.SNS.Official,
		},
		PhotoURLs:      append([]string{}, doc.PhotoURLs...),
		Description:    doc.Description,
		ReviewCount:    doc.Stats.ReviewCount,
		LastReviewedAt: doc.Stats.LastReviewedAt,
	}
}

func storeDocumentToDetailResponse(doc mongodoc.StoreDocument) storeDetailResponse {
	industries := canonicalIndustryCodes(doc.Industries)
	avgRating := 0.0
	if doc.Stats.AvgRating != nil {
		avgRating = math.Round(*doc.Stats.AvgRating*10) / 10
	}

	avgEarning := 0
	avgEarningLabel := "-"
	if doc.Stats.AvgEarning != nil {
		avgEarning = int(math.Round(*doc.Stats.AvgEarning))
		avgEarningLabel = formatAverageEarningLabel(avgEarning)
	}

	waitHours := 0
	waitLabel := "-"
	if doc.Stats.AvgWaitTime != nil {
		waitHours = int(math.Round(*doc.Stats.AvgWaitTime))
		waitLabel = formatWaitTimeLabel(waitHours)
	}

	return storeDetailResponse{
		ID:                  doc.ID.Hex(),
		StoreName:           doc.Name,
		BranchName:          strings.TrimSpace(doc.BranchName),
		Prefecture:          doc.Prefecture,
		Area:                doc.Area,
		Genre:               doc.Genre,
		BusinessHours:       doc.BusinessHours,
		PriceRange:          doc.PriceRange,
		Industries:          industries,
		EmploymentTypes:     append([]string{}, doc.EmploymentTypes...),
		PricePerHour:        doc.PricePerHour,
		AverageRating:       avgRating,
		AverageEarning:      avgEarning,
		AverageEarningLabel: avgEarningLabel,
		WaitTimeHours:       waitHours,
		WaitTimeLabel:       waitLabel,
		ReviewCount:         doc.Stats.ReviewCount,
		LastReviewedAt:      doc.Stats.LastReviewedAt,
		UpdatedAt:           doc.UpdatedAt,
		Tags:                append([]string{}, doc.Tags...),
		PhotoURLs:           append([]string{}, doc.PhotoURLs...),
		HomepageURL:         doc.HomepageURL,
		SNS: storeSNSPayload{
			Twitter:   doc.SNS.Twitter,
			Line:      doc.SNS.Line,
			Instagram: doc.SNS.Instagram,
			TikTok:    doc.SNS.TikTok,
			Official:  doc.SNS.Official,
		},
		Description: doc.Description,
	}
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) == target {
			return true
		}
	}
	return false
}

func extractFirstInt(value any) int {
	switch v := value.(type) {
	case *int:
		if v == nil {
			return 0
		}
		return *v
	case *int32:
		if v == nil {
			return 0
		}
		return int(*v)
	case *int64:
		if v == nil {
			return 0
		}
		return int(*v)
	case *float64:
		if v == nil {
			return 0
		}
		return int(math.Round(*v))
	case int32:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	case float64:
		return int(math.Round(v))
	case string:
		match := numberPattern.FindString(v)
		if match == "" {
			return 0
		}
		num, err := strconv.Atoi(match)
		if err != nil {
			return 0
		}
		return num
	default:
		return 0
	}
}

func buildExcerpt(comment, storeName string, earning any, wait any) string {
	trimmed := strings.TrimSpace(comment)
	if trimmed != "" {
		runes := []rune(trimmed)
		if len(runes) > 60 {
			trimmed = string(runes[:60]) + "…"
		}
		return trimmed
	}

	components := []string{}
	if earningValue := extractFirstInt(earning); earningValue > 0 {
		components = append(components, fmt.Sprintf("平均稼ぎは%d万円", earningValue))
	}
	if waitValue := extractFirstInt(wait); waitValue > 0 {
		components = append(components, fmt.Sprintf("待機は%d時間程度", waitValue))
	}
	if len(components) == 0 {
		return fmt.Sprintf("%sの最新アンケートです。", storeName)
	}
	return strings.Join(components, "／")
}

func buildFallbackDescription(summary reviewSummaryResponse) string {
	return fmt.Sprintf(
		"%sでの体験談です。平均稼ぎはおよそ%d万円、待機時間は%d時間程度でした。年代: %d歳、スペック: %d を参考にしてください。",
		summary.StoreName,
		summary.AverageEarning,
		summary.WaitTimeHours,
		summary.Age,
		summary.SpecScore,
	)
}

func sortReviews(reviews []reviewSummaryResponse, sortKey string) {
	switch sortKey {
	case "helpful":
		sort.SliceStable(reviews, func(i, j int) bool {
			if reviews[i].HelpfulCount == reviews[j].HelpfulCount {
				return reviews[i].CreatedAt > reviews[j].CreatedAt
			}
			return reviews[i].HelpfulCount > reviews[j].HelpfulCount
		})
	case "earning":
		sort.SliceStable(reviews, func(i, j int) bool {
			if reviews[i].AverageEarning == reviews[j].AverageEarning {
				return reviews[i].CreatedAt > reviews[j].CreatedAt
			}
			return reviews[i].AverageEarning > reviews[j].AverageEarning
		})
	default:
		sort.SliceStable(reviews, func(i, j int) bool {
			return reviews[i].CreatedAt > reviews[j].CreatedAt
		})
	}
}
func (s *Server) reviewListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := r.URL.Query()
		categoryRaw := strings.TrimSpace(query.Get("category"))
		storeIDParam := strings.TrimSpace(query.Get("storeId"))
		var storeID primitive.ObjectID
		if storeIDParam != "" {
			objectID, err := primitive.ObjectIDFromHex(storeIDParam)
			if err != nil {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "店舗IDの形式が不正です"})
				return
			}
			storeID = objectID
		}
		params := reviewQueryParams{
			Prefecture: strings.TrimSpace(query.Get("prefecture")),
			Category:   canonicalIndustryCode(categoryRaw),
			StoreName:  strings.TrimSpace(query.Get("storeName")),
			StoreID:    storeID,
			Sort:       strings.TrimSpace(query.Get("sort")),
		}
		params.Page, _ = parsePositiveInt(query.Get("page"), 1)
		params.Limit, _ = parsePositiveInt(query.Get("limit"), 10)
		if params.Limit <= 0 {
			params.Limit = 10
		}

		reviews, err := s.collectReviews(ctx, params)
		if err != nil {
			s.logger.Printf("レビュー一覧の取得に失敗: %v", err)
			http.Error(w, "レビュー一覧の取得に失敗しました", http.StatusInternalServerError)
			return
		}

		total := len(reviews)
		start := (params.Page - 1) * params.Limit
		if start >= total {
			start = total
		}
		end := start + params.Limit
		if end > total {
			end = total
		}

		s.writeJSON(w, http.StatusOK, reviewListResponse{
			Items: reviews[start:end],
			Page:  params.Page,
			Limit: params.Limit,
			Total: total,
		})
	}
}

func (s *Server) reviewLatestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		reviews, err := s.collectReviews(ctx, reviewQueryParams{Sort: "newest", Limit: 3})
		if err != nil {
			s.logger.Printf("最新レビューの取得に失敗: %v", err)
			http.Error(w, "最新レビューの取得に失敗しました", http.StatusInternalServerError)
			return
		}
		if len(reviews) > 3 {
			reviews = reviews[:3]
		}
		if reviews == nil {
			reviews = []reviewSummaryResponse{}
		}
		s.logger.Printf("review latest list count=%d", len(reviews))
		s.writeJSON(w, http.StatusOK, reviews)
	}
}

func (s *Server) reviewHighRatedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		reviews, err := s.collectReviews(ctx, reviewQueryParams{Sort: "helpful", Limit: 3})
		if err != nil {
			s.logger.Printf("高評価レビューの取得に失敗: %v", err)
			http.Error(w, "高評価レビューの取得に失敗しました", http.StatusInternalServerError)
			return
		}
		if len(reviews) > 3 {
			reviews = reviews[:3]
		}
		if reviews == nil {
			reviews = []reviewSummaryResponse{}
		}
		s.logger.Printf("admin review high-rated list count=%d", len(reviews))
		s.writeJSON(w, http.StatusOK, reviews)
	}
}

func (s *Server) reviewHelpfulToggleHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		if idParam == "" {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "IDが指定されていません"})
			return
		}
		if _, err := primitive.ObjectIDFromHex(idParam); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "不正なIDです"})
			return
		}

		payload := struct {
			Helpful *bool `json:"helpful"`
		}{}
		desired := true
		if r.Body != nil {
			defer r.Body.Close()
			decoder := json.NewDecoder(io.LimitReader(r.Body, 1024))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&payload); err != nil && err != io.EOF {
				s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "リクエストの形式が不正です"})
				return
			}
		}
		if payload.Helpful != nil {
			desired = *payload.Helpful
		}

		voterID, err := s.ensureHelpfulVoterID(w, r)
		if err != nil {
			s.logger.Printf("helpful voter cookie error: %v", err)
			http.Error(w, "役に立った投票処理に失敗しました", http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		count, err := s.surveyCommandService.ToggleHelpful(ctx, idParam, voterID, desired)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				http.NotFound(w, r)
				return
			}
			s.logger.Printf("helpful toggle failed: survey=%s voter=%s err=%v", idParam, voterID, err)
			http.Error(w, "役に立った情報の更新に失敗しました", http.StatusInternalServerError)
			return
		}

		s.writeJSON(w, http.StatusOK, map[string]any{
			"helpfulCount": count,
			"helpful":      desired,
		})
	}
}

func (s *Server) reviewDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	idParam := chi.URLParam(r, "id")
	if idParam == "" {
		http.Error(w, "IDが指定されていません", http.StatusBadRequest)
		return
	}

	if _, err := primitive.ObjectIDFromHex(idParam); err != nil {
		http.Error(w, "不正なIDです", http.StatusBadRequest)
		return
	}

	survey, err := s.surveyQueryService.Detail(ctx, idParam)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			http.NotFound(w, r)
			return
		}
		s.logger.Printf("レビュー詳細の取得に失敗: %v", err)
		http.Error(w, "レビュー詳細の取得に失敗しました", http.StatusInternalServerError)
		return
	}

	detail := buildReviewDetailFromDomain(*survey, "匿名店舗アンケート", "")
	s.writeJSON(w, http.StatusOK, detail)
}
