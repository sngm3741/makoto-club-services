package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	adminapp "github.com/sngm3741/makoto-club-services/api/internal/admin/application"
	"github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (h *Handler) storeSearchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		queryValues := r.URL.Query()
		prefecture := strings.TrimSpace(queryValues.Get("prefecture"))
		industry := common.CanonicalIndustryCode(queryValues.Get("industry"))
		keyword := strings.TrimSpace(queryValues.Get("keyword"))
		limit, _ := common.ParsePositiveInt(queryValues.Get("limit"), 20)

		filter := adminapp.StoreFilter{Prefecture: prefecture, Genre: industry, Keyword: keyword}
		paging := adminapp.Paging{Limit: limit}

		stores, err := h.storeService.List(ctx, filter, paging)
		if err != nil {
			h.logger.Printf("admin store search failed: %v", err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "店舗一覧の取得に失敗しました"})
			return
		}

		items := make([]adminStoreResponse, 0, len(stores))
		for _, store := range stores {
			items = append(items, adminStoreDomainToResponse(store))
		}

		common.WriteJSON(h.logger, w, http.StatusOK, map[string]any{"items": items})
	}
}

func (h *Handler) storeDetailHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		objectID, err := primitive.ObjectIDFromHex(idParam)
		if err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "店舗IDの形式が不正です"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		store, err := h.storeService.Detail(ctx, objectID.Hex())
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				common.WriteJSON(h.logger, w, http.StatusNotFound, map[string]string{"error": "店舗が見つかりません"})
				return
			}
			h.logger.Printf("admin store detail fetch failed id=%s err=%v", idParam, err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		common.WriteJSON(h.logger, w, http.StatusOK, adminStoreDomainToResponse(*store))
	}
}

func (h *Handler) storeCreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req adminStoreCreateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, common.MaxReviewRequestBody)).Decode(&req); err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "リクエストの形式が不正です"})
			return
		}

		cmd, err := h.buildStoreCommand(req)
		if err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		store, err := h.storeService.Create(ctx, cmd)
		if err != nil {
			h.logger.Printf("admin store create failed: %v", err)
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		common.WriteJSON(h.logger, w, http.StatusCreated, adminStoreCreateResponse{Store: adminStoreDomainToResponse(*store), Created: true})
	}
}

func (h *Handler) storeReviewCreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		storeIDParam := strings.TrimSpace(chi.URLParam(r, "id"))
		if storeIDParam == "" {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "店舗IDが指定されていません"})
			return
		}
		if _, err := primitive.ObjectIDFromHex(storeIDParam); err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "店舗IDの形式が不正です"})
			return
		}

		var req adminStoreReviewCreateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, common.MaxReviewRequestBody)).Decode(&req); err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "リクエストの形式が不正です"})
			return
		}

		metrics := reviewMetrics{
			VisitedAt:      req.VisitedAt,
			Age:            req.Age,
			SpecScore:      req.SpecScore,
			WaitTimeHours:  req.WaitTimeHours,
			AverageEarning: req.AverageEarning,
			Comment:        req.Comment,
			Rating:         req.Rating,
			ContactEmail:   req.ContactEmail,
		}
		if err := metrics.normalize(); err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		period, err := formatSurveyPeriod(metrics.VisitedAt)
		if err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		store, err := h.storeService.Detail(ctx, storeIDParam)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				common.WriteJSON(h.logger, w, http.StatusNotFound, map[string]string{"error": "指定された店舗が見つかりません"})
				return
			}
			h.logger.Printf("admin store review create store fetch failed id=%s err=%v", storeIDParam, err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "店舗情報の取得に失敗しました"})
			return
		}

		category := resolveIndustryForSurvey(req.IndustryCode, store.Industries.Strings())
		waitMinutes := metrics.WaitTimeHours * 60
		comment := strings.TrimSpace(metrics.Comment)
		email, err := normalizeEmail(metrics.ContactEmail)
		if err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		cmd := adminapp.UpsertSurveyCommand{
			StoreID:         store.ID,
			StoreName:       store.Name,
			BranchName:      store.BranchName,
			Prefecture:      store.Prefecture.String(),
			Area:            store.Area,
			Industries:      []string{category},
			Genre:           store.Genre,
			Period:          period,
			Age:             common.IntPtr(metrics.Age),
			SpecScore:       common.IntPtr(metrics.SpecScore),
			WaitTime:        common.IntPtr(waitMinutes),
			AverageEarning:  common.IntPtr(metrics.AverageEarning),
			EmploymentType:  "",
			CustomerNote:    comment,
			StaffNote:       "",
			EnvironmentNote: "",
			Comment:         comment,
			ContactEmail:    email,
			Tags:            nil,
			Photos:          nil,
			Rating:          metrics.Rating,
			HelpfulCount:    0,
		}

		survey, err := h.surveyService.Create(ctx, cmd)
		if err != nil {
			h.logger.Printf("admin store review create failed storeId=%s err=%v", storeIDParam, err)
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		common.WriteJSON(h.logger, w, http.StatusCreated, adminSurveyDomainToResponse(*survey))
	}
}

func (h *Handler) buildStoreCommand(req adminStoreCreateRequest) (adminapp.UpsertStoreCommand, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return adminapp.UpsertStoreCommand{}, errors.New("店舗名は必須です")
	}
	prefecture := strings.TrimSpace(req.Prefecture)
	if prefecture == "" {
		return adminapp.UpsertStoreCommand{}, errors.New("都道府県は必須です")
	}

	industries, err := common.NormalizeIndustryList(req.Industries)
	if err != nil {
		return adminapp.UpsertStoreCommand{}, err
	}

	employmentTypes, err := common.NormalizeEmploymentTypes(req.EmploymentTypes)
	if err != nil {
		return adminapp.UpsertStoreCommand{}, err
	}

	tags, err := common.NormalizeStoreTags(req.Tags)
	if err != nil {
		return adminapp.UpsertStoreCommand{}, err
	}

	photos, err := normalizePhotoURLs(req.PhotoURLs, common.MaxStorePhotoCount)
	if err != nil {
		return adminapp.UpsertStoreCommand{}, err
	}

	description := strings.TrimSpace(req.Description)
	if utf8.RuneCountInString(description) > common.MaxStoreDescriptionRunes {
		return adminapp.UpsertStoreCommand{}, fmt.Errorf("店舗説明は最大%d文字までです", common.MaxStoreDescriptionRunes)
	}

	if req.PricePerHour < 0 {
		return adminapp.UpsertStoreCommand{}, errors.New("単価は0以上で入力してください")
	}
	if req.AverageEarning < 0 {
		return adminapp.UpsertStoreCommand{}, errors.New("平均稼ぎは0以上で入力してください")
	}

	return adminapp.UpsertStoreCommand{
		Name:            name,
		BranchName:      strings.TrimSpace(req.BranchName),
		GroupName:       strings.TrimSpace(req.GroupName),
		Prefecture:      prefecture,
		Area:            strings.TrimSpace(req.Area),
		Genre:           strings.TrimSpace(req.Genre),
		Industries:      industries,
		EmploymentTypes: employmentTypes,
		PricePerHour:    req.PricePerHour,
		PriceRange:      strings.TrimSpace(req.PriceRange),
		AverageEarning:  req.AverageEarning,
		BusinessHours:   strings.TrimSpace(req.BusinessHours),
		Tags:            tags,
		HomepageURL:     strings.TrimSpace(req.HomepageURL),
		SNS: adminapp.StoreSNSCommand{
			Twitter:   strings.TrimSpace(req.SNS.Twitter),
			Line:      strings.TrimSpace(req.SNS.Line),
			Instagram: strings.TrimSpace(req.SNS.Instagram),
			TikTok:    strings.TrimSpace(req.SNS.TikTok),
			Official:  strings.TrimSpace(req.SNS.Official),
		},
		PhotoURLs:   photos,
		Description: description,
	}, nil
}

func normalizePhotoURLs(urls []string, max int) ([]string, error) {
	if len(urls) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, len(urls))
	for _, raw := range urls {
		urlStr := strings.TrimSpace(raw)
		if urlStr == "" {
			continue
		}
		if len(urlStr) > 2048 {
			return nil, fmt.Errorf("URLが長すぎます: %s", urlStr)
		}
		if _, ok := seen[urlStr]; ok {
			continue
		}
		seen[urlStr] = struct{}{}
		result = append(result, urlStr)
		if len(result) > max {
			return nil, fmt.Errorf("写真URLは最大%d件までです", max)
		}
	}
	return result, nil
}

func resolveIndustryForSurvey(input string, storeIndustries []string) string {
	if category := common.CanonicalIndustryCode(input); category != "" {
		return category
	}
	for _, raw := range storeIndustries {
		if category := common.CanonicalIndustryCode(raw); category != "" {
			return category
		}
	}
	return "デリヘル"
}
