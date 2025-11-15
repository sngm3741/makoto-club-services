package public

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	publicapp "github.com/sngm3741/makoto-club-services/api/internal/public/application"
	"go.mongodb.org/mongo-driver/mongo"
)

// Handler wires public HTTP endpoints to application services.
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
	adminReviewBaseURL   string
}

// Config defines dependencies required by Handler.
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
	AdminReviewBaseURL   string
}

// NewHandler constructs a public HTTP handler set.
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
		adminReviewBaseURL:   cfg.AdminReviewBaseURL,
	}
}

// Register mounts all public routes onto the router.
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
