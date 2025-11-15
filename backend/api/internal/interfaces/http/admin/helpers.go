package admin

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	admindomain "github.com/sngm3741/makoto-club-services/api/internal/admin/domain"
	"github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
)

// adminSurveyDomainToResponse はドメインの Survey 集約を Admin UI 用レスポンスへ変換する。
func adminSurveyDomainToResponse(survey admindomain.Survey) adminReviewResponse {
	industryList := survey.Industries.Strings()
	industries := common.CanonicalIndustryCodes(industryList)
	category := ""
	if len(industries) > 0 {
		category = industries[0]
	} else {
		category = "デリヘル"
	}

	visitedAt, _ := deriveDates(survey.Period)
	waitMinutes := intPtrValue(survey.WaitTime)

	return adminReviewResponse{
		ID:              survey.ID,
		StoreID:         survey.StoreID,
		StoreName:       survey.StoreName,
		BranchName:      strings.TrimSpace(survey.BranchName),
		Prefecture:      survey.Prefecture.String(),
		Area:            survey.Area,
		Category:        category,
		Industries:      industries,
		Genre:           survey.Genre,
		VisitedAt:       visitedAt,
		Age:             intPtrValue(survey.Age),
		SpecScore:       intPtrValue(survey.SpecScore),
		WaitTimeMinutes: waitMinutes,
		AverageEarning:  intPtrValue(survey.AverageEarning),
		EmploymentType:  survey.EmploymentType,
		Rating:          survey.Rating.Float64(),
		Comment:         strings.TrimSpace(survey.Comment),
		CustomerNote:    strings.TrimSpace(survey.CustomerNote),
		StaffNote:       strings.TrimSpace(survey.StaffNote),
		EnvironmentNote: strings.TrimSpace(survey.EnvironmentNote),
		Tags:            survey.Tags.Strings(),
		ContactEmail:    survey.ContactEmail.String(),
		Photos:          convertSurveyPhotosForAdminDomain(survey.Photos),
		HelpfulCount:    survey.HelpfulCount,
		CreatedAt:       survey.CreatedAt,
		UpdatedAt:       survey.UpdatedAt,
	}
}

// convertSurveyPhotosForAdminDomain は写真メタデータを Admin レスポンス形式へ変換する。
func convertSurveyPhotosForAdminDomain(photos []admindomain.SurveyPhoto) []adminSurveyPhotoResponse {
	if len(photos) == 0 {
		return nil
	}
	result := make([]adminSurveyPhotoResponse, 0, len(photos))
	for _, photo := range photos {
		result = append(result, adminSurveyPhotoResponse{
			ID:          photo.ID,
			StoredPath:  photo.StoredPath,
			PublicURL:   photo.PublicURL.String(),
			ContentType: photo.ContentType,
			UploadedAt:  photo.UploadedAt,
		})
	}
	return result
}

// deriveDates は期間文字列から訪問年月と日付を推測する。
func deriveDates(period string) (visited string, created string) {
	period = strings.TrimSpace(period)
	if period == "" {
		now := time.Now()
		return now.Format("2006-01"), now.Format("2006-01-02")
	}

	replacer := strings.NewReplacer("年", "-", "月", "-01")
	normalized := replacer.Replace(period)
	t, err := time.Parse("2006-01-02", normalized)
	if err != nil {
		now := time.Now()
		return now.Format("2006-01"), now.Format("2006-01-02")
	}
	return t.Format("2006-01"), t.Format("2006-01-02")
}

// formatSurveyPeriod は Admin 入力フォーム用に訪問時期を標準化する。
func formatSurveyPeriod(visited string) (string, error) {
	value := strings.TrimSpace(visited)
	if value == "" {
		return "", errors.New("働いた時期を指定してください")
	}

	t, err := time.Parse("2006-01", value)
	if err != nil {
		return "", fmt.Errorf("働いた時期の形式が不正です: %w", err)
	}

	return fmt.Sprintf("%d年%d月", t.Year(), int(t.Month())), nil
}

// normalizeEmail はメールアドレスをトリムし、形式を検証する。
func normalizeEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > 254 {
		return "", errors.New("メールアドレスは254文字以内で入力してください")
	}
	if _, err := mail.ParseAddress(trimmed); err != nil {
		return "", errors.New("メールアドレスの形式が正しくありません")
	}
	return trimmed, nil
}

// intPtrValue は nil セーフに int 値を取り出す。
func intPtrValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
