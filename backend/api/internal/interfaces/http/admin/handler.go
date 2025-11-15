package admin

import (
	"log"

	"github.com/go-chi/chi/v5"
	adminapp "github.com/sngm3741/makoto-club-services/api/internal/admin/application"
)

// Handler wires admin HTTP endpoints to application services.
type Handler struct {
	logger        *log.Logger
	storeService  adminapp.StoreService
	surveyService adminapp.SurveyService
}

// Config provides dependencies for Handler.
type Config struct {
	Logger        *log.Logger
	StoreService  adminapp.StoreService
	SurveyService adminapp.SurveyService
}

// NewHandler constructs an admin HTTP handler set.
func NewHandler(cfg Config) *Handler {
	return &Handler{
		logger:        cfg.Logger,
		storeService:  cfg.StoreService,
		surveyService: cfg.SurveyService,
	}
}

// Register mounts admin routes onto router.
func (h *Handler) Register(r chi.Router) {
	r.Get("/reviews", h.reviewListHandler())
	r.Get("/reviews/{id}", h.reviewDetailHandler())
	r.Patch("/reviews/{id}", h.reviewUpdateHandler())
	r.Get("/stores", h.storeSearchHandler())
	r.Get("/stores/{id}", h.storeDetailHandler())
	r.Post("/stores", h.storeCreateHandler())
	r.Post("/stores/{id}/reviews", h.storeReviewCreateHandler())
}
