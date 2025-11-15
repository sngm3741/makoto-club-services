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

func (h *Handler) storeListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		query := r.URL.Query()
		prefectureFilter := strings.TrimSpace(query.Get("prefecture"))
		categoryFilter := common.CanonicalIndustryCode(query.Get("category"))
		keyword := strings.TrimSpace(query.Get("keyword"))
		sortKey := strings.TrimSpace(query.Get("sort"))

		page, _ := common.ParsePositiveInt(query.Get("page"), 1)
		limit, _ := common.ParsePositiveInt(query.Get("limit"), 10)
		if limit <= 0 {
			limit = 10
		}

		filter := publicapp.StoreFilter{
			Prefecture: prefectureFilter,
			Genre:      categoryFilter,
			Keyword:    keyword,
			Tags:       query["tags"],
		}
		paging := publicapp.Paging{
			Page:  page,
			Limit: limit,
			Sort:  sortKey,
		}

		stores, err := h.storeQueries.List(ctx, filter, paging)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				common.WriteJSON(h.logger, w, http.StatusOK, storeListResponse{
					Items: []storeSummaryResponse{},
					Page:  page,
					Limit: limit,
					Total: 0,
				})
				return
			}
			h.logger.Printf("store list fetch failed: %v", err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "店舗一覧の取得に失敗しました"})
			return
		}

		total := len(stores)
		start := (page - 1) * limit
		if start >= total {
			start = total
		}
		end := start + limit
		if end > total {
			end = total
		}

		items := make([]storeSummaryResponse, 0, end-start)
		for _, store := range stores[start:end] {
			items = append(items, buildStoreSummaryResponse(store))
		}

		common.WriteJSON(h.logger, w, http.StatusOK, storeListResponse{
			Items: items,
			Page:  page,
			Limit: limit,
			Total: total,
		})
	}
}

func (h *Handler) storeDetailHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		if idParam == "" {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "店舗IDが指定されていません"})
			return
		}
		if _, err := primitive.ObjectIDFromHex(idParam); err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "店舗IDの形式が不正です"})
			return
		}

		store, err := h.storeQueries.Detail(ctx, idParam)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				common.WriteJSON(h.logger, w, http.StatusNotFound, map[string]string{"error": "店舗が見つかりません"})
				return
			}
			h.logger.Printf("store detail fetch failed id=%q err=%v", idParam, err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		common.WriteJSON(h.logger, w, http.StatusOK, storeDomainToDetailResponse(*store))
	}
}
