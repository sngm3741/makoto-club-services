package mongo

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/sngm3741/makoto-club-services/api/internal/public/application"
	"github.com/sngm3741/makoto-club-services/api/internal/public/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// StoreRepository implements application.StoreRepository using MongoDB.
type StoreRepository struct {
	collection *mongo.Collection
}

// NewStoreRepository creates a new Mongo-backed store repository.
func NewStoreRepository(db *mongo.Database, collectionName string) *StoreRepository {
	return &StoreRepository{collection: db.Collection(collectionName)}
}

// Find returns store summaries filtered and paginated according to the provided criteria.
func (r *StoreRepository) Find(ctx context.Context, filter application.StoreFilter, paging application.Paging) ([]domain.Store, error) {
	mongoFilter := bson.M{
		"stats.reviewCount": bson.M{"$gt": 0},
	}
	if filter.Prefecture != "" {
		mongoFilter["prefecture"] = strings.TrimSpace(filter.Prefecture)
	}
	if filter.Genre != "" {
		mongoFilter["industries"] = strings.TrimSpace(filter.Genre)
	}
	if len(filter.Tags) > 0 {
		mongoFilter["tags"] = bson.M{"$all": filter.Tags}
	}
	if filter.Keyword != "" {
		mongoFilter["$or"] = []bson.M{
			{"name": bson.M{"$regex": filter.Keyword, "$options": "i"}},
			{"branchName": bson.M{"$regex": filter.Keyword, "$options": "i"}},
			{"area": bson.M{"$regex": filter.Keyword, "$options": "i"}},
		}
	}

	cursor, err := r.collection.Find(ctx, mongoFilter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	stores := make([]domain.Store, 0)
	for cursor.Next(ctx) {
		var doc StoreDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		stores = append(stores, mapStoreDocument(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	sortStores(stores, paging.Sort)
	return stores, nil
}

// FindByID returns a single store by its identifier.
func (r *StoreRepository) FindByID(ctx context.Context, id string) (*domain.Store, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var doc StoreDocument
	if err := r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&doc); err != nil {
		return nil, err
	}
	store := mapStoreDocument(doc)
	return &store, nil
}

func mapStoreDocument(doc StoreDocument) domain.Store {
	createdAt := time.Time{}
	if doc.CreatedAt != nil {
		createdAt = *doc.CreatedAt
	}
	updatedAt := time.Time{}
	if doc.UpdatedAt != nil {
		updatedAt = *doc.UpdatedAt
	}

	stats := domain.StoreStats{
		ReviewCount:    doc.Stats.ReviewCount,
		AvgRating:      doc.Stats.AvgRating,
		AvgEarning:     doc.Stats.AvgEarning,
		AvgWaitTime:    doc.Stats.AvgWaitTime,
		LastReviewedAt: doc.Stats.LastReviewedAt,
	}

	return domain.Store{
		ID:              doc.ID.Hex(),
		Name:            doc.Name,
		BranchName:      strings.TrimSpace(doc.BranchName),
		GroupName:       doc.GroupName,
		Prefecture:      doc.Prefecture,
		Area:            doc.Area,
		Genre:           doc.Genre,
		Industries:      append([]string{}, doc.Industries...),
		EmploymentTypes: append([]string{}, doc.EmploymentTypes...),
		PricePerHour:    doc.PricePerHour,
		PriceRange:      doc.PriceRange,
		AverageEarning:  doc.AverageEarning,
		BusinessHours:   doc.BusinessHours,
		Tags:            append([]string{}, doc.Tags...),
		HomepageURL:     doc.HomepageURL,
		SNS:             mapSNSDocument(doc.SNS),
		PhotoURLs:       append([]string{}, doc.PhotoURLs...),
		Description:     doc.Description,
		Stats:           stats,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}
}

func mapSNSDocument(doc StoreSNSDocument) domain.SNSLinks {
	return domain.SNSLinks{
		Twitter:   doc.Twitter,
		Line:      doc.Line,
		Instagram: doc.Instagram,
		TikTok:    doc.TikTok,
		Official:  doc.Official,
	}
}

func sortStores(stores []domain.Store, sortKey string) {
	switch sortKey {
	case "rating":
		sort.SliceStable(stores, func(i, j int) bool {
			return ptrFloat(stores[i].Stats.AvgRating) > ptrFloat(stores[j].Stats.AvgRating)
		})
	case "earning":
		sort.SliceStable(stores, func(i, j int) bool {
			return ptrFloat(stores[i].Stats.AvgEarning) > ptrFloat(stores[j].Stats.AvgEarning)
		})
	default:
		sort.SliceStable(stores, func(i, j int) bool {
			return stores[i].CreatedAt.After(stores[j].CreatedAt)
		})
	}
}

func ptrFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return math.Round(*v*10) / 10
}
