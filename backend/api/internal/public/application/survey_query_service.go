package application

import (
	"context"
	"sort"

	"github.com/sngm3741/makoto-club-services/api/internal/public/domain"
)

// surveyQueryService implements SurveyQueryService.
type surveyQueryService struct {
	repo SurveyRepository
}

// NewSurveyQueryService creates a new SurveyQueryService.
func NewSurveyQueryService(repo SurveyRepository) SurveyQueryService {
	return &surveyQueryService{repo: repo}
}

func (s *surveyQueryService) List(ctx context.Context, filter SurveyFilter, paging Paging) ([]domain.Survey, error) {
	surveys, err := s.repo.Find(ctx, filter, paging)
	if err != nil {
		return nil, err
	}
	sortSurveys(surveys, paging.Sort)
	return surveys, nil
}

func (s *surveyQueryService) Detail(ctx context.Context, id string) (*domain.Survey, error) {
	return s.repo.FindByID(ctx, id)
}

func sortSurveys(surveys []domain.Survey, sortKey string) {
	switch sortKey {
	case "helpful":
		sort.SliceStable(surveys, func(i, j int) bool {
			if surveys[i].HelpfulCount == surveys[j].HelpfulCount {
				return surveys[i].CreatedAt.After(surveys[j].CreatedAt)
			}
			return surveys[i].HelpfulCount > surveys[j].HelpfulCount
		})
	case "earning":
		sort.SliceStable(surveys, func(i, j int) bool {
			iEarning := valueOrZero(surveys[i].AverageEarning)
			jEarning := valueOrZero(surveys[j].AverageEarning)
			if iEarning == jEarning {
				return surveys[i].CreatedAt.After(surveys[j].CreatedAt)
			}
			return iEarning > jEarning
		})
	default:
		sort.SliceStable(surveys, func(i, j int) bool {
			return surveys[i].CreatedAt.After(surveys[j].CreatedAt)
		})
	}
}

func valueOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
