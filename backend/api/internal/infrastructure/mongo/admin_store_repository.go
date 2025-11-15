package mongo

import (
	"context"
	"errors"
	"fmt"
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

// AdminStoreRepository は管理者向け Store 集約の Mongo 実装。
type AdminStoreRepository struct {
	collection *mongo.Collection
}

// NewAdminStoreRepository は MongoDB コレクションを束縛した AdminStoreRepository を生成する。
func NewAdminStoreRepository(db *mongo.Database, collection string) *AdminStoreRepository {
	return &AdminStoreRepository{collection: db.Collection(collection)}
}

// Find は曖昧検索とページングをサポートした管理者用の店舗一覧を返す。
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
		store, err := mapAdminStore(doc)
		if err != nil {
			return nil, err
		}
		stores = append(stores, store)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return stores, nil
}

// FindByID は 16 進 ObjectID を受け取り単一店舗を VO 化して返す。
func (r *AdminStoreRepository) FindByID(ctx context.Context, id string) (*admindomain.Store, error) {
	objectID, err := primitive.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	var doc StoreDocument
	if err := r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&doc); err != nil {
		return nil, err
	}
	store, err := mapAdminStore(doc)
	if err != nil {
		return nil, err
	}
	return &store, nil
}

// Create は店舗名+支店名の重複チェックを行った上で Store を新規作成する。
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
	payload, err := buildStoreDocument(store, true)
	if err != nil {
		return err
	}
	_, err = r.collection.InsertOne(ctx, payload)
	return err
}

// Update は Store の ObjectID を用いて差し替えを行い、値オブジェクト経由で整形したデータのみを保存する。
func (r *AdminStoreRepository) Update(ctx context.Context, store *admindomain.Store) error {
	objectID, err := primitive.ObjectIDFromHex(strings.TrimSpace(store.ID))
	if err != nil {
		return err
	}
	update, err := buildStoreDocument(store, false)
	if err != nil {
		return err
	}
	_, err = r.collection.UpdateByID(ctx, objectID, bson.M{"$set": update})
	return err
}

// mapAdminStore は Mongo ドキュメントを Admin ドメインの Store に変換する。
func mapAdminStore(doc StoreDocument) (admindomain.Store, error) {
	pref, err := admindomain.NewPrefecture(doc.Prefecture)
	if err != nil {
		return admindomain.Store{}, err
	}
	industries, err := admindomain.NewIndustryList(doc.Industries)
	if err != nil {
		return admindomain.Store{}, err
	}
	employment, err := admindomain.NewEmploymentTypeList(doc.EmploymentTypes)
	if err != nil {
		return admindomain.Store{}, err
	}
	tags, err := admindomain.NewTagList(doc.Tags)
	if err != nil {
		return admindomain.Store{}, err
	}
	homepage, err := admindomain.NewURL(doc.HomepageURL)
	if err != nil {
		return admindomain.Store{}, err
	}
	photos, err := admindomain.NewPhotoURLList(doc.PhotoURLs, 0)
	if err != nil {
		return admindomain.Store{}, err
	}
	price, err := admindomain.NewMoney(doc.PricePerHour)
	if err != nil {
		return admindomain.Store{}, err
	}
	avg, err := admindomain.NewMoney(doc.AverageEarning)
	if err != nil {
		return admindomain.Store{}, err
	}
	sns, err := admindomain.NewSNSLinks(doc.SNS.Twitter, doc.SNS.Line, doc.SNS.Instagram, doc.SNS.TikTok, doc.SNS.Official)
	if err != nil {
		return admindomain.Store{}, err
	}

	store := admindomain.Store{
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
	if doc.CreatedAt != nil {
		store.CreatedAt = *doc.CreatedAt
	}
	if doc.UpdatedAt != nil {
		store.UpdatedAt = *doc.UpdatedAt
	}
	return store, nil
}

// buildStoreDocument は Store の値オブジェクト群を Mongo 用 BSON に展開する。
func buildStoreDocument(store *admindomain.Store, includeCreated bool) (bson.M, error) {
	if store == nil {
		return nil, fmt.Errorf("store payload is nil")
	}
	payload := bson.M{
		"name":            store.Name,
		"branchName":      store.BranchName,
		"groupName":       store.GroupName,
		"prefecture":      store.Prefecture.String(),
		"area":            store.Area,
		"genre":           store.Genre,
		"industries":      store.Industries.Strings(),
		"employmentTypes": store.EmploymentTypes.Strings(),
		"pricePerHour":    store.PricePerHour.Int(),
		"priceRange":      store.PriceRange,
		"averageEarning":  store.AverageEarning.Int(),
		"businessHours":   store.BusinessHours,
		"tags":            store.Tags.Strings(),
		"homepageURL":     store.HomepageURL.String(),
		"sns":             flattenAdminSNSLinks(store.SNS),
		"photoURLs":       store.PhotoURLs.Strings(),
		"description":     store.Description,
		"updatedAt":       time.Now().UTC(),
	}
	if includeCreated {
		payload["stats"] = StoreStatsDocument{}
		payload["createdAt"] = time.Now().UTC()
	}
	return payload, nil
}

// flattenAdminSNSLinks は SNSLinks VO を Mongo の埋め込みドキュメントにフラット化する。
func flattenAdminSNSLinks(links admindomain.SNSLinks) StoreSNSDocument {
	return StoreSNSDocument{
		Twitter:   links.Twitter.String(),
		Line:      links.Line.String(),
		Instagram: links.Instagram.String(),
		TikTok:    links.TikTok.String(),
		Official:  links.Official.String(),
	}
}
