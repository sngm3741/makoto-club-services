package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type config struct {
	addr             string
	mongoURI         string
	mongoDatabase    string
	pingCollection   string
	surveyCollection string
	timeout          time.Duration
	timezone         string
	serverLog        *log.Logger
}

type server struct {
	logger   *log.Logger
	client   *mongo.Client
	database *mongo.Database
	pings    *mongo.Collection
	surveys  *mongo.Collection
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
	router.Get("/api/stores", srv.storeListHandler())
	router.Get("/api/reviews", srv.reviewListHandler())
	router.Get("/api/reviews/new", srv.reviewLatestHandler())
	router.Get("/api/reviews/high-rated", srv.reviewHighRatedHandler())
	router.Get("/api/reviews/{id}", srv.reviewDetailHandler)

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
		addr:             envOrDefault("HTTP_ADDR", ":8080"),
		mongoURI:         envOrDefault("MONGO_URI", "mongodb://mongo:27017"),
		mongoDatabase:    envOrDefault("MONGO_DB", "makoto-club"),
		surveyCollection: envOrDefault("SURVEY_COLLECTION", "tokumei-tenpo-ankeet"),
		pingCollection:   envOrDefault("PING_COLLECTION", "pings"),
		timeout:          timeout,
		timezone:         envOrDefault("TIMEZONE", "Asia/Tokyo"),
		serverLog:        log.New(os.Stdout, "[makoto-club-api] ", log.LstdFlags|log.Lshortfile),
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
	srv.surveys = srv.database.Collection(cfg.surveyCollection)
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

func (s *server) storeListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := r.URL.Query()
		prefectureFilter := strings.TrimSpace(query.Get("prefecture"))
		categoryFilter := strings.TrimSpace(query.Get("category"))
		avgEarningFilter, hasAvgFilter := parseInt(query.Get("avgEarning"))

		page, _ := parsePositiveInt(query.Get("page"), 1)
		limit, _ := parsePositiveInt(query.Get("limit"), 10)
		if limit <= 0 {
			limit = 10
		}

		filter := bson.M{}
		if prefectureFilter != "" {
			filter["prefecture"] = prefectureFilter
		}

		cursor, err := s.surveys.Find(ctx, filter)
		if err != nil {
			s.logger.Printf("店舗アンケートの取得に失敗: %v", err)
			http.Error(w, "アンケートデータの取得に失敗しました", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		aggregated := map[string]*storeAggregate{}

		for cursor.Next(ctx) {
			var doc surveyDocument
			if err := cursor.Decode(&doc); err != nil {
				s.logger.Printf("アンケートドキュメントのデコードに失敗: %v", err)
				continue
			}

			item := aggregated[doc.StoreName]
			if item == nil {
				item = &storeAggregate{
					storeName:  doc.StoreName,
					prefecture: doc.Prefecture,
					category:   determineCategory(doc.StoreName),
				}
				aggregated[doc.StoreName] = item
			}

			item.reviewCount++

			if doc.Prefecture != "" {
				item.prefecture = doc.Prefecture
			}

			if doc.AverageEarning != "" {
				if value, ok := parseFirstNumber(doc.AverageEarning); ok {
					item.averageEarningSum += value
					item.averageEarningCount++
				}
				item.averageEarningLabel = doc.AverageEarning
			}

			if doc.WaitTime != "" {
				if value, ok := parseFirstNumber(doc.WaitTime); ok {
					item.waitTimeSum += value
					item.waitTimeCount++
				}
				item.waitTimeLabel = doc.WaitTime
			}
		}

		if err := cursor.Err(); err != nil {
			s.logger.Printf("アンケートカーソル処理中にエラー: %v", err)
			http.Error(w, "アンケートデータの処理に失敗しました", http.StatusInternalServerError)
			return
		}

		summaries := make([]storeSummaryResponse, 0, len(aggregated))
		for _, agg := range aggregated {
			averageEarning := 0
			if agg.averageEarningCount > 0 {
				averageEarning = int(math.Round(agg.averageEarningSum / float64(agg.averageEarningCount)))
			}

			waitTime := 0
			if agg.waitTimeCount > 0 {
				waitTime = int(math.Round(agg.waitTimeSum / float64(agg.waitTimeCount)))
			}

			summary := storeSummaryResponse{
				ID:                  agg.id(),
				StoreName:           agg.storeName,
				Prefecture:          agg.prefecture,
				Category:            agg.category,
				AverageEarning:      averageEarning,
				AverageEarningLabel: agg.averageEarningLabel,
				WaitTimeHours:       waitTime,
				WaitTimeLabel:       agg.waitTimeLabel,
				ReviewCount:         agg.reviewCount,
			}
			summaries = append(summaries, summary)
		}

		filtered := summaries[:0]
		for _, summary := range summaries {
			if categoryFilter != "" && summary.Category != categoryFilter {
				continue
			}
			if hasAvgFilter && summary.AverageEarning != avgEarningFilter {
				continue
			}
			filtered = append(filtered, summary)
		}
		summaries = filtered

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

		response := storeListResponse{
			Items: summaries[start:end],
			Page:  page,
			Limit: limit,
			Total: total,
		}

		s.writeJSON(w, http.StatusOK, response)
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

type surveyDocument struct {
	ID             primitive.ObjectID `bson:"_id"`
	StoreName      string             `bson:"storeName"`
	Prefecture     string             `bson:"prefecture"`
	Period         string             `bson:"period"`
	Age            any                `bson:"age"`
	Spec           any                `bson:"spec"`
	WaitTime       string             `bson:"waitTime"`
	AverageEarning string             `bson:"averageEarning"`
}

type reviewSummaryResponse struct {
	ID             string `json:"id"`
	StoreName      string `json:"storeName"`
	Prefecture     string `json:"prefecture"`
	Category       string `json:"category"`
	VisitedAt      string `json:"visitedAt"`
	Age            int    `json:"age"`
	SpecScore      int    `json:"specScore"`
	WaitTimeHours  int    `json:"waitTimeHours"`
	AverageEarning int    `json:"averageEarning"`
	CreatedAt      string `json:"createdAt"`
	HelpfulCount   int    `json:"helpfulCount,omitempty"`
	Excerpt        string `json:"excerpt,omitempty"`
}

type reviewDetailResponse struct {
	reviewSummaryResponse
	Description        string `json:"description"`
	AuthorDisplayName  string `json:"authorDisplayName"`
	AuthorAvatarURL    string `json:"authorAvatarUrl,omitempty"`
}

type reviewListResponse struct {
	Items []reviewSummaryResponse `json:"items"`
	Page  int                     `json:"page"`
	Limit int                     `json:"limit"`
	Total int                     `json:"total"`
}

type storeAggregate struct {
	storeName           string
	prefecture          string
	category            string
	averageEarningSum   float64
	averageEarningCount int
	averageEarningLabel string
	waitTimeSum         float64
	waitTimeCount       int
	waitTimeLabel       string
	reviewCount         int
}

func (a *storeAggregate) id() string {
	return fmt.Sprintf("%s-%s", a.prefecture, a.storeName)
}

type storeSummaryResponse struct {
	ID                  string `json:"id"`
	StoreName           string `json:"storeName"`
	Prefecture          string `json:"prefecture"`
	Category            string `json:"category"`
	AverageEarning      int    `json:"averageEarning"`
	AverageEarningLabel string `json:"averageEarningLabel,omitempty"`
	WaitTimeHours       int    `json:"waitTimeHours"`
	WaitTimeLabel       string `json:"waitTimeLabel,omitempty"`
	ReviewCount         int    `json:"reviewCount"`
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

var storeCategoryMap = map[string]string{
	"恋するバニー":     "delivery_health",
	"シンデレラ":      "delivery_health",
	"ジュエル":       "delivery_health",
	"プリンセスリング":   "soap",
	"トワイライトガーデン": "store_health",
	"シュガーガール":    "pinsaro",
	"ミルキームーン":    "dc",
	"ハートフルルーム":   "store_health",
	"ドリームスパ":     "store_health",
	"ブルーミスト":     "store_health",
	"kazusa素人学園": "delivery_health",
	"アンドエッセンス":   "delivery_health",
	"ハピネス本店":     "store_health",
	"ルミエール":      "store_health",
	"ネクストステージ":   "delivery_health",
}

func determineCategory(storeName string) string {
	if category, ok := storeCategoryMap[storeName]; ok {
		return category
	}
	return "delivery_health"
}

type reviewQueryParams struct {
	Prefecture    string
	Category      string
	StoreName     string
	Sort          string
	AvgEarning    int
	HasAvgEarning bool
	Page          int
	Limit         int
}

func (s *server) collectReviews(ctx context.Context, params reviewQueryParams) ([]reviewSummaryResponse, error) {
	filter := bson.M{}
	if params.Prefecture != "" {
		filter["prefecture"] = params.Prefecture
	}
	if params.StoreName != "" {
		filter["storeName"] = bson.M{"$regex": params.StoreName}
	}

	cursor, err := s.surveys.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var summaries []reviewSummaryResponse
	for cursor.Next(ctx) {
		var doc surveyDocument
		if err := cursor.Decode(&doc); err != nil {
			s.logger.Printf("レビュー用ドキュメントのデコードに失敗: %v", err)
			continue
		}

		summary := buildReviewSummary(doc)

		if params.Category != "" && summary.Category != params.Category {
			continue
		}
		if params.HasAvgEarning && summary.AverageEarning != params.AvgEarning {
			continue
		}

		summaries = append(summaries, summary)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	sortReviews(summaries, params.Sort)
	return summaries, nil
}

func buildReviewSummary(doc surveyDocument) reviewSummaryResponse {
	category := determineCategory(doc.StoreName)
	averageEarning := extractFirstInt(doc.AverageEarning)
	waitTime := extractFirstInt(doc.WaitTime)
	age := extractFirstInt(doc.Age)
	spec := extractFirstInt(doc.Spec)

	visitedAt, createdAt := deriveDates(doc.Period)
	helpful := deriveHelpfulCount(doc.ID, spec)
	excerpt := buildExcerpt(doc.StoreName, doc.AverageEarning, doc.WaitTime)

	return reviewSummaryResponse{
		ID:             doc.ID.Hex(),
		StoreName:      doc.StoreName,
		Prefecture:     doc.Prefecture,
		Category:       category,
		VisitedAt:      visitedAt,
		Age:            age,
		SpecScore:      spec,
		WaitTimeHours:  waitTime,
		AverageEarning: averageEarning,
		CreatedAt:      createdAt,
		HelpfulCount:   helpful,
		Excerpt:        excerpt,
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

func deriveHelpfulCount(id primitive.ObjectID, spec int) int {
	base := int(id.Timestamp().Unix()%10) + spec
	if base < 5 {
		base = 5
	}
	return base % 40
}

func extractFirstInt(value any) int {
	switch v := value.(type) {
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

func buildExcerpt(storeName, earningLabel, waitTimeLabel string) string {
	components := []string{}
	if earningLabel != "" {
		components = append(components, fmt.Sprintf("平均稼ぎは%s万円", earningsDisplay(earningLabel)))
	}
	if waitTimeLabel != "" {
		components = append(components, fmt.Sprintf("待機は%s", waitTimeLabel))
	}
	if len(components) == 0 {
		return fmt.Sprintf("%sの最新アンケートです。", storeName)
	}
	return strings.Join(components, "／")
}

func buildDescription(summary reviewSummaryResponse) string {
	return fmt.Sprintf(
		"%sでの体験談です。平均稼ぎはおよそ%d万円、待機時間は%d時間程度でした。年代: %d歳、スペック: %d を参考にしてください。",
		summary.StoreName,
		summary.AverageEarning,
		summary.WaitTimeHours,
		summary.Age,
		summary.SpecScore,
	)
}

func earningsDisplay(label string) string {
	match := numberPattern.FindString(label)
	if match == "" {
		return label
	}
	return match
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
		params := reviewQueryParams{
			Prefecture: strings.TrimSpace(query.Get("prefecture")),
			Category:   strings.TrimSpace(query.Get("category")),
			StoreName:  strings.TrimSpace(query.Get("storeName")),
			Sort:       strings.TrimSpace(query.Get("sort")),
		}
		params.AvgEarning, params.HasAvgEarning = parseInt(query.Get("avgEarning"))
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

		reviews, err := s.collectReviews(ctx, reviewQueryParams{Sort: "newest"})
		if err != nil {
			s.logger.Printf("最新レビューの取得に失敗: %v", err)
			http.Error(w, "最新レビューの取得に失敗しました", http.StatusInternalServerError)
			return
		}
		if len(reviews) > 3 {
			reviews = reviews[:3]
		}
		s.writeJSON(w, http.StatusOK, reviews)
	}
}

func (s *server) reviewHighRatedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		reviews, err := s.collectReviews(ctx, reviewQueryParams{Sort: "helpful"})
		if err != nil {
			s.logger.Printf("高評価レビューの取得に失敗: %v", err)
			http.Error(w, "高評価レビューの取得に失敗しました", http.StatusInternalServerError)
			return
		}
		if len(reviews) > 3 {
			reviews = reviews[:3]
		}
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

	var doc surveyDocument
	if err := s.surveys.FindOne(ctx, bson.M{"_id": objectID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			http.NotFound(w, r)
			return
		}
		s.logger.Printf("レビュー詳細の取得に失敗: %v", err)
		http.Error(w, "レビュー詳細の取得に失敗しました", http.StatusInternalServerError)
		return
	}

	summary := buildReviewSummary(doc)
	detail := reviewDetailResponse{
		reviewSummaryResponse: summary,
		Description:           buildDescription(summary),
		AuthorDisplayName:     "匿名店舗アンケート",
	}
	s.writeJSON(w, http.StatusOK, detail)
}
