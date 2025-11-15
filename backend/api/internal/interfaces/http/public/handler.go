package public

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	publicapp "github.com/sngm3741/makoto-club-services/api/internal/public/application"
	"go.mongodb.org/mongo-driver/mongo"
)

// Handler は Public コンテキストの HTTP エンドポイントをアプリケーションサービスへ接続する。
// DDD の Interface 層として、外部プロトコルとユースケースを橋渡しする責務を担う。
type Handler struct {
	logger               *log.Logger
	storeQueries         publicapp.StoreQueryService
	surveyQueries        publicapp.SurveyQueryService
	surveyCommands       publicapp.SurveyCommandService
	stores               *mongo.Collection
	reviews              *mongo.Collection
	location             *time.Location
	helpfulCookieSecret  []byte
	helpfulCookieSecure  bool
	httpClient           *http.Client
	messengerEndpoint    string
	messengerDestination string
	discordDestination   string
	slackDestination     string
	adminReviewBaseURL   string
	failedNotifications  *mongo.Collection
}

// Config は Handler を構築するための依存関係をまとめた値オブジェクト。
type Config struct {
	Logger               *log.Logger
	StoreQueries         publicapp.StoreQueryService
	SurveyQueries        publicapp.SurveyQueryService
	SurveyCommands       publicapp.SurveyCommandService
	Stores               *mongo.Collection
	Reviews              *mongo.Collection
	Location             *time.Location
	HelpfulCookieSecret  []byte
	HelpfulCookieSecure  bool
	HTTPClient           *http.Client
	MessengerEndpoint    string
	MessengerDestination string
	DiscordDestination   string
	SlackDestination     string
	FailedNotifications  *mongo.Collection
	AdminReviewBaseURL   string
}

// NewHandler は Public コンテキスト用の Handler を生成し、必要な依存を保持させる。
func NewHandler(cfg Config) *Handler {
	return &Handler{
		logger:               cfg.Logger,
		storeQueries:         cfg.StoreQueries,
		surveyQueries:        cfg.SurveyQueries,
		surveyCommands:       cfg.SurveyCommands,
		stores:               cfg.Stores,
		reviews:              cfg.Reviews,
		location:             cfg.Location,
		helpfulCookieSecret:  cfg.HelpfulCookieSecret,
		helpfulCookieSecure:  cfg.HelpfulCookieSecure,
		httpClient:           cfg.HTTPClient,
		messengerEndpoint:    cfg.MessengerEndpoint,
		messengerDestination: cfg.MessengerDestination,
		discordDestination:   cfg.DiscordDestination,
		slackDestination:     cfg.SlackDestination,
		adminReviewBaseURL:   cfg.AdminReviewBaseURL,
		failedNotifications:  cfg.FailedNotifications,
	}
}

// Register は Public 向けのルートを chi.Router へ登録し、認証ミドルウェアなどを組み合わせる。
func (h *Handler) Register(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	r.Get("/stores", h.storeListHandler())
	r.Get("/stores/{id}", h.storeDetailHandler())
	r.Get("/reviews", h.reviewListHandler())
	r.Get("/reviews/new", h.reviewLatestHandler())
	r.Get("/reviews/high-rated", h.reviewHighRatedHandler())
	r.Get("/reviews/{id}", h.reviewDetailHandler())
	r.Post("/reviews/{id}/helpful", h.reviewHelpfulToggleHandler())
	r.With(authMiddleware).Post("/reviews", h.reviewCreateHandler())
	r.With(authMiddleware).Get("/auth/verify", h.authVerifyHandler())
}
