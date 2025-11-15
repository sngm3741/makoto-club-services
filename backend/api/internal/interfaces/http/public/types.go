package public

import (
	"math"
	"strings"
	"time"

	"github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
	publicdomain "github.com/sngm3741/makoto-club-services/api/internal/public/domain"
)

type reviewSummaryResponse = publicdomain.SurveySummary
type reviewDetailResponse = publicdomain.SurveyDetail

type storeSummaryResponse struct {
	ID                  string   `json:"id"`
	StoreName           string   `json:"storeName"`
	BranchName          string   `json:"branchName,omitempty"`
	Prefecture          string   `json:"prefecture"`
	Industries          []string `json:"industries,omitempty"`
	AverageRating       float64  `json:"averageRating"`
	AverageEarning      int      `json:"averageEarning"`
	AverageEarningLabel string   `json:"averageEarningLabel,omitempty"`
	WaitTimeHours       int      `json:"waitTimeHours"`
	WaitTimeLabel       string   `json:"waitTimeLabel,omitempty"`
	ReviewCount         int      `json:"reviewCount"`
	Tags                []string `json:"tags,omitempty"`
	PhotoURLs           []string `json:"photoUrls,omitempty"`
}

type storeDetailResponse struct {
	ID                  string          `json:"id"`
	StoreName           string          `json:"storeName"`
	BranchName          string          `json:"branchName,omitempty"`
	Prefecture          string          `json:"prefecture,omitempty"`
	Area                string          `json:"area,omitempty"`
	Genre               string          `json:"genre,omitempty"`
	BusinessHours       string          `json:"businessHours,omitempty"`
	PriceRange          string          `json:"priceRange,omitempty"`
	Industries          []string        `json:"industries,omitempty"`
	EmploymentTypes     []string        `json:"employmentTypes,omitempty"`
	PricePerHour        int             `json:"pricePerHour,omitempty"`
	AverageRating       float64         `json:"averageRating"`
	AverageEarning      int             `json:"averageEarning"`
	AverageEarningLabel string          `json:"averageEarningLabel,omitempty"`
	WaitTimeHours       int             `json:"waitTimeHours"`
	WaitTimeLabel       string          `json:"waitTimeLabel,omitempty"`
	ReviewCount         int             `json:"reviewCount"`
	LastReviewedAt      *time.Time      `json:"lastReviewedAt,omitempty"`
	UpdatedAt           *time.Time      `json:"updatedAt,omitempty"`
	Tags                []string        `json:"tags,omitempty"`
	PhotoURLs           []string        `json:"photoUrls,omitempty"`
	HomepageURL         string          `json:"homepageUrl,omitempty"`
	SNS                 storeSNSPayload `json:"sns"`
	Description         string          `json:"description,omitempty"`
}

type storeListResponse struct {
	Items []storeSummaryResponse `json:"items"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
	Total int                    `json:"total"`
}

type reviewListResponse struct {
	Items []reviewSummaryResponse `json:"items"`
	Page  int                     `json:"page"`
	Limit int                     `json:"limit"`
	Total int                     `json:"total"`
}

type storeSNSPayload struct {
	Twitter   string `json:"twitter,omitempty"`
	Line      string `json:"line,omitempty"`
	Instagram string `json:"instagram,omitempty"`
	TikTok    string `json:"tiktok,omitempty"`
	Official  string `json:"official,omitempty"`
}

// buildStoreSummaryResponse は Store ドメインモデルを一覧表示用 DTO に変換する。
func buildStoreSummaryResponse(store publicdomain.Store) storeSummaryResponse {
	avgEarning := 0
	avgEarningLabel := "-"
	if store.Stats.AvgEarning != nil {
		avgEarning = int(math.Round(*store.Stats.AvgEarning))
		avgEarningLabel = formatAverageEarningLabel(avgEarning)
	}

	waitHours := 0
	waitLabel := "-"
	if store.Stats.AvgWaitTime != nil {
		waitHours = int(math.Round(*store.Stats.AvgWaitTime))
		waitLabel = formatWaitTimeLabel(waitHours)
	}

	avgRating := 0.0
	if store.Stats.AvgRating != nil {
		avgRating = math.Round(*store.Stats.AvgRating*10) / 10
	}

	return storeSummaryResponse{
		ID:                  store.ID,
		StoreName:           store.Name,
		BranchName:          strings.TrimSpace(store.BranchName),
		Prefecture:          store.Prefecture,
		Industries:          common.CanonicalIndustryCodes(store.Industries),
		AverageRating:       avgRating,
		AverageEarning:      avgEarning,
		AverageEarningLabel: avgEarningLabel,
		WaitTimeHours:       waitHours,
		WaitTimeLabel:       waitLabel,
		ReviewCount:         store.Stats.ReviewCount,
		Tags:                append([]string{}, store.Tags...),
		PhotoURLs:           append([]string{}, store.PhotoURLs...),
	}
}

// storeDomainToDetailResponse は Store ドメインモデルを詳細表示用 DTO に変換する。
func storeDomainToDetailResponse(store publicdomain.Store) storeDetailResponse {
	industries := common.CanonicalIndustryCodes(store.Industries)

	avgRating := 0.0
	if store.Stats.AvgRating != nil {
		avgRating = math.Round(*store.Stats.AvgRating*10) / 10
	}

	avgEarning := 0
	avgEarningLabel := "-"
	if store.Stats.AvgEarning != nil {
		avgEarning = int(math.Round(*store.Stats.AvgEarning))
		avgEarningLabel = formatAverageEarningLabel(avgEarning)
	}

	waitHours := 0
	waitLabel := "-"
	if store.Stats.AvgWaitTime != nil {
		waitHours = int(math.Round(*store.Stats.AvgWaitTime))
		waitLabel = formatWaitTimeLabel(waitHours)
	}

	var updatedAt *time.Time
	if !store.UpdatedAt.IsZero() {
		t := store.UpdatedAt
		updatedAt = &t
	}

	return storeDetailResponse{
		ID:                  store.ID,
		StoreName:           store.Name,
		BranchName:          strings.TrimSpace(store.BranchName),
		Prefecture:          store.Prefecture,
		Area:                store.Area,
		Genre:               store.Genre,
		BusinessHours:       store.BusinessHours,
		PriceRange:          store.PriceRange,
		Industries:          industries,
		EmploymentTypes:     append([]string{}, store.EmploymentTypes...),
		PricePerHour:        store.PricePerHour,
		AverageRating:       avgRating,
		AverageEarning:      avgEarning,
		AverageEarningLabel: avgEarningLabel,
		WaitTimeHours:       waitHours,
		WaitTimeLabel:       waitLabel,
		ReviewCount:         store.Stats.ReviewCount,
		LastReviewedAt:      store.Stats.LastReviewedAt,
		UpdatedAt:           updatedAt,
		Tags:                append([]string{}, store.Tags...),
		PhotoURLs:           append([]string{}, store.PhotoURLs...),
		HomepageURL:         store.HomepageURL,
		SNS: storeSNSPayload{
			Twitter:   store.SNS.Twitter,
			Line:      store.SNS.Line,
			Instagram: store.SNS.Instagram,
			TikTok:    store.SNS.TikTok,
			Official:  store.SNS.Official,
		},
		Description: store.Description,
	}
}
