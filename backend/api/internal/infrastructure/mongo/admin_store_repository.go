package mongo

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/sngm3741/makoto-club-services/api/internal/admin/application"
	admindomain "github.com/sngm3741/makoto-club-services/api/internal/admin/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AdminStoreRepository implements application.StoreRepository.
type AdminStoreRepository struct {
	collection *mongo.Collection
}

func NewAdminStoreRepository(db *mongo.Database, collection string) *AdminStoreRepository {
	return &AdminStoreRepository{collection: db.Collection(collection)}
}

func (r *AdminStoreRepository) Find(ctx context.Context, filter application.StoreFilter, paging application.Paging) ([]admindomain.Store, error) {
	mongoFilter := bson.M{}
	clauses := make([]bson.M, 0)
	if filter.Prefecture != "" {
		clauses = append(clauses, bson.M{"prefecture": filter.Prefecture})
	}
	if filter.Genre != "" {
		clauses = append(clauses, bson.M{"industries": filter.Genre})
	}
	if filter.Keyword != "" {
		pattern := regexp.QuoteMeta(filter.Keyword)
		regex := primitive.Regex{Pattern: pattern, Options: "i"}
		clauses = append(clauses, bson.M{"$or": bson.A{
			bson.M{"name": regex},
			bson.M{"branchName": regex},
		}})
	}
	if len(clauses) == 1 {
		mongoFilter = clauses[0]
	} else if len(clauses) > 1 {
		mongoFilter["$and"] = clauses
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = paging.Limit
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	opts := options.Find().SetSort(bson.D{{Key: "stats.reviewCount", Value: -1}, {Key: "name", Value: 1}})
	opts.SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, mongoFilter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	stores := make([]admindomain.Store, 0)
	for cursor.Next(ctx) {
		var doc StoreDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		stores = append(stores, mapAdminStore(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return stores, nil
}

func (r *AdminStoreRepository) FindByID(ctx context.Context, id string) (*admindomain.Store, error) {
	objectID, err := primitive.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	var doc StoreDocument
	if err := r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&doc); err != nil {
		return nil, err
	}
	store := mapAdminStore(doc)
	return &store, nil
}

func (r *AdminStoreRepository) Create(ctx context.Context, store *admindomain.Store) error {
	filter := bson.M{
		"name":       strings.TrimSpace(store.Name),
		"branchName": strings.TrimSpace(store.BranchName),
	}
	if filter["branchName"] == "" {
		filter["branchName"] = nil
	}
	if err := r.collection.FindOne(ctx, filter).Err(); err == nil {
		return errors.New("store already exists")
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	payload := bson.M{
		"name":            store.Name,
		"branchName":      store.BranchName,
		"groupName":       store.GroupName,
		"prefecture":      store.Prefecture,
		"area":            store.Area,
		"genre":           store.Genre,
		"industries":      store.Industries,
		"employmentTypes": store.EmploymentTypes,
		"pricePerHour":    store.PricePerHour,
		"priceRange":      store.PriceRange,
		"averageEarning":  store.AverageEarning,
		"businessHours":   store.BusinessHours,
		"tags":            store.Tags,
		"homepageURL":     store.HomepageURL,
		"sns":             flattenAdminSNSLinks(store.SNS),
		"photoURLs":       store.PhotoURLs,
		"description":     store.Description,
		"stats":           StoreStatsDocument{},
		"createdAt":       time.Now().UTC(),
		"updatedAt":       time.Now().UTC(),
	}
	_, err := r.collection.InsertOne(ctx, payload)
	return err
}

func (r *AdminStoreRepository) Update(ctx context.Context, store *admindomain.Store) error {
	objectID, err := primitive.ObjectIDFromHex(strings.TrimSpace(store.ID))
	if err != nil {
		return err
	}
	update := bson.M{
		"name":            store.Name,
		"branchName":      store.BranchName,
		"groupName":       store.GroupName,
		"prefecture":      store.Prefecture,
		"area":            store.Area,
		"genre":           store.Genre,
		"industries":      store.Industries,
		"employmentTypes": store.EmploymentTypes,
		"pricePerHour":    store.PricePerHour,
		"priceRange":      store.PriceRange,
		"averageEarning":  store.AverageEarning,
		"businessHours":   store.BusinessHours,
		"tags":            store.Tags,
		"homepageURL":     store.HomepageURL,
		"sns":             flattenAdminSNSLinks(store.SNS),
		"photoURLs":       store.PhotoURLs,
		"description":     store.Description,
		"updatedAt":       time.Now().UTC(),
	}
	_, err = r.collection.UpdateByID(ctx, objectID, bson.M{"$set": update})
	return err
}

func mapAdminStore(doc StoreDocument) admindomain.Store {
	pref, _ := admindomain.NewPrefecture(doc.Prefecture)
	industries, _ := admindomain.NewIndustryList(doc.Industries)
	employment, _ := admindomain.NewEmploymentTypeList(doc.EmploymentTypes)
	tags, _ := admindomain.NewTagList(doc.Tags)
	homepage, _ := admindomain.NewURL(doc.HomepageURL)
	photos, _ := admindomain.NewPhotoURLList(doc.PhotoURLs, 0)
	price, _ := admindomain.NewMoney(doc.PricePerHour)
	avg, _ := admindomain.NewMoney(doc.AverageEarning)
	sns, _ := admindomain.NewSNSLinks(doc.SNS.Twitter, doc.SNS.Line, doc.SNS.Instagram, doc.SNS.TikTok, doc.SNS.Official)

	return admindomain.Store{
		ID:              doc.ID.Hex(),
		Name:            doc.Name,
		BranchName:      strings.TrimSpace(doc.BranchName),
		GroupName:       doc.GroupName,
		Prefecture:      pref,
		Area:            doc.Area,
		Genre:           doc.Genre,
		BusinessHours:   doc.BusinessHours,
		Industries:      industries,
		EmploymentTypes: employment,
		PricePerHour:    price,
		PriceRange:      doc.PriceRange,
		AverageEarning:  avg,
		Tags:            tags,
		HomepageURL:     homepage,
		SNS:             sns,
		PhotoURLs:       photos,
		Description:     doc.Description,
		ReviewCount:     doc.Stats.ReviewCount,
		LastReviewedAt:  doc.Stats.LastReviewedAt,
	}
}

func flattenAdminSNSLinks(links admindomain.SNSLinks) StoreSNSDocument {
	return StoreSNSDocument{
		Twitter:   links.Twitter.String(),
		Line:      links.Line.String(),
		Instagram: links.Instagram.String(),
		TikTok:    links.TikTok.String(),
		Official:  links.Official.String(),
	}
}
