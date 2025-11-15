package application

import (
	"context"

	admindomain "github.com/sngm3741/makoto-club-services/api/internal/admin/domain"
)

// storeService implements StoreService.
type storeService struct {
	repo StoreRepository
}

func NewStoreService(repo StoreRepository) StoreService {
	return &storeService{repo: repo}
}

func (s *storeService) List(ctx context.Context, filter StoreFilter, paging Paging) ([]admindomain.Store, error) {
	return s.repo.Find(ctx, filter, paging)
}

func (s *storeService) Detail(ctx context.Context, id string) (*admindomain.Store, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *storeService) Create(ctx context.Context, cmd UpsertStoreCommand) (*admindomain.Store, error) {
	store := &admindomain.Store{
		Name:            cmd.Name,
		BranchName:      cmd.BranchName,
		GroupName:       cmd.GroupName,
		Prefecture:      cmd.Prefecture,
		Area:            cmd.Area,
		Genre:           cmd.Genre,
		Industries:      append([]string{}, cmd.Industries...),
		EmploymentTypes: append([]string{}, cmd.EmploymentTypes...),
		PricePerHour:    cmd.PricePerHour,
		PriceRange:      cmd.PriceRange,
		AverageEarning:  cmd.AverageEarning,
		BusinessHours:   cmd.BusinessHours,
		Tags:            append([]string{}, cmd.Tags...),
		HomepageURL:     cmd.HomepageURL,
		SNS: admindomain.SNSLinks{
			Twitter:   cmd.SNS.Twitter,
			Line:      cmd.SNS.Line,
			Instagram: cmd.SNS.Instagram,
			TikTok:    cmd.SNS.TikTok,
			Official:  cmd.SNS.Official,
		},
		PhotoURLs:   append([]string{}, cmd.PhotoURLs...),
		Description: cmd.Description,
	}
	if err := s.repo.Create(ctx, store); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *storeService) Update(ctx context.Context, id string, cmd UpsertStoreCommand) (*admindomain.Store, error) {
	store := &admindomain.Store{
		ID:              id,
		Name:            cmd.Name,
		BranchName:      cmd.BranchName,
		GroupName:       cmd.GroupName,
		Prefecture:      cmd.Prefecture,
		Area:            cmd.Area,
		Genre:           cmd.Genre,
		Industries:      append([]string{}, cmd.Industries...),
		EmploymentTypes: append([]string{}, cmd.EmploymentTypes...),
		PricePerHour:    cmd.PricePerHour,
		PriceRange:      cmd.PriceRange,
		AverageEarning:  cmd.AverageEarning,
		BusinessHours:   cmd.BusinessHours,
		Tags:            append([]string{}, cmd.Tags...),
		HomepageURL:     cmd.HomepageURL,
		SNS: admindomain.SNSLinks{
			Twitter:   cmd.SNS.Twitter,
			Line:      cmd.SNS.Line,
			Instagram: cmd.SNS.Instagram,
			TikTok:    cmd.SNS.TikTok,
			Official:  cmd.SNS.Official,
		},
		PhotoURLs:   append([]string{}, cmd.PhotoURLs...),
		Description: cmd.Description,
	}
	if err := s.repo.Update(ctx, store); err != nil {
		return nil, err
	}
	return store, nil
}
