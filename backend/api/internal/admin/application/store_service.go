package application

import (
	"context"
	"fmt"
	"strings"

	admindomain "github.com/sngm3741/makoto-club-services/api/internal/admin/domain"
)

const maxStorePhotoCount = 10

// storeService は Admin コンテキストの店舗ユースケースを実装する。
type storeService struct {
	repo StoreRepository
}

// NewStoreService は StoreService 実装を生成する。
func NewStoreService(repo StoreRepository) StoreService {
	return &storeService{repo: repo}
}

// List は検索条件とページングに従って店舗一覧を返す。
func (s *storeService) List(ctx context.Context, filter StoreFilter, paging Paging) ([]admindomain.Store, error) {
	return s.repo.Find(ctx, filter, paging)
}

// Detail は店舗IDに紐づく集約を取得する。
func (s *storeService) Detail(ctx context.Context, id string) (*admindomain.Store, error) {
	return s.repo.FindByID(ctx, id)
}

// Create は店舗を新規登録し、ドメインオブジェクトを返す。
func (s *storeService) Create(ctx context.Context, cmd UpsertStoreCommand) (*admindomain.Store, error) {
	store, err := s.buildStore("", cmd)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, store); err != nil {
		return nil, err
	}
	return store, nil
}

// Update は既存店舗をコマンドの内容で更新する。
func (s *storeService) Update(ctx context.Context, id string, cmd UpsertStoreCommand) (*admindomain.Store, error) {
	store, err := s.buildStore(id, cmd)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, store); err != nil {
		return nil, err
	}
	return store, nil
}

// buildStore はコマンドからドメインの Store 集約を構築する。
func (s *storeService) buildStore(id string, cmd UpsertStoreCommand) (*admindomain.Store, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, fmt.Errorf("store name is required")
	}
	pref, err := admindomain.NewPrefecture(cmd.Prefecture)
	if err != nil {
		return nil, err
	}
	industries, err := admindomain.NewIndustryList(cmd.Industries)
	if err != nil {
		return nil, err
	}
	employmentTypes, err := admindomain.NewEmploymentTypeList(cmd.EmploymentTypes)
	if err != nil {
		return nil, err
	}
	pricePerHour, err := admindomain.NewMoney(cmd.PricePerHour)
	if err != nil {
		return nil, err
	}
	avgEarning, err := admindomain.NewMoney(cmd.AverageEarning)
	if err != nil {
		return nil, err
	}
	tags, err := admindomain.NewTagList(cmd.Tags)
	if err != nil {
		return nil, err
	}
	homepage, err := admindomain.NewURL(cmd.HomepageURL)
	if err != nil {
		return nil, err
	}
	photos, err := admindomain.NewPhotoURLList(cmd.PhotoURLs, maxStorePhotoCount)
	if err != nil {
		return nil, err
	}
	sns, err := admindomain.NewSNSLinks(cmd.SNS.Twitter, cmd.SNS.Line, cmd.SNS.Instagram, cmd.SNS.TikTok, cmd.SNS.Official)
	if err != nil {
		return nil, err
	}

	return &admindomain.Store{
		ID:              id,
		Name:            name,
		BranchName:      strings.TrimSpace(cmd.BranchName),
		GroupName:       strings.TrimSpace(cmd.GroupName),
		Prefecture:      pref,
		Area:            strings.TrimSpace(cmd.Area),
		Genre:           strings.TrimSpace(cmd.Genre),
		Industries:      industries,
		EmploymentTypes: employmentTypes,
		PricePerHour:    pricePerHour,
		PriceRange:      strings.TrimSpace(cmd.PriceRange),
		AverageEarning:  avgEarning,
		BusinessHours:   strings.TrimSpace(cmd.BusinessHours),
		Tags:            tags,
		HomepageURL:     homepage,
		SNS:             sns,
		PhotoURLs:       photos,
		Description:     strings.TrimSpace(cmd.Description),
	}, nil
}
