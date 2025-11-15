package public

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
	publicdomain "github.com/sngm3741/makoto-club-services/api/internal/public/domain"
)

var numberPattern = regexp.MustCompile(`\d+(?:\.\d+)?`)

func formatWaitTimeLabel(hours int) string {
	return fmt.Sprintf("%d時間", hours)
}

func formatAverageEarningLabel(value int) string {
	if value >= 20 {
		return "20万円以上"
	}
	return fmt.Sprintf("%d万円", value)
}

func formatVisitedDisplay(visited string) string {
	t, err := time.Parse("2006-01", visited)
	if err != nil {
		return visited
	}
	return fmt.Sprintf("%d年%d月", t.Year(), int(t.Month()))
}

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

func deriveHelpfulCount(createdAt time.Time, spec int) int {
	base := int(createdAt.Unix()%10) + spec
	if base < 5 {
		base = 5
	}
	return base % 40
}

func buildReviewSummaryFromDomain(survey publicdomain.Survey) reviewSummaryResponse {
	industries := common.CanonicalIndustryCodes(survey.Industries)

	visitedAt, createdAt := deriveDates(survey.Period)
	if !survey.CreatedAt.IsZero() {
		createdAt = survey.CreatedAt.Format(time.RFC3339)
	}

	spec := common.IntPtrValue(survey.SpecScore)
	wait := common.IntPtrValue(survey.WaitTime)
	earning := common.IntPtrValue(survey.AverageEarning)
	helpful := survey.HelpfulCount
	if helpful == 0 {
		helpful = deriveHelpfulCount(survey.CreatedAt, spec)
	}

	waitLabel := ""
	if wait > 0 {
		waitLabel = formatWaitTimeLabel(wait)
	}
	earningLabel := ""
	if earning > 0 {
		earningLabel = formatAverageEarningLabel(earning)
	}

	excerpt := buildExcerpt(survey.Comment, survey.StoreName, earningLabel, waitLabel)

	return reviewSummaryResponse{
		ID:             survey.ID,
		StoreID:        survey.StoreID,
		StoreName:      survey.StoreName,
		BranchName:     strings.TrimSpace(survey.BranchName),
		Prefecture:     survey.Prefecture,
		Industries:     industries,
		VisitedAt:      visitedAt,
		Age:            common.IntPtrValue(survey.Age),
		SpecScore:      spec,
		WaitTimeHours:  wait,
		AverageEarning: earning,
		Rating:         survey.Rating,
		CreatedAt:      createdAt,
		HelpfulCount:   helpful,
		Excerpt:        excerpt,
		Tags:           append([]string{}, survey.Tags...),
		Photos:         append([]publicdomain.SurveyPhoto{}, survey.Photos...),
	}
}

func buildReviewDetailFromDomain(survey publicdomain.Survey, authorName, authorAvatar string) reviewDetailResponse {
	summary := buildReviewSummaryFromDomain(survey)
	description := strings.TrimSpace(survey.Comment)
	if description == "" {
		description = buildFallbackDescription(summary)
	}
	return reviewDetailResponse{
		SurveySummary:     summary,
		Description:       description,
		AuthorDisplayName: authorName,
		AuthorAvatarURL:   authorAvatar,
		CustomerNote:      strings.TrimSpace(survey.CustomerNote),
		StaffNote:         strings.TrimSpace(survey.StaffNote),
		EnvironmentNote:   strings.TrimSpace(survey.EnvironmentNote),
		Comment:           strings.TrimSpace(survey.Comment),
	}
}

func buildFallbackDescription(summary reviewSummaryResponse) string {
	return fmt.Sprintf(
		"%sでの体験談です。平均稼ぎはおよそ%d万円、待機時間は%d時間程度でした。年代: %d歳、スペック: %d を参考にしてください。",
		summary.StoreName,
		summary.AverageEarning,
		summary.WaitTimeHours,
		summary.Age,
		summary.SpecScore,
	)
}

func buildExcerpt(comment, storeName string, earning any, wait any) string {
	trimmed := strings.TrimSpace(comment)
	if trimmed != "" {
		runes := []rune(trimmed)
		if len(runes) > 60 {
			trimmed = string(runes[:60]) + "…"
		}
		return trimmed
	}

	components := []string{}
	if earningValue := extractFirstInt(earning); earningValue > 0 {
		components = append(components, fmt.Sprintf("平均稼ぎは%d万円", earningValue))
	}
	if waitValue := extractFirstInt(wait); waitValue > 0 {
		components = append(components, fmt.Sprintf("待機は%d時間程度", waitValue))
	}
	if len(components) == 0 {
		return fmt.Sprintf("%sの最新アンケートです。", storeName)
	}
	return strings.Join(components, "／")
}

func extractFirstInt(value any) int {
	switch v := value.(type) {
	case *int:
		if v == nil {
			return 0
		}
		return *v
	case *int32:
		if v == nil {
			return 0
		}
		return int(*v)
	case *int64:
		if v == nil {
			return 0
		}
		return int(*v)
	case *float64:
		if v == nil {
			return 0
		}
		return int(math.Round(*v))
	case int32:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	case float64:
		return int(math.Round(v))
	case string:
		match := numberPattern.FindString(v)
		if match == "" {
			return 0
		}
		num, err := strconv.Atoi(match)
		if err != nil {
			return 0
		}
		return num
	default:
		return 0
	}
}

func reviewerDisplayName(user common.AuthenticatedUser) string {
	name := strings.TrimSpace(user.Name)
	if name != "" {
		return name
	}
	return "匿名店舗アンケート"
}
