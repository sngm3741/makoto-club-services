package application

import (
	"context"
	"time"

	admindomain "github.com/sngm3741/makoto-club-services/api/internal/admin/domain"
)

// StoreRepository exposes admin operations on stores.
type StoreRepository interface {
	Find(ctx context.Context, filter StoreFilter, paging Paging) ([]admindomain.Store, error)
	FindByID(ctx context.Context, id string) (*admindomain.Store, error)
	Create(ctx context.Context, store *admindomain.Store) error
	Update(ctx context.Context, store *admindomain.Store) error
}

// SurveyRepository exposes CRUD for admin surveys.
type SurveyRepository interface {
	Find(ctx context.Context, filter SurveyFilter, paging Paging) ([]admindomain.Survey, error)
	FindByID(ctx context.Context, id string) (*admindomain.Survey, error)
	Create(ctx context.Context, survey *admindomain.Survey) error
	Update(ctx context.Context, survey *admindomain.Survey) error
}

// InboundRepository allows listing inbound surveys.
type InboundRepository interface {
	List(ctx context.Context, paging Paging) ([]admindomain.InboundSurvey, error)
	Delete(ctx context.Context, id string) error
}

// StoreFilter expresses admin search criteria.
type StoreFilter struct {
	Prefecture string
	Genre      string
	Keyword    string
	Limit      int
}

// SurveyFilter expresses admin search criteria.
type SurveyFilter struct {
	StoreID string
	Keyword string
}

// Paging controls pagination.
type Paging struct {
	Page  int
	Limit int
	Sort  string
}

// StoreService describes admin store use-cases.
type StoreService interface {
	List(ctx context.Context, filter StoreFilter, paging Paging) ([]admindomain.Store, error)
	Detail(ctx context.Context, id string) (*admindomain.Store, error)
	Create(ctx context.Context, cmd UpsertStoreCommand) (*admindomain.Store, error)
	Update(ctx context.Context, id string, cmd UpsertStoreCommand) (*admindomain.Store, error)
}

// SurveyService describes admin survey use-cases.
type SurveyService interface {
	List(ctx context.Context, filter SurveyFilter, paging Paging) ([]admindomain.Survey, error)
	Detail(ctx context.Context, id string) (*admindomain.Survey, error)
	Create(ctx context.Context, cmd UpsertSurveyCommand) (*admindomain.Survey, error)
	Update(ctx context.Context, id string, cmd UpsertSurveyCommand) (*admindomain.Survey, error)
}

// UpsertStoreCommand contains inputs for creating/updating stores.
type UpsertStoreCommand struct {
	Name            string
	BranchName      string
	GroupName       string
	Prefecture      string
	Area            string
	Genre           string
	Industries      []string
	EmploymentTypes []string
	PricePerHour    int
	PriceRange      string
	AverageEarning  int
	BusinessHours   string
	Tags            []string
	HomepageURL     string
	SNS             StoreSNSCommand
	PhotoURLs       []string
	Description     string
}

// StoreSNSCommand holds SNS links for a store.
type StoreSNSCommand struct {
	Twitter   string
	Line      string
	Instagram string
	TikTok    string
	Official  string
}

// UpsertSurveyCommand contains inputs for survey CRUD.
type UpsertSurveyCommand struct {
	StoreID         string
	StoreName       string
	BranchName      string
	Prefecture      string
	Area            string
	Industries      []string
	Genre           string
	Period          string
	Age             *int
	SpecScore       *int
	WaitTime        *int
	WaitTimeHours   *int
	EmploymentType  string
	AverageEarning  *int
	CustomerNote    string
	StaffNote       string
	EnvironmentNote string
	Comment         string
	ContactEmail    string
	Tags            []string
	Photos          []SurveyPhotoCommand
	Rating          float64
	HelpfulCount    int
}

// SurveyPhotoCommand represents uploaded photo metadata.
type SurveyPhotoCommand struct {
	ID          string
	StoredPath  string
	PublicURL   string
	ContentType string
	UploadedAt  time.Time
}
