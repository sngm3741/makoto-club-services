package admin

import (
	"log"
	"time"

	"github.com/go-chi/chi/v5"
	adminapp "github.com/sngm3741/makoto-club-services/api/internal/admin/application"
	"go.mongodb.org/mongo-driver/mongo"
)

// Handler wires admin HTTP endpoints to application services.
type Handler struct {
	logger       *log.Logger
	storeService adminapp.StoreService
	stores       *mongo.Collection
	reviews      *mongo.Collection
	location     *time.Location
}

// Config provides dependencies for Handler.
type Config struct {
	Logger       *log.Logger
	StoreService adminapp.StoreService
	Stores       *mongo.Collection
	Reviews      *mongo.Collection
	Location     *time.Location
}

// NewHandler constructs an admin HTTP handler set.
func NewHandler(cfg Config) *Handler {
	return &Handler{
		logger:       cfg.Logger,
		storeService: cfg.StoreService,
		stores:       cfg.Stores,
		reviews:      cfg.Reviews,
		location:     cfg.Location,
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
