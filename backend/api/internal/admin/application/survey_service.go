package application

import (
	"context"
	"errors"

	admindomain "github.com/sngm3741/makoto-club-services/api/internal/admin/domain"
)

type surveyService struct {
	repo SurveyRepository
}

func NewSurveyService(repo SurveyRepository) SurveyService {
	return &surveyService{repo: repo}
}

func (s *surveyService) List(ctx context.Context, filter SurveyFilter, paging Paging) ([]admindomain.Survey, error) {
	return s.repo.Find(ctx, filter, paging)
}

func (s *surveyService) Detail(ctx context.Context, id string) (*admindomain.Survey, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *surveyService) Create(ctx context.Context, cmd UpsertSurveyCommand) (*admindomain.Survey, error) {
	industries := append([]string{}, cmd.Industries...)
	if len(industries) == 0 {
		return nil, errors.New("industries must not be empty")
	}
	waitTime := normalizeWaitTime(cmd.WaitTime, cmd.WaitTimeHours)
	photos := mapSurveyPhotoCommands(cmd.Photos)

	survey := &admindomain.Survey{
		StoreID:         cmd.StoreID,
		StoreName:       cmd.StoreName,
		BranchName:      cmd.BranchName,
		Prefecture:      cmd.Prefecture,
		Area:            cmd.Area,
		Industries:      industries,
		Genre:           cmd.Genre,
		Period:          cmd.Period,
		Age:             cmd.Age,
		SpecScore:       cmd.SpecScore,
		WaitTime:        waitTime,
		EmploymentType:  cmd.EmploymentType,
		AverageEarning:  cmd.AverageEarning,
		CustomerNote:    cmd.CustomerNote,
		StaffNote:       cmd.StaffNote,
		EnvironmentNote: cmd.EnvironmentNote,
		Comment:         cmd.Comment,
		ContactEmail:    cmd.ContactEmail,
		Tags:            append([]string{}, cmd.Tags...),
		Photos:          photos,
		Rating:          normalizeRating(cmd.Rating),
		HelpfulCount:    cmd.HelpfulCount,
	}
	if err := s.repo.Create(ctx, survey); err != nil {
		return nil, err
	}
	return survey, nil
}

func (s *surveyService) Update(ctx context.Context, id string, cmd UpsertSurveyCommand) (*admindomain.Survey, error) {
	industries := append([]string{}, cmd.Industries...)
	if len(industries) == 0 {
		return nil, errors.New("industries must not be empty")
	}
	waitTime := normalizeWaitTime(cmd.WaitTime, cmd.WaitTimeHours)
	photos := mapSurveyPhotoCommands(cmd.Photos)

	survey := &admindomain.Survey{
		ID:              id,
		StoreID:         cmd.StoreID,
		StoreName:       cmd.StoreName,
		BranchName:      cmd.BranchName,
		Prefecture:      cmd.Prefecture,
		Area:            cmd.Area,
		Industries:      industries,
		Genre:           cmd.Genre,
		Period:          cmd.Period,
		Age:             cmd.Age,
		SpecScore:       cmd.SpecScore,
		WaitTime:        waitTime,
		EmploymentType:  cmd.EmploymentType,
		AverageEarning:  cmd.AverageEarning,
		CustomerNote:    cmd.CustomerNote,
		StaffNote:       cmd.StaffNote,
		EnvironmentNote: cmd.EnvironmentNote,
		Comment:         cmd.Comment,
		ContactEmail:    cmd.ContactEmail,
		Tags:            append([]string{}, cmd.Tags...),
		Photos:          photos,
		Rating:          normalizeRating(cmd.Rating),
		HelpfulCount:    cmd.HelpfulCount,
	}
	if err := s.repo.Update(ctx, survey); err != nil {
		return nil, err
	}
	return survey, nil
}

func mapSurveyPhotoCommands(inputs []SurveyPhotoCommand) []admindomain.SurveyPhoto {
	if len(inputs) == 0 {
		return nil
	}
	photos := make([]admindomain.SurveyPhoto, 0, len(inputs))
	for _, input := range inputs {
		photos = append(photos, admindomain.SurveyPhoto{
			ID:          input.ID,
			StoredPath:  input.StoredPath,
			PublicURL:   input.PublicURL,
			ContentType: input.ContentType,
			UploadedAt:  input.UploadedAt,
		})
	}
	return photos
}

func normalizeWaitTime(minutes *int, hours *int) *int {
	if minutes != nil {
		value := *minutes
		return &value
	}
	if hours != nil {
		value := *hours * 60
		return &value
	}
	return nil
}

func normalizeRating(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return value
}
