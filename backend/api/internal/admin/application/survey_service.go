package application

import (
	"context"
	"errors"
	"time"

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
	survey, err := buildSurveyFromCommand("", cmd)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	survey.CreatedAt = now
	survey.UpdatedAt = now
	if err := s.repo.Create(ctx, survey); err != nil {
		return nil, err
	}
	return survey, nil
}

func (s *surveyService) Update(ctx context.Context, id string, cmd UpsertSurveyCommand) (*admindomain.Survey, error) {
	survey, err := buildSurveyFromCommand(id, cmd)
	if err != nil {
		return nil, err
	}
	survey.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, survey); err != nil {
		return nil, err
	}
	return survey, nil
}

func buildSurveyFromCommand(id string, cmd UpsertSurveyCommand) (*admindomain.Survey, error) {
	industries := append([]string{}, cmd.Industries...)
	if len(industries) == 0 {
		return nil, errors.New("industries must not be empty")
	}
	pref, err := admindomain.NewPrefecture(cmd.Prefecture)
	if err != nil {
		return nil, err
	}
	industryList, err := admindomain.NewIndustryList(cmd.Industries)
	if err != nil {
		return nil, err
	}
	email, err := admindomain.NewEmail(cmd.ContactEmail)
	if err != nil {
		return nil, err
	}
	tags, err := admindomain.NewTagList(cmd.Tags)
	if err != nil {
		return nil, err
	}
	rating, err := admindomain.NewRating(normalizeRating(cmd.Rating))
	if err != nil {
		return nil, err
	}
	photos, err := mapSurveyPhotoCommands(cmd.Photos)
	if err != nil {
		return nil, err
	}

	waitTime := normalizeWaitTime(cmd.WaitTime, cmd.WaitTimeHours)

	return &admindomain.Survey{
		ID:              id,
		StoreID:         cmd.StoreID,
		StoreName:       cmd.StoreName,
		BranchName:      cmd.BranchName,
		Prefecture:      pref,
		Area:            cmd.Area,
		Industries:      industryList,
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
		ContactEmail:    email,
		Tags:            tags,
		Photos:          photos,
		Rating:          rating,
		HelpfulCount:    cmd.HelpfulCount,
	}, nil
}

func mapSurveyPhotoCommands(inputs []SurveyPhotoCommand) ([]admindomain.SurveyPhoto, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	photos := make([]admindomain.SurveyPhoto, 0, len(inputs))
	for _, input := range inputs {
		publicURL, err := admindomain.NewPhotoURL(input.PublicURL)
		if err != nil {
			return nil, err
		}
		photos = append(photos, admindomain.SurveyPhoto{
			ID:          input.ID,
			StoredPath:  input.StoredPath,
			PublicURL:   publicURL,
			ContentType: input.ContentType,
			UploadedAt:  input.UploadedAt,
		})
	}
	return photos, nil
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
