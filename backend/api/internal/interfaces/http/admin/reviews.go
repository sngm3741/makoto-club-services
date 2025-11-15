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
	adminapp "github.com/sngm3741/makoto-club-services/api/internal/admin/application"
	admindomain "github.com/sngm3741/makoto-club-services/api/internal/admin/domain"
	"github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
	"go.mongodb.org/mongo-driver/mongo"
)

func (h *Handler) reviewListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		storeID := strings.TrimSpace(query.Get("storeId"))
		keyword := strings.TrimSpace(query.Get("keyword"))
		limit, _ := common.ParsePositiveInt(query.Get("limit"), 0)

		filter := adminapp.SurveyFilter{StoreID: storeID, Keyword: keyword}
		paging := adminapp.Paging{Limit: limit}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		surveys, err := h.surveyService.List(ctx, filter, paging)
		if err != nil {
			h.logger.Printf("admin review list fetch failed: %v", err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "アンケート一覧の取得に失敗しました"})
			return
		}

		items := make([]adminReviewResponse, 0, len(surveys))
		for _, survey := range surveys {
			items = append(items, adminSurveyDomainToResponse(survey))
		}
		common.WriteJSON(h.logger, w, http.StatusOK, adminReviewListResponse{Items: items})
	}
}

func (h *Handler) reviewDetailHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		if idParam == "" {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "アンケートIDが指定されていません"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		survey, err := h.surveyService.Detail(ctx, idParam)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				common.WriteJSON(h.logger, w, http.StatusNotFound, map[string]string{"error": "アンケートが見つかりません"})
				return
			}
			h.logger.Printf("admin review detail fetch failed id=%s err=%v", idParam, err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "アンケートの取得に失敗しました"})
			return
		}

		common.WriteJSON(h.logger, w, http.StatusOK, adminSurveyDomainToResponse(*survey))
	}
}

func (h *Handler) reviewUpdateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		if idParam == "" {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "アンケートIDが指定されていません"})
			return
		}

		var req updateReviewContentRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, common.MaxReviewRequestBody)).Decode(&req); err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "リクエストの形式が不正です"})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		existing, err := h.surveyService.Detail(ctx, idParam)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				common.WriteJSON(h.logger, w, http.StatusNotFound, map[string]string{"error": "アンケートが見つかりません"})
				return
			}
			h.logger.Printf("admin review update detail fetch failed id=%s err=%v", idParam, err)
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "アンケートの取得に失敗しました"})
			return
		}

		cmd := buildSurveyCommandFromDomain(*existing)
		if err := applyReviewUpdateRequest(req, &cmd, *existing); err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		updated, err := h.surveyService.Update(ctx, idParam, cmd)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				common.WriteJSON(h.logger, w, http.StatusNotFound, map[string]string{"error": "アンケートが見つかりません"})
				return
			}
			h.logger.Printf("admin review update failed id=%s err=%v", idParam, err)
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		common.WriteJSON(h.logger, w, http.StatusOK, adminSurveyDomainToResponse(*updated))
	}
}

func buildSurveyCommandFromDomain(survey admindomain.Survey) adminapp.UpsertSurveyCommand {
	cmd := adminapp.UpsertSurveyCommand{
		StoreID:         survey.StoreID,
		StoreName:       survey.StoreName,
		BranchName:      survey.BranchName,
		Prefecture:      survey.Prefecture.String(),
		Area:            survey.Area,
		Industries:      survey.Industries.Strings(),
		Genre:           survey.Genre,
		Period:          survey.Period,
		EmploymentType:  survey.EmploymentType,
		CustomerNote:    survey.CustomerNote,
		StaffNote:       survey.StaffNote,
		EnvironmentNote: survey.EnvironmentNote,
		Comment:         survey.Comment,
		ContactEmail:    survey.ContactEmail.String(),
		Tags:            survey.Tags.Strings(),
		Rating:          survey.Rating.Float64(),
		HelpfulCount:    survey.HelpfulCount,
	}
	if survey.Age != nil {
		cmd.Age = common.IntPtr(*survey.Age)
	}
	if survey.SpecScore != nil {
		cmd.SpecScore = common.IntPtr(*survey.SpecScore)
	}
	if survey.WaitTime != nil {
		cmd.WaitTime = common.IntPtr(*survey.WaitTime)
	}
	if survey.AverageEarning != nil {
		cmd.AverageEarning = common.IntPtr(*survey.AverageEarning)
	}
	if len(survey.Photos) > 0 {
		cmd.Photos = mapSurveyPhotosToCommands(survey.Photos)
	}
	return cmd
}

func mapSurveyPhotosToCommands(photos []admindomain.SurveyPhoto) []adminapp.SurveyPhotoCommand {
	result := make([]adminapp.SurveyPhotoCommand, 0, len(photos))
	for _, photo := range photos {
		result = append(result, adminapp.SurveyPhotoCommand{
			ID:          photo.ID,
			StoredPath:  photo.StoredPath,
			PublicURL:   photo.PublicURL.String(),
			ContentType: photo.ContentType,
			UploadedAt:  photo.UploadedAt,
		})
	}
	return result
}

func applyReviewUpdateRequest(req updateReviewContentRequest, cmd *adminapp.UpsertSurveyCommand, existing admindomain.Survey) error {
	changed := false

	if req.StoreID != nil {
		if trimmed := strings.TrimSpace(*req.StoreID); trimmed != "" && trimmed != existing.StoreID {
			return errors.New("店舗IDの変更はできません")
		}
	}

	if req.StoreName != nil {
		name := strings.TrimSpace(*req.StoreName)
		if name == "" {
			return errors.New("店舗名は必須です")
		}
		cmd.StoreName = name
		changed = true
	}
	if req.BranchName != nil {
		cmd.BranchName = strings.TrimSpace(*req.BranchName)
		changed = true
	}
	if req.Prefecture != nil {
		cmd.Prefecture = strings.TrimSpace(*req.Prefecture)
		changed = true
	}
	if req.Category != nil {
		category := common.CanonicalIndustryCode(*req.Category)
		if category == "" {
			return errors.New("業種を指定してください")
		}
		cmd.Industries = []string{category}
		changed = true
	}
	if req.VisitedAt != nil {
		period, err := formatSurveyPeriod(strings.TrimSpace(*req.VisitedAt))
		if err != nil {
			return err
		}
		cmd.Period = period
		changed = true
	}
	if req.Age != nil {
		value, err := validateAge(*req.Age)
		if err != nil {
			return err
		}
		cmd.Age = common.IntPtr(value)
		changed = true
	}
	if req.SpecScore != nil {
		value, err := validateSpecScore(*req.SpecScore)
		if err != nil {
			return err
		}
		cmd.SpecScore = common.IntPtr(value)
		changed = true
	}
	if req.WaitTimeHours != nil {
		value, err := validateWaitTimeHours(*req.WaitTimeHours)
		if err != nil {
			return err
		}
		minutes := value * 60
		cmd.WaitTime = common.IntPtr(minutes)
		cmd.WaitTimeHours = nil
		changed = true
	}
	if req.AverageEarning != nil {
		value, err := validateAverageEarning(*req.AverageEarning)
		if err != nil {
			return err
		}
		cmd.AverageEarning = common.IntPtr(value)
		changed = true
	}
	if req.Comment != nil {
		comment := strings.TrimSpace(*req.Comment)
		if len([]rune(comment)) > 2000 {
			return errors.New("感想は2000文字以内で入力してください")
		}
		cmd.Comment = comment
		cmd.CustomerNote = comment
		changed = true
	}
	if req.Rating != nil {
		rating := *req.Rating
		if rating < 0 || rating > 5 {
			return errors.New("総評は0〜5の範囲で入力してください")
		}
		cmd.Rating = math.Round(rating*2) / 2
		changed = true
	}
	if req.ContactEmail != nil {
		email, err := normalizeEmail(*req.ContactEmail)
		if err != nil {
			return err
		}
		cmd.ContactEmail = email
		changed = true
	}

	if !changed {
		return errors.New("更新内容が指定されていません")
	}
	return nil
}

func validateAge(age int) (int, error) {
	if age < 18 {
		return 0, errors.New("年齢は18歳以上で入力してください")
	}
	if age > 60 {
		return 60, nil
	}
	return age, nil
}

func validateSpecScore(score int) (int, error) {
	if score < 60 {
		return 0, errors.New("スペックは60以上で入力してください")
	}
	if score > 140 {
		return 140, nil
	}
	return score, nil
}

func validateWaitTimeHours(hours int) (int, error) {
	if hours < 1 {
		return 0, errors.New("待機時間は1時間以上で入力してください")
	}
	if hours > 24 {
		return 24, nil
	}
	return hours, nil
}

func validateAverageEarning(value int) (int, error) {
	if value < 0 {
		return 0, errors.New("平均稼ぎは0以上で入力してください")
	}
	if value > 20 {
		return 20, nil
	}
	return value, nil
}
