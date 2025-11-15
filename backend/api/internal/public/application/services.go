package application

import (
	context "context"
	"time"

	"github.com/sngm3741/makoto-club-services/api/internal/public/domain"
)

// StoreRepository abstracts read access to stores.
// StoreRepository は Public コンテキストで店舗を読み取るためのポート。
type StoreRepository interface {
	Find(ctx context.Context, filter StoreFilter, paging Paging) ([]domain.Store, error)
	FindByID(ctx context.Context, id string) (*domain.Store, error)
}

// SurveyRepository handles survey reads/writes.
// SurveyRepository は Public コンテキストのアンケート読み取りを提供するポート。
type SurveyRepository interface {
	Find(ctx context.Context, filter SurveyFilter, paging Paging) ([]domain.Survey, error)
	FindByID(ctx context.Context, id string) (*domain.Survey, error)
	Create(ctx context.Context, survey *domain.Survey) error
	IncrementHelpful(ctx context.Context, surveyID, voterID string, inc bool) (int, error)
}

// HelpfulVoteRepository enforces voter toggles.
type HelpfulVoteRepository interface {
	Toggle(ctx context.Context, surveyID, voterID string, desiredState bool) (bool, error)
}

// StoreFilter expresses search criteria for stores.
type StoreFilter struct {
	Prefecture string
	Genre      string
	Keyword    string
	Tags       []string
}

// SurveyFilter expresses search criteria for surveys.
type SurveyFilter struct {
	StoreID    string
	Prefecture string
	Genre      string
	Keyword    string
	Tags       []string
	StoreName  string
}

// Paging controls pagination.
type Paging struct {
	Page  int
	Limit int
	Sort  string
}

// StoreQueryService describes read use-cases.
// StoreQueryService は店舗に関するユースケースを提供するリーダーモデル。
type StoreQueryService interface {
	List(ctx context.Context, filter StoreFilter, paging Paging) ([]domain.Store, error)
	Detail(ctx context.Context, id string) (*domain.Store, error)
}

// SurveyQueryService describes survey read use-cases.
// SurveyQueryService はアンケート参照ユースケースを提供するリーダーモデル。
type SurveyQueryService interface {
	List(ctx context.Context, filter SurveyFilter, paging Paging) ([]domain.Survey, error)
	Detail(ctx context.Context, id string) (*domain.Survey, error)
}

// SurveyCommandService handles writing use-cases.
type SurveyCommandService interface {
	Submit(ctx context.Context, cmd SubmitSurveyCommand) (*domain.Survey, error)
	ToggleHelpful(ctx context.Context, surveyID, voterID string, desiredState bool) (int, error)
}

// SubmitSurveyCommand captures anonymous input.
type SubmitSurveyCommand struct {
	StoreID         string
	StoreName       string
	BranchName      string
	Prefecture      string
	Area            string
	Industries      []string
	Period          string
	Age             *int
	SpecScore       *int
	WaitTime        *int
	EmploymentType  string
	AverageEarning  *int
	CustomerNote    string
	StaffNote       string
	EnvironmentNote string
	Comment         string
	Rating          float64
	ContactEmail    string
	Tags            []string
	Photos          []domain.SurveyPhoto
}

func NewSurveyCommandService(repo SurveyRepository) SurveyCommandService {
	return &surveyCommandService{repo: repo}
}

type surveyCommandService struct {
	repo SurveyRepository
}

func (s *surveyCommandService) Submit(ctx context.Context, cmd SubmitSurveyCommand) (*domain.Survey, error) {
	now := time.Now().UTC()
	survey := &domain.Survey{
		StoreID:         cmd.StoreID,
		StoreName:       cmd.StoreName,
		BranchName:      cmd.BranchName,
		Prefecture:      cmd.Prefecture,
		Area:            cmd.Area,
		Industries:      append([]string{}, cmd.Industries...),
		Period:          cmd.Period,
		Age:             cmd.Age,
		SpecScore:       cmd.SpecScore,
		WaitTime:        cmd.WaitTime,
		EmploymentType:  cmd.EmploymentType,
		AverageEarning:  cmd.AverageEarning,
		CustomerNote:    cmd.CustomerNote,
		StaffNote:       cmd.StaffNote,
		EnvironmentNote: cmd.EnvironmentNote,
		Comment:         cmd.Comment,
		Rating:          cmd.Rating,
		ContactEmail:    cmd.ContactEmail,
		Tags:            append([]string{}, cmd.Tags...),
		Photos:          append([]domain.SurveyPhoto{}, cmd.Photos...),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return survey, s.repo.Create(ctx, survey)
}

func (s *surveyCommandService) ToggleHelpful(ctx context.Context, surveyID, voterID string, desiredState bool) (int, error) {
	return s.repo.IncrementHelpful(ctx, surveyID, voterID, desiredState)
}
