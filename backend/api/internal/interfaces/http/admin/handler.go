package admin

import (
	"log"

	"github.com/go-chi/chi/v5"
	adminapp "github.com/sngm3741/makoto-club-services/api/internal/admin/application"
)

// Handler は Admin コンテキストの HTTP エンドポイントをアプリケーションサービスへ接続する。
type Handler struct {
	logger        *log.Logger
	storeService  adminapp.StoreService
	surveyService adminapp.SurveyService
}

// Config は Admin Handler を構築するための依存関係をまとめた値オブジェクト。
type Config struct {
	Logger        *log.Logger
	StoreService  adminapp.StoreService
	SurveyService adminapp.SurveyService
}

// NewHandler は Admin 用の Handler を生成する。
func NewHandler(cfg Config) *Handler {
	return &Handler{
		logger:        cfg.Logger,
		storeService:  cfg.StoreService,
		surveyService: cfg.SurveyService,
	}
}

// Register は Admin 向けルートを chi.Router に登録する。
func (h *Handler) Register(r chi.Router) {
	r.Get("/reviews", h.reviewListHandler())
	r.Get("/reviews/{id}", h.reviewDetailHandler())
	r.Patch("/reviews/{id}", h.reviewUpdateHandler())
	r.Get("/stores", h.storeSearchHandler())
	r.Get("/stores/{id}", h.storeDetailHandler())
	r.Post("/stores", h.storeCreateHandler())
	r.Post("/stores/{id}/reviews", h.storeReviewCreateHandler())
}
