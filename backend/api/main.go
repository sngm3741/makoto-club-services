package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type config struct {
	addr                 string
	mongoURI             string
	mongoDatabase        string
	pingCollection       string
	storeCollection      string
	reviewCollection     string
	timeout              time.Duration
	timezone             string
	serverLog            *log.Logger
	jwtConfigs           []jwtConfig
	jwtAudience          string
	messengerEndpoint    string
	messengerDestination string
	discordDestination   string
	messengerTimeout     time.Duration
	adminReviewBaseURL   string
	allowedOrigins       []string
	mediaBaseURL         string
}

type server struct {
	logger               *log.Logger
	client               *mongo.Client
	database             *mongo.Database
	pings                *mongo.Collection
	stores               *mongo.Collection
	reviews              *mongo.Collection
	location             *time.Location
	jwtConfigs           []jwtConfig
	jwtAudience          string
	httpClient           *http.Client
	messengerEndpoint    string
	messengerDestination string
	discordDestination   string
	adminReviewBaseURL   string
	mediaBaseURL         string
}

type jwtConfig struct {
	issuer string
	secret []byte
}

type contextKey string

const authUserContextKey contextKey = "authUser"

var jstLocation = time.FixedZone("JST", 9*60*60)

type authenticatedUser struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
	Picture  string `json:"picture,omitempty"`
}

type storeStatsDocument struct {
	ReviewCount    int        `bson:"reviewCount"`
	AvgRating      *float64   `bson:"avgRating,omitempty"`
	AvgEarning     *float64   `bson:"avgEarning,omitempty"`
	AvgWaitTime    *float64   `bson:"avgWaitTime,omitempty"`
	LastReviewedAt *time.Time `bson:"lastReviewedAt,omitempty"`
}

type storeDocument struct {
	ID            primitive.ObjectID `bson:"_id"`
	Name          string             `bson:"name"`
	BranchName    string             `bson:"branchName,omitempty"`
	Prefecture    string             `bson:"prefecture,omitempty"`
	IndustryCodes []string           `bson:"industryCodes,omitempty"`
	Stats         storeStatsDocument `bson:"stats"`
	CreatedAt     *time.Time         `bson:"createdAt,omitempty"`
	UpdatedAt     *time.Time         `bson:"updatedAt,omitempty"`
}

type reviewAttachmentDocument struct {
	StoredFilename string `bson:"storedFilename"`
	ContentType    string `bson:"contentType,omitempty"`
}

type reviewRewardDocument struct {
	Status string     `bson:"status"`
	SentAt *time.Time `bson:"sentAt,omitempty"`
	Note   string     `bson:"note,omitempty"`
}

type reviewDocument struct {
	ID               primitive.ObjectID         `bson:"_id"`
	StoreID          primitive.ObjectID         `bson:"storeId"`
	IndustryCode     string                     `bson:"industryCode"`
	Status           string                     `bson:"status"`
	StatusNote       string                     `bson:"statusNote,omitempty"`
	ReviewedAt       *time.Time                 `bson:"reviewedAt,omitempty"`
	ReviewedBy       string                     `bson:"reviewedBy,omitempty"`
	Period           string                     `bson:"period,omitempty"`
	Age              *int                       `bson:"age,omitempty"`
	SpecScore        *int                       `bson:"specScore,omitempty"`
	WaitTimeHours    *int                       `bson:"waitTimeHours,omitempty"`
	AverageEarning   *int                       `bson:"averageEarning,omitempty"`
	Rating           float64                    `bson:"rating"`
	Comment          string                     `bson:"comment"`
	Attachments      []reviewAttachmentDocument `bson:"attachments,omitempty"`
	Reward           reviewRewardDocument       `bson:"reward"`
	ReviewerID       string                     `bson:"reviewerId,omitempty"`
	ReviewerName     string                     `bson:"reviewerName,omitempty"`
	ReviewerUsername string                     `bson:"reviewerUsername,omitempty"`
	CreatedAt        time.Time                  `bson:"createdAt"`
	UpdatedAt        time.Time                  `bson:"updatedAt"`
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
	router.Use(withCORS(cfg.allowedOrigins))

	router.Get("/api/healthz", srv.healthHandler())
	router.Get("/api/ping", srv.pingHandler())
	router.Get("/api/stores", srv.storeListHandler())
	router.Get("/api/reviews", srv.reviewListHandler())
	router.Get("/api/reviews/new", srv.reviewLatestHandler())
	router.Get("/api/reviews/high-rated", srv.reviewHighRatedHandler())
	router.Get("/api/reviews/{id}", srv.reviewDetailHandler)
	router.With(srv.authMiddleware).Post("/api/reviews", srv.reviewCreateHandler())
	router.With(srv.authMiddleware).Get("/api/auth/verify", srv.authVerifyHandler())
	router.Route("/api/admin", func(r chi.Router) {
		r.Get("/reviews", srv.adminReviewListHandler())
		r.Get("/reviews/{id}", srv.adminReviewDetailHandler())
		r.Patch("/reviews/{id}", srv.adminReviewUpdateHandler())
		r.Patch("/reviews/{id}/status", srv.adminReviewStatusHandler())
		r.Get("/stores", srv.adminStoreSearchHandler())
		r.Post("/stores", srv.adminStoreCreateHandler())
	})

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

	messengerEndpoint := strings.TrimSpace(os.Getenv("MESSENGER_GATEWAY_URL"))
	if messengerEndpoint == "" {
		messengerEndpoint = "http://messenger-gateway:3000"
	}

	messengerDestination := strings.TrimSpace(os.Getenv("MESSENGER_GATEWAY_DESTINATION"))
	if messengerDestination == "" {
		messengerDestination = "line"
	}

	discordDestination := strings.TrimSpace(os.Getenv("MESSENGER_DISCORD_INCOMING_DESTINATION"))

	messengerTimeout := 3 * time.Second
	if raw := strings.TrimSpace(os.Getenv("MESSENGER_GATEWAY_TIMEOUT")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			messengerTimeout = parsed
		}
	}
	allowedOrigins := parseList("API_ALLOWED_ORIGINS", []string{"*"})
	adminReviewBaseURL := strings.TrimSpace(os.Getenv("ADMIN_REVIEW_BASE_URL"))

	var jwtConfigs []jwtConfig
	if secret := strings.TrimSpace(os.Getenv("AUTH_LINE_JWT_SECRET")); secret != "" {
		jwtConfigs = append(jwtConfigs, jwtConfig{
			issuer: envOrDefault("AUTH_LINE_JWT_ISSUER", "makoto-club-auth"),
			secret: []byte(secret),
		})
	}
	if secret := strings.TrimSpace(os.Getenv("AUTH_TWITTER_JWT_SECRET")); secret != "" {
		jwtConfigs = append(jwtConfigs, jwtConfig{
			issuer: envOrDefault("AUTH_TWITTER_JWT_ISSUER", "auth-twitter"),
			secret: []byte(secret),
		})
	}

	if len(jwtConfigs) == 0 {
		log.Fatal("JWT secrets not configured. Set AUTH_TWITTER_JWT_SECRET or AUTH_LINE_JWT_SECRET.")
	}

	jwtAudience := strings.TrimSpace(os.Getenv("AUTH_JWT_AUDIENCE"))
	if jwtAudience == "" {
		jwtAudience = strings.TrimSpace(os.Getenv("AUTH_LINE_JWT_AUDIENCE"))
	}
	if jwtAudience == "" {
		jwtAudience = strings.TrimSpace(os.Getenv("AUTH_TWITTER_JWT_AUDIENCE"))
	}

	storeCollection := envOrDefault("STORE_COLLECTION", "stores")
	reviewCollection := strings.TrimSpace(os.Getenv("REVIEW_COLLECTION"))
	if reviewCollection == "" {
		reviewCollection = envOrDefault("SURVEY_COLLECTION", "reviews")
	}

	cfgStruct := config{
		addr:                 envOrDefault("HTTP_ADDR", ":8080"),
		mongoURI:             envOrDefault("MONGO_URI", "mongodb://mongo:27017"),
		mongoDatabase:        envOrDefault("MONGO_DB", "makoto-club"),
		storeCollection:      storeCollection,
		reviewCollection:     reviewCollection,
		pingCollection:       envOrDefault("PING_COLLECTION", "pings"),
		timeout:              timeout,
		timezone:             envOrDefault("TIMEZONE", "Asia/Tokyo"),
		serverLog:            log.New(os.Stdout, "[makoto-club-api] ", log.LstdFlags|log.Lshortfile),
		jwtConfigs:           jwtConfigs,
		jwtAudience:          jwtAudience,
		messengerEndpoint:    messengerEndpoint,
		messengerDestination: messengerDestination,
		discordDestination:   discordDestination,
		messengerTimeout:     messengerTimeout,
		adminReviewBaseURL:   adminReviewBaseURL,
		allowedOrigins:       allowedOrigins,
		mediaBaseURL:         strings.TrimSpace(os.Getenv("MEDIA_BASE_URL")),
	}

	cfgStruct.serverLog.Printf("loaded config: adminReviewBaseURL=%q messengerEndpoint=%q destination=%q", adminReviewBaseURL, messengerEndpoint, messengerDestination)

	return cfgStruct
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseList(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}

	if len(values) == 0 {
		return fallback
	}
	return values
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

func (s *server) mediaURL(storedFilename string) string {
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

func (s *server) findStoreIDs(ctx context.Context, prefecture, name string) ([]primitive.ObjectID, error) {
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

func (s *server) loadStoresMap(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]storeDocument, error) {
	result := make(map[primitive.ObjectID]storeDocument, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	cursor, err := s.stores.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc storeDocument
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

func (s *server) getStoreByID(ctx context.Context, id primitive.ObjectID) (storeDocument, error) {
	var store storeDocument
	err := s.stores.FindOne(ctx, bson.M{"_id": id}).Decode(&store)
	return store, err
}

func (s *server) findOrCreateStore(ctx context.Context, name, branch, prefecture, category string) (storeDocument, error) {
	name = strings.TrimSpace(name)
	branch = strings.TrimSpace(branch)
	prefecture = strings.TrimSpace(prefecture)
	category = canonicalIndustryCode(category)
	if name == "" {
		return storeDocument{}, errors.New("店舗名が指定されていません")
	}

	filter := bson.M{"name": name}
	if branch != "" {
		filter["branchName"] = branch
	}
	if prefecture != "" {
		filter["prefecture"] = prefecture
	}

	var store storeDocument
	err := s.stores.FindOne(ctx, filter).Decode(&store)
	if err == nil {
		return store, nil
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return storeDocument{}, err
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
		doc["industryCodes"] = bson.A{category}
	}

	if _, err := s.stores.InsertOne(ctx, doc); err != nil {
		return storeDocument{}, err
	}

	return s.getStoreByID(ctx, newID)
}

func (s *server) recalculateStoreStats(ctx context.Context, storeID primitive.ObjectID) error {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"storeId": storeID,
			"status":  "approved",
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

func newServer(cfg config, client *mongo.Client) *server {
	loc, err := time.LoadLocation(cfg.timezone)
	if err != nil {
		loc = time.FixedZone("JST", 9*60*60)
		cfg.serverLog.Printf("タイムゾーン %s の読み込みに失敗: %v, JST を使用します", cfg.timezone, err)
	}

	endpoint := normaliseBaseURL(cfg.messengerEndpoint)
	if endpoint == "" {
		endpoint = "http://messenger-gateway:3000"
	}

	srv := &server{
		logger:               cfg.serverLog,
		client:               client,
		database:             client.Database(cfg.mongoDatabase),
		location:             loc,
		jwtConfigs:           append([]jwtConfig(nil), cfg.jwtConfigs...),
		jwtAudience:          cfg.jwtAudience,
		httpClient:           &http.Client{Timeout: cfg.messengerTimeout},
		messengerEndpoint:    endpoint,
		messengerDestination: cfg.messengerDestination,
		discordDestination:   cfg.discordDestination,
		adminReviewBaseURL:   cfg.adminReviewBaseURL,
		mediaBaseURL:         strings.TrimSuffix(strings.TrimSpace(cfg.mediaBaseURL), "/"),
	}
	srv.pings = srv.database.Collection(cfg.pingCollection)
	srv.stores = srv.database.Collection(cfg.storeCollection)
	srv.reviews = srv.database.Collection(cfg.reviewCollection)
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
			"status": "ok_test２",
			"time":   now.Format(time.RFC3339),
		})
	}
}

func (s *server) authMiddleware(next http.Handler) http.Handler {
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

		ctx := context.WithValue(r.Context(), authUserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *server) parseAuthToken(tokenString string) (*authClaims, error) {
	if len(s.jwtConfigs) == 0 {
		return nil, fmt.Errorf("認証設定が構成されていません")
	}

	for _, cfg := range s.jwtConfigs {
		claims := &authClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
			}
			return cfg.secret, nil
		}, jwt.WithLeeway(30*time.Second))

		if err != nil || !token.Valid {
			continue
		}

		if cfg.issuer != "" && claims.Issuer != cfg.issuer {
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

func (s *server) authVerifyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUserFromContext(r.Context())
		if !ok {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "認証情報の取得に失敗しました"})
			return
		}

		s.writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"user":   user,
		})
	}
}

func authenticatedUserFromContext(ctx context.Context) (authenticatedUser, bool) {
	user, ok := ctx.Value(authUserContextKey).(authenticatedUser)
	return user, ok
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

func (s *server) storeListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := r.URL.Query()
		prefectureFilter := strings.TrimSpace(query.Get("prefecture"))
		categoryFilter := canonicalIndustryCode(query.Get("category"))
		page, _ := parsePositiveInt(query.Get("page"), 1)
		limit, _ := parsePositiveInt(query.Get("limit"), 10)
		if limit <= 0 {
			limit = 10
		}

		filter := bson.M{
			"stats.reviewCount": bson.M{"$gt": 0},
		}
		if prefectureFilter != "" {
			filter["prefecture"] = prefectureFilter
		}
		if categoryFilter != "" {
			filter["industryCodes"] = categoryFilter
		}

		cursor, err := s.stores.Find(ctx, filter)
		if err != nil {
			s.logger.Printf("店舗情報の取得に失敗: %v", err)
			http.Error(w, "店舗情報の取得に失敗しました", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		summaries := make([]storeSummaryResponse, 0)
		for cursor.Next(ctx) {
			var store storeDocument
			if err := cursor.Decode(&store); err != nil {
				s.logger.Printf("店舗ドキュメントのデコードに失敗: %v", err)
				continue
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

			category := categoryFilter
			if category == "" && len(store.IndustryCodes) > 0 {
				category = store.IndustryCodes[0]
			}

			avgRating := 0.0
			if store.Stats.AvgRating != nil {
				avgRating = math.Round(*store.Stats.AvgRating*10) / 10
			}

			summary := storeSummaryResponse{
				ID:                  store.ID.Hex(),
				StoreName:           store.Name,
				Prefecture:          store.Prefecture,
				Category:            category,
				AverageRating:       avgRating,
				AverageEarning:      avgEarning,
				AverageEarningLabel: avgEarningLabel,
				WaitTimeHours:       waitHours,
				WaitTimeLabel:       waitLabel,
				ReviewCount:         store.Stats.ReviewCount,
			}
			summaries = append(summaries, summary)
		}

		if err := cursor.Err(); err != nil {
			s.logger.Printf("店舗カーソル処理中にエラー: %v", err)
			http.Error(w, "店舗情報の処理に失敗しました", http.StatusInternalServerError)
			return
		}

		sort.Slice(summaries, func(i, j int) bool {
			if summaries[i].Prefecture == summaries[j].Prefecture {
				return summaries[i].StoreName < summaries[j].StoreName
			}
			return summaries[i].Prefecture < summaries[j].Prefecture
		})

		total := len(summaries)
		start := (page - 1) * limit
		if start >= total {
			start = total
		}
		end := start + limit
		if end > total {
			end = total
		}

		s.writeJSON(w, http.StatusOK, storeListResponse{
			Items: summaries[start:end],
			Page:  page,
			Limit: limit,
			Total: total,
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

type reviewSummaryResponse struct {
	ID             string  `json:"id"`
	StoreID        string  `json:"storeId"`
	StoreName      string  `json:"storeName"`
	BranchName     string  `json:"branchName,omitempty"`
	Prefecture     string  `json:"prefecture"`
	Category       string  `json:"category"`
	VisitedAt      string  `json:"visitedAt"`
	Age            int     `json:"age"`
	SpecScore      int     `json:"specScore"`
	WaitTimeHours  int     `json:"waitTimeHours"`
	AverageEarning int     `json:"averageEarning"`
	Rating         float64 `json:"rating"`
	CreatedAt      string  `json:"createdAt"`
	HelpfulCount   int     `json:"helpfulCount,omitempty"`
	Excerpt        string  `json:"excerpt,omitempty"`
}

type reviewDetailResponse struct {
	reviewSummaryResponse
	Description       string `json:"description"`
	AuthorDisplayName string `json:"authorDisplayName"`
	AuthorAvatarURL   string `json:"authorAvatarUrl,omitempty"`
}

type reviewListResponse struct {
	Items []reviewSummaryResponse `json:"items"`
	Page  int                     `json:"page"`
	Limit int                     `json:"limit"`
	Total int                     `json:"total"`
}

type adminReviewResponse struct {
	ID             string     `json:"id"`
	StoreID        string     `json:"storeId"`
	StoreName      string     `json:"storeName"`
	BranchName     string     `json:"branchName,omitempty"`
	Prefecture     string     `json:"prefecture"`
	Category       string     `json:"category"`
	VisitedAt      string     `json:"visitedAt"`
	Age            int        `json:"age"`
	SpecScore      int        `json:"specScore"`
	WaitTimeHours  int        `json:"waitTimeHours"`
	AverageEarning int        `json:"averageEarning"`
	Rating         float64    `json:"rating"`
	Status         string     `json:"status"`
	StatusNote     string     `json:"statusNote,omitempty"`
	ReviewedBy     string     `json:"reviewedBy,omitempty"`
	ReviewedAt     *time.Time `json:"reviewedAt,omitempty"`
	Comment        string     `json:"comment,omitempty"`
	RewardStatus   string     `json:"rewardStatus"`
	RewardNote     string     `json:"rewardNote,omitempty"`
	RewardSentAt   *time.Time `json:"rewardSentAt,omitempty"`
	ReviewerID     string     `json:"reviewerId,omitempty"`
	ReviewerName   string     `json:"reviewerName,omitempty"`
	ReviewerHandle string     `json:"reviewerHandle,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type adminReviewListResponse struct {
	Items []adminReviewResponse `json:"items"`
}

type adminStoreResponse struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	BranchName     string     `json:"branchName,omitempty"`
	Prefecture     string     `json:"prefecture,omitempty"`
	IndustryCodes  []string   `json:"industryCodes,omitempty"`
	ReviewCount    int        `json:"reviewCount"`
	LastReviewedAt *time.Time `json:"lastReviewedAt,omitempty"`
}

type createReviewRequest struct {
	StoreName      string  `json:"storeName"`
	BranchName     string  `json:"branchName"`
	Prefecture     string  `json:"prefecture"`
	Category       string  `json:"category,omitempty"`
	VisitedAt      string  `json:"visitedAt"`
	Age            int     `json:"age"`
	SpecScore      int     `json:"specScore"`
	WaitTimeHours  int     `json:"waitTimeHours"`
	AverageEarning int     `json:"averageEarning"`
	Comment        string  `json:"comment"`
	Rating         float64 `json:"rating"`
}

type createReviewResponse struct {
	Status string                `json:"status"`
	Review reviewSummaryResponse `json:"review"`
	Detail reviewDetailResponse  `json:"detail"`
}

const maxReviewRequestBody = 1 << 20

func (req *createReviewRequest) validate() error {
	if strings.TrimSpace(req.StoreName) == "" {
		return errors.New("店舗名は必須です")
	}
	if strings.TrimSpace(req.Prefecture) == "" {
		return errors.New("都道府県は必須です")
	}
	if strings.TrimSpace(req.VisitedAt) == "" {
		return errors.New("働いた時期を指定してください")
	}
	if req.Age < 18 {
		return errors.New("年齢は18歳以上で入力してください")
	}
	if req.Age > 60 {
		req.Age = 60
	}
	if req.SpecScore < 60 {
		return errors.New("スペックは60以上で入力してください")
	}
	if req.SpecScore > 140 {
		req.SpecScore = 140
	}
	if req.WaitTimeHours < 1 {
		return errors.New("待機時間は1時間以上で入力してください")
	}
	if req.WaitTimeHours > 24 {
		req.WaitTimeHours = 24
	}
	if req.AverageEarning < 0 {
		return errors.New("平均稼ぎは0以上で入力してください")
	}
	if req.AverageEarning > 20 {
		req.AverageEarning = 20
	}
	if req.Rating < 0 || req.Rating > 5 {
		return errors.New("総評は0〜5の範囲で入力してください")
	}
	req.Rating = math.Round(req.Rating*2) / 2
	if comment := strings.TrimSpace(req.Comment); len([]rune(comment)) > 2000 {
		return errors.New("感想は2000文字以内で入力してください")
	}
	req.Comment = strings.TrimSpace(req.Comment)
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
	ID                  string  `json:"id"`
	StoreName           string  `json:"storeName"`
	Prefecture          string  `json:"prefecture"`
	Category            string  `json:"category"`
	AverageRating       float64 `json:"averageRating"`
	AverageEarning      int     `json:"averageEarning"`
	AverageEarningLabel string  `json:"averageEarningLabel,omitempty"`
	WaitTimeHours       int     `json:"waitTimeHours"`
	WaitTimeLabel       string  `json:"waitTimeLabel,omitempty"`
	ReviewCount         int     `json:"reviewCount"`
}

type storeListResponse struct {
	Items []storeSummaryResponse `json:"items"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
	Total int                    `json:"total"`
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

func (s *server) notifyReviewReceipt(ctx context.Context, user authenticatedUser, summary reviewSummaryResponse, comment string) {
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

func (s *server) sendMessengerMessage(ctx context.Context, destination, userID, text string) error {
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

	endpoint := s.messengerEndpoint + "/api/messages"
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

func (s *server) sendLineMessage(ctx context.Context, userID, text string) error {
	return s.sendMessengerMessage(ctx, s.messengerDestination, userID, text)
}

func (s *server) sendDiscordMessage(ctx context.Context, userID, text string) error {
	dest := strings.TrimSpace(s.discordDestination)
	if dest == "" {
		return nil
	}
	return s.sendMessengerMessage(ctx, dest, userID, text)
}

func (s *server) reviewCreateHandler() http.HandlerFunc {
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

		now := time.Now().In(s.location)
		category := canonicalIndustryCode(req.Category)
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

		reviewID := primitive.NewObjectID()
		reviewDoc := reviewDocument{
			ID:               reviewID,
			StoreID:          store.ID,
			IndustryCode:     category,
			Status:           "pending",
			Period:           period,
			Age:              intPtr(req.Age),
			SpecScore:        intPtr(req.SpecScore),
			WaitTimeHours:    intPtr(req.WaitTimeHours),
			AverageEarning:   intPtr(req.AverageEarning),
			Rating:           req.Rating,
			Comment:          comment,
			Attachments:      []reviewAttachmentDocument{},
			Reward:           reviewRewardDocument{Status: "pending"},
			ReviewerID:       user.ID,
			ReviewerName:     user.Name,
			ReviewerUsername: user.Username,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		if _, err := s.reviews.InsertOne(ctx, reviewDoc); err != nil {
			s.logger.Printf("レビューの保存に失敗: %v", err)
			http.Error(w, "レビューの保存に失敗しました", http.StatusInternalServerError)
			return
		}

		if category != "" {
			_, err := s.stores.UpdateByID(ctx, store.ID, bson.M{"$addToSet": bson.M{"industryCodes": category}})
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

		summary := s.buildReviewSummary(reviewDoc, store)
		detail := s.buildReviewDetail(reviewDoc, store)
		detail.AuthorDisplayName = reviewerDisplayName(user)
		detail.AuthorAvatarURL = user.Picture

		go s.notifyReviewReceipt(context.Background(), user, summary, comment)

		s.writeJSON(w, http.StatusCreated, createReviewResponse{
			Status: "ok",
			Review: summary,
			Detail: detail,
		})
	}
}

type reviewQueryParams struct {
	Prefecture  string
	Category    string
	CategoryRaw string
	StoreName   string
	Sort        string
	Page        int
	Limit       int
}

func (s *server) collectReviews(ctx context.Context, params reviewQueryParams) ([]reviewSummaryResponse, error) {
	filter := bson.M{
		"status": "approved",
	}
	if params.Category != "" {
		categories := []string{params.Category}
		raw := strings.TrimSpace(params.CategoryRaw)
		if raw != "" && !contains(categories, raw) {
			categories = append(categories, raw)
		}
		filter["industryCode"] = bson.M{"$in": categories}
	}

	if params.Prefecture != "" || params.StoreName != "" {
		storeIDs, err := s.findStoreIDs(ctx, params.Prefecture, params.StoreName)
		if err != nil {
			return nil, err
		}
		if len(storeIDs) == 0 {
			return []reviewSummaryResponse{}, nil
		}
		filter["storeId"] = bson.M{"$in": storeIDs}
	}

	cursor, err := s.reviews.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var reviews []reviewDocument
	storeIDSet := make(map[primitive.ObjectID]struct{})
	for cursor.Next(ctx) {
		var doc reviewDocument
		if err := cursor.Decode(&doc); err != nil {
			s.logger.Printf("レビュー用ドキュメントのデコードに失敗: %v", err)
			continue
		}
		reviews = append(reviews, doc)
		storeIDSet[doc.StoreID] = struct{}{}
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	storeIDs := make([]primitive.ObjectID, 0, len(storeIDSet))
	for id := range storeIDSet {
		storeIDs = append(storeIDs, id)
	}

	storeMap, err := s.loadStoresMap(ctx, storeIDs)
	if err != nil {
		return nil, err
	}

	summaries := make([]reviewSummaryResponse, 0, len(reviews))
	for _, review := range reviews {
		store, ok := storeMap[review.StoreID]
		if !ok {
			continue
		}
		summaries = append(summaries, s.buildReviewSummary(review, store))
	}

	sortReviews(summaries, params.Sort)
	return summaries, nil
}

func (s *server) buildReviewSummary(review reviewDocument, store storeDocument) reviewSummaryResponse {
	category := canonicalIndustryCode(review.IndustryCode)
	if category == "" && len(store.IndustryCodes) > 0 {
		category = store.IndustryCodes[0]
	}

	visitedAt, createdAt := deriveDates(review.Period)
	if !review.CreatedAt.IsZero() {
		createdAt = review.CreatedAt.Format(time.RFC3339)
	}

	spec := intPtrValue(review.SpecScore)
	wait := intPtrValue(review.WaitTimeHours)
	earning := intPtrValue(review.AverageEarning)
	helpful := deriveHelpfulCount(review.ID, spec)

	waitLabel := ""
	if wait > 0 {
		waitLabel = formatWaitTimeLabel(wait)
	}
	earningLabel := ""
	if earning > 0 {
		earningLabel = formatAverageEarningLabel(earning)
	}

	excerpt := buildExcerpt(review.Comment, store.Name, earningLabel, waitLabel)

	return reviewSummaryResponse{
		ID:             review.ID.Hex(),
		StoreID:        review.StoreID.Hex(),
		StoreName:      store.Name,
		BranchName:     strings.TrimSpace(store.BranchName),
		Prefecture:     store.Prefecture,
		Category:       category,
		VisitedAt:      visitedAt,
		Age:            intPtrValue(review.Age),
		SpecScore:      spec,
		WaitTimeHours:  wait,
		AverageEarning: earning,
		Rating:         review.Rating,
		CreatedAt:      createdAt,
		HelpfulCount:   helpful,
		Excerpt:        excerpt,
	}
}

func (s *server) buildReviewDetail(review reviewDocument, store storeDocument) reviewDetailResponse {
	summary := s.buildReviewSummary(review, store)
	description := strings.TrimSpace(review.Comment)
	if description == "" {
		description = buildFallbackDescription(summary)
	}
	displayName := strings.TrimSpace(review.ReviewerName)
	if displayName == "" {
		displayName = "匿名店舗アンケート"
	}
	detail := reviewDetailResponse{
		reviewSummaryResponse: summary,
		Description:           description,
		AuthorDisplayName:     displayName,
	}
	return detail
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

func buildAdminReviewResponse(review reviewDocument, store storeDocument) adminReviewResponse {
	category := canonicalIndustryCode(review.IndustryCode)
	if category == "" && len(store.IndustryCodes) > 0 {
		category = store.IndustryCodes[0]
	}
	if category == "" {
		category = "デリヘル"
	}

	visitedAt, _ := deriveDates(review.Period)

	status := strings.TrimSpace(review.Status)
	if status == "" {
		status = "pending"
	}

	rewardStatus := strings.TrimSpace(review.Reward.Status)
	if rewardStatus == "" {
		rewardStatus = "pending"
	}

	return adminReviewResponse{
		ID:             review.ID.Hex(),
		StoreID:        review.StoreID.Hex(),
		StoreName:      store.Name,
		BranchName:     strings.TrimSpace(store.BranchName),
		Prefecture:     store.Prefecture,
		Category:       category,
		VisitedAt:      visitedAt,
		Age:            intPtrValue(review.Age),
		SpecScore:      intPtrValue(review.SpecScore),
		WaitTimeHours:  intPtrValue(review.WaitTimeHours),
		AverageEarning: intPtrValue(review.AverageEarning),
		Rating:         review.Rating,
		Status:         status,
		StatusNote:     strings.TrimSpace(review.StatusNote),
		ReviewedBy:     strings.TrimSpace(review.ReviewedBy),
		ReviewedAt:     review.ReviewedAt,
		Comment:        strings.TrimSpace(review.Comment),
		RewardStatus:   rewardStatus,
		RewardNote:     strings.TrimSpace(review.Reward.Note),
		RewardSentAt:   review.Reward.SentAt,
		ReviewerID:     strings.TrimSpace(review.ReviewerID),
		ReviewerName:   strings.TrimSpace(review.ReviewerName),
		ReviewerHandle: strings.TrimSpace(review.ReviewerUsername),
		CreatedAt:      review.CreatedAt,
		UpdatedAt:      review.UpdatedAt,
	}
}

func deriveHelpfulCount(id primitive.ObjectID, spec int) int {
	base := int(id.Timestamp().Unix()%10) + spec
	if base < 5 {
		base = 5
	}
	return base % 40
}

type updateReviewStatusRequest struct {
	Status       string `json:"status"`
	StatusNote   string `json:"statusNote"`
	ReviewedBy   string `json:"reviewedBy"`
	RewardStatus string `json:"rewardStatus"`
	RewardNote   string `json:"rewardNote"`
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
}

func (s *server) adminReviewListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		filter := bson.M{}
		if status != "" && status != "all" {
			filter["status"] = status
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

		var reviews []reviewDocument
		storeIDSet := make(map[primitive.ObjectID]struct{})
		for cursor.Next(ctx) {
			var doc reviewDocument
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

		s.logger.Printf("admin review list: status=%q count=%d", status, len(items))
		s.writeJSON(w, http.StatusOK, adminReviewListResponse{Items: items})
	}
}

func (s *server) adminReviewDetailHandler() http.HandlerFunc {
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

		var review reviewDocument
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

		s.logger.Printf("admin review detail success id=%q status=%q rewardStatus=%q", idParam, strings.TrimSpace(review.Status), strings.TrimSpace(review.Reward.Status))

		s.writeJSON(w, http.StatusOK, buildAdminReviewResponse(review, store))
	}
}

func (s *server) adminReviewUpdateHandler() http.HandlerFunc {
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

		var existing reviewDocument
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
			if _, err := s.stores.UpdateByID(ctx, targetStoreID, bson.M{"$addToSet": bson.M{"industryCodes": addIndustry}}); err != nil {
				s.logger.Printf("admin review content update industry append failed id=%q storeId=%s err=%v", idParam, targetStoreID.Hex(), err)
			}
		}

		var updated reviewDocument
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

func (s *server) adminReviewStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		s.logger.Printf("admin review status update request id=%q", idParam)
		objectID, err := primitive.ObjectIDFromHex(idParam)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "レビューIDの形式が不正です"})
			return
		}

		var req updateReviewStatusRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, maxReviewRequestBody)).Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "リクエストの形式が不正です"})
			return
		}

		update := bson.M{}
		now := time.Now().In(s.location)

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if status := strings.TrimSpace(req.Status); status != "" {
			update["status"] = status
			update["statusNote"] = strings.TrimSpace(req.StatusNote)
			update["reviewedBy"] = strings.TrimSpace(req.ReviewedBy)
			if status == "approved" || status == "rejected" {
				update["reviewedAt"] = now
			} else {
				update["reviewedAt"] = nil
			}
		}

		if reward := strings.TrimSpace(req.RewardStatus); reward != "" {
			update["reward.status"] = reward
			update["reward.note"] = strings.TrimSpace(req.RewardNote)
			if reward == "sent" {
				update["reward.sentAt"] = now
			} else {
				update["reward.sentAt"] = nil
			}
		}

		if len(update) == 0 {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "更新内容が指定されていません"})
			return
		}

		update["updatedAt"] = now

		result := s.reviews.FindOneAndUpdate(ctx, bson.M{"_id": objectID}, bson.M{"$set": update}, options.FindOneAndUpdate().SetReturnDocument(options.After))
		var updated reviewDocument
		if err := result.Decode(&updated); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				s.logger.Printf("admin review status update not found id=%q", idParam)
				s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "レビューが見つかりません"})
				return
			}
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "レビューの更新に失敗しました"})
			return
		}

		if err := s.recalculateStoreStats(ctx, updated.StoreID); err != nil {
			s.logger.Printf("admin review status update stats recalculation failed id=%q err=%v", idParam, err)
		}

		store, err := s.getStoreByID(ctx, updated.StoreID)
		if err != nil {
			s.logger.Printf("admin review status update store fetch failed id=%q storeId=%s err=%v", idParam, updated.StoreID.Hex(), err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		s.logger.Printf("admin review status update success id=%q status=%q rewardStatus=%q", idParam, strings.TrimSpace(updated.Status), strings.TrimSpace(updated.Reward.Status))

		s.writeJSON(w, http.StatusOK, buildAdminReviewResponse(updated, store))
	}
}

func (s *server) adminStoreSearchHandler() http.HandlerFunc {
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

		filters := []bson.M{
			{"stats.reviewCount": bson.M{"$gt": 0}},
		}
		if prefecture != "" {
			filters = append(filters, bson.M{"prefecture": prefecture})
		}
		if industry != "" {
			filters = append(filters, bson.M{"industryCodes": industry})
		}
		if keyword != "" {
			pattern := regexp.QuoteMeta(keyword)
			regex := primitive.Regex{Pattern: pattern, Options: "i"}
			filters = append(filters, bson.M{
				"$or": bson.A{
					bson.M{"name": regex},
					bson.M{"branchName": regex},
				},
			})
		}

		filter := bson.M{}
		if len(filters) == 1 {
			filter = filters[0]
		} else {
			filter["$and"] = filters
		}

		opts := options.Find().
			SetSort(bson.D{{Key: "stats.reviewCount", Value: -1}, {Key: "name", Value: 1}}).
			SetLimit(int64(limit))

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		cursor, err := s.stores.Find(ctx, filter, opts)
		if err != nil {
			s.logger.Printf("admin store search find failed: %v", err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗候補の取得に失敗しました"})
			return
		}
		defer cursor.Close(ctx)

		items := make([]adminStoreResponse, 0)
		for cursor.Next(ctx) {
			var doc storeDocument
			if err := cursor.Decode(&doc); err != nil {
				s.logger.Printf("admin store search decode failed: %v", err)
				continue
			}
			items = append(items, storeDocumentToAdminResponse(doc))
		}
		if err := cursor.Err(); err != nil {
			s.logger.Printf("admin store search cursor err: %v", err)
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗候補の取得に失敗しました"})
			return
		}

		s.writeJSON(w, http.StatusOK, map[string]any{
			"items": items,
		})
	}
}

type adminStoreCreateRequest struct {
	Name         string `json:"name"`
	BranchName   string `json:"branchName"`
	Prefecture   string `json:"prefecture"`
	IndustryCode string `json:"industryCode"`
}

type adminStoreCreateResponse struct {
	Store   adminStoreResponse `json:"store"`
	Created bool               `json:"created"`
}

func (s *server) adminStoreCreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req adminStoreCreateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, maxReviewRequestBody)).Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "リクエストの形式が不正です"})
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "店舗名は必須です"})
			return
		}
		prefecture := strings.TrimSpace(req.Prefecture)
		if prefecture == "" {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "都道府県は必須です"})
			return
		}
		industry := strings.TrimSpace(req.IndustryCode)
		if industry == "" {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "業種コードは必須です"})
			return
		}
		branch := strings.TrimSpace(req.BranchName)

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		filter := bson.M{"name": name}
		if branch != "" {
			filter["branchName"] = branch
		}
		if prefecture != "" {
			filter["prefecture"] = prefecture
		}

		created := false
		var store storeDocument
		err := s.stores.FindOne(ctx, filter).Decode(&store)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				created = true
				store, err = s.findOrCreateStore(ctx, name, branch, prefecture, industry)
				if err != nil {
					s.logger.Printf("admin store create insert failed: %v", err)
					s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗の作成に失敗しました"})
					return
				}
			} else {
				s.logger.Printf("admin store create find failed: %v", err)
				s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
				return
			}
		}

		if !containsString(store.IndustryCodes, industry) {
			update := bson.M{
				"$addToSet": bson.M{"industryCodes": industry},
				"$set":      bson.M{"updatedAt": time.Now().In(s.location)},
			}
			if _, err := s.stores.UpdateByID(ctx, store.ID, update); err != nil {
				s.logger.Printf("admin store create add industry failed id=%s err=%v", store.ID.Hex(), err)
			} else {
				store, _ = s.getStoreByID(ctx, store.ID)
			}
		}

		response := adminStoreCreateResponse{
			Store:   storeDocumentToAdminResponse(store),
			Created: created,
		}

		s.writeJSON(w, http.StatusCreated, response)
	}
}

func storeDocumentToAdminResponse(doc storeDocument) adminStoreResponse {
	return adminStoreResponse{
		ID:             doc.ID.Hex(),
		Name:           doc.Name,
		BranchName:     strings.TrimSpace(doc.BranchName),
		Prefecture:     doc.Prefecture,
		IndustryCodes:  append([]string(nil), doc.IndustryCodes...),
		ReviewCount:    doc.Stats.ReviewCount,
		LastReviewedAt: doc.Stats.LastReviewedAt,
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
func (s *server) reviewListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := r.URL.Query()
		categoryRaw := strings.TrimSpace(query.Get("category"))
		params := reviewQueryParams{
			Prefecture:  strings.TrimSpace(query.Get("prefecture")),
			Category:    canonicalIndustryCode(categoryRaw),
			CategoryRaw: categoryRaw,
			StoreName:   strings.TrimSpace(query.Get("storeName")),
			Sort:        strings.TrimSpace(query.Get("sort")),
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

func (s *server) reviewLatestHandler() http.HandlerFunc {
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

func (s *server) reviewHighRatedHandler() http.HandlerFunc {
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

func (s *server) reviewDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	idParam := chi.URLParam(r, "id")
	if idParam == "" {
		http.Error(w, "IDが指定されていません", http.StatusBadRequest)
		return
	}

	objectID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		http.Error(w, "不正なIDです", http.StatusBadRequest)
		return
	}

	var review reviewDocument
	if err := s.reviews.FindOne(ctx, bson.M{"_id": objectID}).Decode(&review); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			http.NotFound(w, r)
			return
		}
		s.logger.Printf("レビュー詳細の取得に失敗: %v", err)
		http.Error(w, "レビュー詳細の取得に失敗しました", http.StatusInternalServerError)
		return
	}

	store, err := s.getStoreByID(ctx, review.StoreID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			http.NotFound(w, r)
			return
		}
		s.logger.Printf("店舗情報の取得に失敗: %v", err)
		http.Error(w, "店舗情報の取得に失敗しました", http.StatusInternalServerError)
		return
	}

	detail := s.buildReviewDetail(review, store)
	s.writeJSON(w, http.StatusOK, detail)
}
