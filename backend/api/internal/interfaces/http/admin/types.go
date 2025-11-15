package admin

import (
	"strings"
	"time"

	admindomain "github.com/sngm3741/makoto-club-services/api/internal/admin/domain"
)

type adminReviewResponse struct {
	ID              string                     `json:"id"`
	StoreID         string                     `json:"storeId"`
	StoreName       string                     `json:"storeName"`
	BranchName      string                     `json:"branchName,omitempty"`
	Prefecture      string                     `json:"prefecture"`
	Area            string                     `json:"area,omitempty"`
	Category        string                     `json:"category"`
	Industries      []string                   `json:"industries,omitempty"`
	Genre           string                     `json:"genre,omitempty"`
	VisitedAt       string                     `json:"visitedAt"`
	Age             int                        `json:"age"`
	SpecScore       int                        `json:"specScore"`
	WaitTimeMinutes int                        `json:"waitTimeMinutes"`
	AverageEarning  int                        `json:"averageEarning"`
	EmploymentType  string                     `json:"employmentType,omitempty"`
	Rating          float64                    `json:"rating"`
	Comment         string                     `json:"comment,omitempty"`
	CustomerNote    string                     `json:"customerNote,omitempty"`
	StaffNote       string                     `json:"staffNote,omitempty"`
	EnvironmentNote string                     `json:"environmentNote,omitempty"`
	Tags            []string                   `json:"tags,omitempty"`
	ContactEmail    string                     `json:"contactEmail,omitempty"`
	Photos          []adminSurveyPhotoResponse `json:"photos,omitempty"`
	HelpfulCount    int                        `json:"helpfulCount"`
	CreatedAt       time.Time                  `json:"createdAt"`
	UpdatedAt       time.Time                  `json:"updatedAt"`
}

type adminReviewListResponse struct {
	Items []adminReviewResponse `json:"items"`
}

type adminStoreResponse struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	BranchName      string               `json:"branchName,omitempty"`
	GroupName       string               `json:"groupName,omitempty"`
	Prefecture      string               `json:"prefecture,omitempty"`
	Area            string               `json:"area,omitempty"`
	Genre           string               `json:"genre,omitempty"`
	Industries      []string             `json:"industries,omitempty"`
	EmploymentTypes []string             `json:"employmentTypes,omitempty"`
	BusinessHours   string               `json:"businessHours,omitempty"`
	PricePerHour    int                  `json:"pricePerHour,omitempty"`
	PriceRange      string               `json:"priceRange,omitempty"`
	AverageEarning  int                  `json:"averageEarning,omitempty"`
	Tags            []string             `json:"tags,omitempty"`
	HomepageURL     string               `json:"homepageUrl,omitempty"`
	SNS             adminStoreSNSPayload `json:"sns"`
	PhotoURLs       []string             `json:"photoUrls,omitempty"`
	Description     string               `json:"description,omitempty"`
	ReviewCount     int                  `json:"reviewCount"`
	LastReviewedAt  *time.Time           `json:"lastReviewedAt,omitempty"`
}

type adminStoreSNSPayload struct {
	Twitter   string `json:"twitter,omitempty"`
	Line      string `json:"line,omitempty"`
	Instagram string `json:"instagram,omitempty"`
	TikTok    string `json:"tiktok,omitempty"`
	Official  string `json:"official,omitempty"`
}

type adminSurveyPhotoResponse struct {
	ID          string    `json:"id"`
	StoredPath  string    `json:"storedPath,omitempty"`
	PublicURL   string    `json:"publicUrl,omitempty"`
	ContentType string    `json:"contentType,omitempty"`
	UploadedAt  time.Time `json:"uploadedAt"`
}

type adminStoreCreateRequest struct {
	Name            string               `json:"name"`
	BranchName      string               `json:"branchName"`
	GroupName       string               `json:"groupName"`
	Prefecture      string               `json:"prefecture"`
	Area            string               `json:"area"`
	Genre           string               `json:"genre"`
	Industries      []string             `json:"industries"`
	EmploymentTypes []string             `json:"employmentTypes"`
	PricePerHour    int                  `json:"pricePerHour"`
	PriceRange      string               `json:"priceRange"`
	AverageEarning  int                  `json:"averageEarning"`
	BusinessHours   string               `json:"businessHours"`
	Tags            []string             `json:"tags"`
	HomepageURL     string               `json:"homepageUrl"`
	SNS             adminStoreSNSPayload `json:"sns"`
	PhotoURLs       []string             `json:"photoUrls"`
	Description     string               `json:"description"`
}

type adminStoreCreateResponse struct {
	Store   adminStoreResponse `json:"store"`
	Created bool               `json:"created"`
}

type adminStoreReviewCreateRequest struct {
	VisitedAt      string  `json:"visitedAt"`
	Age            int     `json:"age"`
	SpecScore      int     `json:"specScore"`
	WaitTimeHours  int     `json:"waitTimeHours"`
	AverageEarning int     `json:"averageEarning"`
	Comment        string  `json:"comment"`
	Rating         float64 `json:"rating"`
	IndustryCode   string  `json:"industryCode"`
	ContactEmail   string  `json:"contactEmail,omitempty"`
}

type updateReviewContentRequest struct {
	StoreID        *string  `json:"storeId"`
	StoreName      *string  `json:"storeName"`
	BranchName     *string  `json:"branchName"`
	Prefecture     *string  `json:"prefecture"`
	Category       *string  `json:"category"`
	VisitedAt      *string  `json:"visitedAt"`
	Age            *int     `json:"age"`
	SpecScore      *int     `json:"specScore"`
	WaitTimeHours  *int     `json:"waitTimeHours"`
	AverageEarning *int     `json:"averageEarning"`
	Comment        *string  `json:"comment"`
	Rating         *float64 `json:"rating"`
	ContactEmail   *string  `json:"contactEmail"`
}

func adminStoreDomainToResponse(store admindomain.Store) adminStoreResponse {
	return adminStoreResponse{
		ID:              store.ID,
		Name:            store.Name,
		BranchName:      strings.TrimSpace(store.BranchName),
		GroupName:       store.GroupName,
		Prefecture:      store.Prefecture,
		Area:            store.Area,
		Genre:           store.Genre,
		Industries:      append([]string{}, store.Industries...),
		EmploymentTypes: append([]string{}, store.EmploymentTypes...),
		BusinessHours:   store.BusinessHours,
		PricePerHour:    store.PricePerHour,
		PriceRange:      store.PriceRange,
		AverageEarning:  store.AverageEarning,
		Tags:            append([]string{}, store.Tags...),
		HomepageURL:     store.HomepageURL,
		SNS: adminStoreSNSPayload{
			Twitter:   store.SNS.Twitter,
			Line:      store.SNS.Line,
			Instagram: store.SNS.Instagram,
			TikTok:    store.SNS.TikTok,
			Official:  store.SNS.Official,
		},
		PhotoURLs:      append([]string{}, store.PhotoURLs...),
		Description:    store.Description,
		ReviewCount:    store.ReviewCount,
		LastReviewedAt: store.LastReviewedAt,
	}
}
