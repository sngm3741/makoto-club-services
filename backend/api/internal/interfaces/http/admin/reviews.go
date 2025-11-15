package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	mongodoc "github.com/sngm3741/makoto-club-services/api/internal/infrastructure/mongo"
	"github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (h *Handler) reviewListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		storeIDParam := strings.TrimSpace(query.Get("storeId"))
		filter := bson.M{}
		if storeIDParam != "" {
			storeID, err := primitive.ObjectIDFromHex(storeIDParam)
			if err != nil {
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "店舗IDの形式が不正です"})
				return
			}
			filter["storeId"] = storeID
		}

		opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		cursor, err := h.reviews.Find(ctx, filter, opts)
		if err != nil {
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "アンケート一覧の取得に失敗しました"})
			return
		}
		defer cursor.Close(ctx)

		var reviews []mongodoc.ReviewDocument
		storeIDSet := make(map[primitive.ObjectID]struct{})
		for cursor.Next(ctx) {
			var doc mongodoc.ReviewDocument
			if err := cursor.Decode(&doc); err != nil {
				h.logger.Printf("管理リスト用レビューのデコードに失敗: %v", err)
				continue
			}
			reviews = append(reviews, doc)
			storeIDSet[doc.StoreID] = struct{}{}
		}

		if err := cursor.Err(); err != nil {
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "アンケート一覧の取得に失敗しました"})
			return
		}

		storeIDs := make([]primitive.ObjectID, 0, len(storeIDSet))
		for id := range storeIDSet {
			storeIDs = append(storeIDs, id)
		}

		storeMap, err := h.loadStoresMap(ctx, storeIDs)
		if err != nil {
			h.logger.Printf("管理リスト用店舗の取得に失敗: %v", err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		items := make([]adminReviewResponse, 0, len(reviews))
		for _, review := range reviews {
			store, ok := storeMap[review.StoreID]
			if !ok {
				if fetched, fetchErr := h.getStoreByID(ctx, review.StoreID); fetchErr == nil {
					store = fetched
				} else {
					h.logger.Printf("管理リスト用店舗が見つかりません reviewId=%s storeId=%s err=%v", review.ID.Hex(), review.StoreID.Hex(), fetchErr)
				}
			}
			items = append(items, buildAdminReviewResponse(review, store))
		}

		h.logger.Printf("admin review list: storeId=%q count=%d", storeIDParam, len(items))
		common.WriteJSON(h.logger, w, http.StatusOK, adminReviewListResponse{Items: items})
	}
}

func (h *Handler) reviewDetailHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		h.logger.Printf("admin review detail request id=%q", idParam)
		objectID, err := primitive.ObjectIDFromHex(idParam)
		if err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "レビューIDの形式が不正です"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var review mongodoc.ReviewDocument
		if err := h.reviews.FindOne(ctx, bson.M{"_id": objectID}).Decode(&review); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				h.logger.Printf("admin review detail not found id=%q", idParam)
				common.WriteJSON(h.logger, w, http.StatusNotFound, map[string]string{"error": "レビューが見つかりません"})
				return
			}
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "レビューの取得に失敗しました"})
			return
		}

		store, err := h.getStoreByID(ctx, review.StoreID)
		if err != nil {
			h.logger.Printf("admin review detail store fetch failed id=%q storeId=%s err=%v", idParam, review.StoreID.Hex(), err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		h.logger.Printf("admin review detail success id=%q", idParam)
		common.WriteJSON(h.logger, w, http.StatusOK, buildAdminReviewResponse(review, store))
	}
}

func (h *Handler) reviewUpdateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		h.logger.Printf("admin review content update request id=%q", idParam)
		objectID, err := primitive.ObjectIDFromHex(idParam)
		if err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "レビューIDの形式が不正です"})
			return
		}

		var req updateReviewContentRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, common.MaxReviewRequestBody)).Decode(&req); err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "リクエストの形式が不正です"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var existing mongodoc.ReviewDocument
		if err := h.reviews.FindOne(ctx, bson.M{"_id": objectID}).Decode(&existing); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				h.logger.Printf("admin review content update not found id=%q", idParam)
				common.WriteJSON(h.logger, w, http.StatusNotFound, map[string]string{"error": "レビューが見つかりません"})
				return
			}
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "レビューの取得に失敗しました"})
			return
		}

		reviewUpdate := bson.M{}
		storeUpdate := bson.M{}
		now := time.Now().In(h.location)
		var addIndustry string
		targetStoreID := existing.StoreID
		storeChanged := false

		if req.StoreID != nil {
			storeIDHex := strings.TrimSpace(*req.StoreID)
			if storeIDHex == "" {
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "店舗IDが指定されていません"})
				return
			}
			newStoreID, err := primitive.ObjectIDFromHex(storeIDHex)
			if err != nil {
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "店舗IDの形式が不正です"})
				return
			}
			if newStoreID != existing.StoreID {
				if err := h.stores.FindOne(ctx, bson.M{"_id": newStoreID}).Err(); err != nil {
					if errors.Is(err, mongo.ErrNoDocuments) {
						common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "指定された店舗が存在しません"})
						return
					}
					h.logger.Printf("admin review content update store lookup failed id=%q storeId=%s err=%v", idParam, storeIDHex, err)
					common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
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
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "店舗名は必須です"})
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
			category := common.CanonicalIndustryCode(*req.Category)
			reviewUpdate["industryCode"] = category
			if category != "" {
				addIndustry = category
			}
		}
		if req.VisitedAt != nil {
			period, err := formatSurveyPeriod(strings.TrimSpace(*req.VisitedAt))
			if err != nil {
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			reviewUpdate["period"] = period
		}
		if req.Age != nil {
			age := *req.Age
			if age < 18 {
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "年齢は18歳以上で入力してください"})
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
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "スペックは60以上で入力してください"})
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
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "待機時間は1時間以上で入力してください"})
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
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "平均稼ぎは0以上で入力してください"})
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
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "感想は2000文字以内で入力してください"})
				return
			}
			reviewUpdate["comment"] = comment
		}
		if req.Rating != nil {
			rating := *req.Rating
			if rating < 0 || rating > 5 {
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "総評は0〜5の範囲で入力してください"})
				return
			}
			reviewUpdate["rating"] = math.Round(rating*2) / 2
		}
		if req.ContactEmail != nil {
			email, err := normalizeEmail(*req.ContactEmail)
			if err != nil {
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			reviewUpdate["contactEmail"] = email
		}

		if len(storeUpdate) == 0 && len(reviewUpdate) == 0 && addIndustry == "" {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "更新内容が指定されていません"})
			return
		}

		if len(storeUpdate) > 0 && !targetStoreID.IsZero() {
			storeUpdate["updatedAt"] = now
			if _, err := h.stores.UpdateByID(ctx, targetStoreID, bson.M{"$set": storeUpdate}); err != nil {
				h.logger.Printf("admin review content update store update failed id=%q storeId=%s err=%v", idParam, targetStoreID.Hex(), err)
				common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の更新に失敗しました"})
				return
			}
		}
		if addIndustry != "" && !targetStoreID.IsZero() {
			if _, err := h.stores.UpdateByID(ctx, targetStoreID, bson.M{"$addToSet": bson.M{"industries": addIndustry}}); err != nil {
				h.logger.Printf("admin review content update industry append failed id=%q storeId=%s err=%v", idParam, targetStoreID.Hex(), err)
			}
		}

		var updated mongodoc.ReviewDocument
		if len(reviewUpdate) > 0 {
			reviewUpdate["updatedAt"] = now
			result := h.reviews.FindOneAndUpdate(ctx, bson.M{"_id": objectID}, bson.M{"$set": reviewUpdate}, options.FindOneAndUpdate().SetReturnDocument(options.After))
			if err := result.Decode(&updated); err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					h.logger.Printf("admin review content update disappeared id=%q", idParam)
					common.WriteJSON(h.logger, w, http.StatusNotFound, map[string]string{"error": "レビューが見つかりません"})
					return
				}
				common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "レビューの更新に失敗しました"})
				return
			}
		} else {
			updated = existing
		}

		if err := h.recalculateStoreStats(ctx, updated.StoreID); err != nil {
			h.logger.Printf("admin review content update stats recalculation failed id=%q err=%v", idParam, err)
		}
		if storeChanged && existing.StoreID != updated.StoreID && !existing.StoreID.IsZero() {
			if err := h.recalculateStoreStats(ctx, existing.StoreID); err != nil {
				h.logger.Printf("admin review content update old store stats recalculation failed id=%q storeId=%s err=%v", idParam, existing.StoreID.Hex(), err)
			}
		}

		store, err := h.getStoreByID(ctx, updated.StoreID)
		if err != nil {
			h.logger.Printf("admin review content update store fetch failed id=%q storeId=%s err=%v", idParam, updated.StoreID.Hex(), err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		response := buildAdminReviewResponse(updated, store)
		common.WriteJSON(h.logger, w, http.StatusOK, response)
	}
}
