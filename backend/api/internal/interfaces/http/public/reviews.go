package public

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
	publicapp "github.com/sngm3741/makoto-club-services/api/internal/public/application"
)

type reviewQueryParams struct {
	Prefecture string
	Category   string
	StoreName  string
	StoreID    primitive.ObjectID
	Sort       string
	Page       int
	Limit      int
}

// reviewListHandler はユーザー向けのアンケート一覧 API。
// DDD では Query Service を介して読み取り専用ユースケースを実現する。
func (h *Handler) reviewListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := r.URL.Query()
		params := reviewQueryParams{
			Prefecture: strings.TrimSpace(query.Get("prefecture")),
			Category:   common.CanonicalIndustryCode(query.Get("category")),
			StoreName:  strings.TrimSpace(query.Get("storeName")),
			Sort:       strings.TrimSpace(query.Get("sort")),
		}

		if storeID := strings.TrimSpace(query.Get("storeId")); storeID != "" {
			if parsed, err := primitive.ObjectIDFromHex(storeID); err == nil {
				params.StoreID = parsed
			} else {
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "店舗IDの形式が不正です"})
				return
			}
		}

		params.Page, _ = common.ParsePositiveInt(query.Get("page"), 1)
		params.Limit, _ = common.ParsePositiveInt(query.Get("limit"), 10)

		reviews, err := h.collectReviews(ctx, params)
		if err != nil {
			h.logger.Printf("review list fetch failed: %v", err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "アンケートの取得に失敗しました"})
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

		common.WriteJSON(h.logger, w, http.StatusOK, reviewListResponse{
			Items: reviews[start:end],
			Page:  params.Page,
			Limit: params.Limit,
			Total: total,
		})
	}
}

// reviewLatestHandler は最新アンケートを上限3件まで返す。
func (h *Handler) reviewLatestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		reviews, err := h.collectReviews(ctx, reviewQueryParams{Sort: "newest", Limit: 3})
		if err != nil {
			h.logger.Printf("最新レビューの取得に失敗: %v", err)
			http.Error(w, "最新レビューの取得に失敗しました", http.StatusInternalServerError)
			return
		}
		if len(reviews) > 3 {
			reviews = reviews[:3]
		}
		if reviews == nil {
			reviews = []reviewSummaryResponse{}
		}
		h.logger.Printf("review latest list count=%d", len(reviews))
		common.WriteJSON(h.logger, w, http.StatusOK, reviews)
	}
}

// reviewHighRatedHandler は高評価順のアンケートを上限3件まで返す。
func (h *Handler) reviewHighRatedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		reviews, err := h.collectReviews(ctx, reviewQueryParams{Sort: "helpful", Limit: 3})
		if err != nil {
			h.logger.Printf("高評価レビューの取得に失敗: %v", err)
			http.Error(w, "高評価レビューの取得に失敗しました", http.StatusInternalServerError)
			return
		}
		if len(reviews) > 3 {
			reviews = reviews[:3]
		}
		if reviews == nil {
			reviews = []reviewSummaryResponse{}
		}
		h.logger.Printf("review high-rated list count=%d", len(reviews))
		common.WriteJSON(h.logger, w, http.StatusOK, reviews)
	}
}

// reviewDetailHandler はアンケートIDを指定して詳細情報を返す。
// Store情報等を付与したDTOへ変換する責務も担う。
func (h *Handler) reviewDetailHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		if idParam == "" {
			http.Error(w, "IDが指定されていません", http.StatusBadRequest)
			return
		}

		if _, err := primitive.ObjectIDFromHex(idParam); err != nil {
			http.Error(w, "不正なIDです", http.StatusBadRequest)
			return
		}

		survey, err := h.surveyQueries.Detail(ctx, idParam)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				http.NotFound(w, r)
				return
			}
			h.logger.Printf("レビュー詳細の取得に失敗: %v", err)
			http.Error(w, "レビュー詳細の取得に失敗しました", http.StatusInternalServerError)
			return
		}

		detail := buildReviewDetailFromDomain(*survey, "匿名店舗アンケート", "")
		common.WriteJSON(h.logger, w, http.StatusOK, detail)
	}
}

// collectReviews は Query Service からアンケートを取得し、ハンドラ用の表示モデルへ整形する。
func (h *Handler) collectReviews(ctx context.Context, params reviewQueryParams) ([]reviewSummaryResponse, error) {
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

	surveys, err := h.surveyQueries.List(ctx, filter, paging)
	if err != nil {
		return nil, err
	}

	summaries := make([]reviewSummaryResponse, 0, len(surveys))
	for _, survey := range surveys {
		summaries = append(summaries, buildReviewSummaryFromDomain(survey))
	}
	return summaries, nil
}
