package mongo

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	adminapp "github.com/sngm3741/makoto-club-services/api/internal/admin/application"
	admindomain "github.com/sngm3741/makoto-club-services/api/internal/admin/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AdminSurveyRepository は管理者向けアンケート集約を MongoDB 経由で扱うリポジトリ。
type AdminSurveyRepository struct {
	reviews *mongo.Collection
	stores  *mongo.Collection
}

// NewAdminSurveyRepository はレビュー・店舗の 2 コレクションを束縛したリポジトリを生成する。
func NewAdminSurveyRepository(db *mongo.Database, reviewCollection, storeCollection string) *AdminSurveyRepository {
	return &AdminSurveyRepository{
		reviews: db.Collection(reviewCollection),
		stores:  db.Collection(storeCollection),
	}
}

// Find は Store ID/キーワード条件を Mongo クエリへ変換し、管理画面一覧を返す。
func (r *AdminSurveyRepository) Find(ctx context.Context, filter adminapp.SurveyFilter, paging adminapp.Paging) ([]admindomain.Survey, error) {
	mongoFilter := bson.M{}
	if storeID := strings.TrimSpace(filter.StoreID); storeID != "" {
		id, err := primitive.ObjectIDFromHex(storeID)
		if err != nil {
			return nil, err
		}
		mongoFilter["storeId"] = id
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		pattern := primitive.Regex{Pattern: regexp.QuoteMeta(keyword), Options: "i"}
		mongoFilter["$or"] = bson.A{
			bson.M{"storeName": pattern},
			bson.M{"branchName": pattern},
			bson.M{"comment": pattern},
			bson.M{"customerNote": pattern},
		}
	}

	findOpts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	if paging.Limit > 0 {
		findOpts.SetLimit(int64(paging.Limit))
		if paging.Page > 1 {
			skip := int64((paging.Page - 1) * paging.Limit)
			findOpts.SetSkip(skip)
		}
	}

	cursor, err := r.reviews.Find(ctx, mongoFilter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	surveys := make([]admindomain.Survey, 0)
	for cursor.Next(ctx) {
		var doc ReviewDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		survey, err := mapAdminSurveyDocument(doc)
		if err != nil {
			return nil, err
		}
		surveys = append(surveys, survey)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return surveys, nil
}

// FindByID はアンケート ID を ObjectID 化して単一エンティティを復元する。
func (r *AdminSurveyRepository) FindByID(ctx context.Context, id string) (*admindomain.Survey, error) {
	objectID, err := primitive.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	var doc ReviewDocument
	if err := r.reviews.FindOne(ctx, bson.M{"_id": objectID}).Decode(&doc); err != nil {
		return nil, err
	}
	survey, err := mapAdminSurveyDocument(doc)
	if err != nil {
		return nil, err
	}
	return &survey, nil
}

// Create はドメインアンケートを Mongo ドキュメントへ変換し、新規登録と店舗統計の再計算を行う。
func (r *AdminSurveyRepository) Create(ctx context.Context, survey *admindomain.Survey) error {
	if survey == nil {
		return errors.New("survey payload is nil")
	}
	doc, err := mapDomainSurveyToDocument(survey)
	if err != nil {
		return err
	}
	doc.ID = primitive.NewObjectID()
	survey.ID = doc.ID.Hex()
	if _, err := r.reviews.InsertOne(ctx, doc); err != nil {
		return err
	}
	return r.recalculateStoreStats(ctx, doc.StoreID)
}

// Update はアンケートの差し替え更新と統計再計算までを一括で担う。
func (r *AdminSurveyRepository) Update(ctx context.Context, survey *admindomain.Survey) error {
	if survey == nil {
		return errors.New("survey payload is nil")
	}
	if strings.TrimSpace(survey.ID) == "" {
		return errors.New("survey id is required")
	}
	doc, err := mapDomainSurveyToDocument(survey)
	if err != nil {
		return err
	}
	objectID, err := primitive.ObjectIDFromHex(survey.ID)
	if err != nil {
		return err
	}
	update := buildSurveyUpdatePayload(doc)
	if _, err := r.reviews.UpdateByID(ctx, objectID, bson.M{"$set": update}); err != nil {
		return err
	}
	return r.recalculateStoreStats(ctx, doc.StoreID)
}

// mapAdminSurveyDocument は Mongo レビュー文書を Admin ドメイン Survey へ変換する。
func mapAdminSurveyDocument(doc ReviewDocument) (admindomain.Survey, error) {
	pref, err := admindomain.NewPrefecture(doc.Prefecture)
	if err != nil {
		return admindomain.Survey{}, err
	}
	industries, err := admindomain.NewIndustryList(doc.Industries)
	if err != nil {
		return admindomain.Survey{}, err
	}
	email, err := admindomain.NewEmail(doc.ContactEmail)
	if err != nil {
		return admindomain.Survey{}, err
	}
	tags, err := admindomain.NewTagList(doc.Tags)
	if err != nil {
		return admindomain.Survey{}, err
	}
	rating, err := admindomain.NewRating(doc.Rating)
	if err != nil {
		return admindomain.Survey{}, err
	}
	photos, err := mapSurveyPhotoDocumentsToDomain(doc.Photos)
	if err != nil {
		return admindomain.Survey{}, err
	}

	return admindomain.Survey{
		ID:              doc.ID.Hex(),
		StoreID:         doc.StoreID.Hex(),
		StoreName:       doc.StoreName,
		BranchName:      doc.BranchName,
		Prefecture:      pref,
		Area:            doc.Area,
		Industries:      industries,
		Genre:           doc.Genre,
		Period:          doc.Period,
		Age:             doc.Age,
		SpecScore:       doc.SpecScore,
		WaitTime:        doc.WaitTimeMinutes,
		EmploymentType:  doc.EmploymentType,
		AverageEarning:  doc.AverageEarning,
		CustomerNote:    doc.CustomerNote,
		StaffNote:       doc.StaffNote,
		EnvironmentNote: doc.EnvironmentNote,
		Comment:         doc.Comment,
		ContactEmail:    email,
		Rating:          rating,
		HelpfulCount:    doc.HelpfulCount,
		Tags:            tags,
		Photos:          photos,
		CreatedAt:       doc.CreatedAt,
		UpdatedAt:       doc.UpdatedAt,
	}, nil
}

// mapDomainSurveyToDocument はドメイン Survey を Mongo 保存形式に射影する。
func mapDomainSurveyToDocument(survey *admindomain.Survey) (ReviewDocument, error) {
	storeID, err := primitive.ObjectIDFromHex(strings.TrimSpace(survey.StoreID))
	if err != nil {
		return ReviewDocument{}, err
	}
	photoDocs, err := mapSurveyPhotosToDocuments(survey.Photos)
	if err != nil {
		return ReviewDocument{}, err
	}

	createdAt := survey.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	updatedAt := survey.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	return ReviewDocument{
		StoreID:         storeID,
		StoreName:       survey.StoreName,
		BranchName:      survey.BranchName,
		Prefecture:      survey.Prefecture.String(),
		Area:            survey.Area,
		Industries:      survey.Industries.Strings(),
		Genre:           survey.Genre,
		Period:          survey.Period,
		Age:             survey.Age,
		SpecScore:       survey.SpecScore,
		WaitTimeMinutes: survey.WaitTime,
		AverageEarning:  survey.AverageEarning,
		EmploymentType:  survey.EmploymentType,
		CustomerNote:    survey.CustomerNote,
		StaffNote:       survey.StaffNote,
		EnvironmentNote: survey.EnvironmentNote,
		Rating:          survey.Rating.Float64(),
		Comment:         survey.Comment,
		ContactEmail:    survey.ContactEmail.String(),
		Photos:          photoDocs,
		Tags:            survey.Tags.Strings(),
		HelpfulCount:    survey.HelpfulCount,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}, nil
}

// mapSurveyPhotoDocumentsToDomain は写真ドキュメントを SurveyPhoto に復元し、URL の妥当性も合わせて確認する。
func mapSurveyPhotoDocumentsToDomain(docs []SurveyPhotoDocument) ([]admindomain.SurveyPhoto, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	result := make([]admindomain.SurveyPhoto, 0, len(docs))
	for _, doc := range docs {
		publicURL, err := admindomain.NewPhotoURL(doc.PublicURL)
		if err != nil {
			return nil, err
		}
		result = append(result, admindomain.SurveyPhoto{
			ID:          doc.ID,
			StoredPath:  doc.StoredPath,
			PublicURL:   publicURL,
			ContentType: doc.ContentType,
			UploadedAt:  doc.UploadedAt,
		})
	}
	return result, nil
}

func mapSurveyPhotosToDocuments(photos []admindomain.SurveyPhoto) ([]SurveyPhotoDocument, error) {
	if len(photos) == 0 {
		return nil, nil
	}
	result := make([]SurveyPhotoDocument, 0, len(photos))
	for _, photo := range photos {
		result = append(result, SurveyPhotoDocument{
			ID:          photo.ID,
			StoredPath:  photo.StoredPath,
			PublicURL:   photo.PublicURL.String(),
			ContentType: photo.ContentType,
			UploadedAt:  photo.UploadedAt,
		})
	}
	return result, nil
}

// buildSurveyUpdatePayload は ReviewDocument を $set 用の BSON マップに変換する。
func buildSurveyUpdatePayload(doc ReviewDocument) bson.M {
	payload := bson.M{
		"storeId":         doc.StoreID,
		"storeName":       doc.StoreName,
		"branchName":      doc.BranchName,
		"prefecture":      doc.Prefecture,
		"area":            doc.Area,
		"industries":      doc.Industries,
		"genre":           doc.Genre,
		"period":          doc.Period,
		"age":             doc.Age,
		"specScore":       doc.SpecScore,
		"waitTimeMinutes": doc.WaitTimeMinutes,
		"averageEarning":  doc.AverageEarning,
		"employmentType":  doc.EmploymentType,
		"customerNote":    doc.CustomerNote,
		"staffNote":       doc.StaffNote,
		"environmentNote": doc.EnvironmentNote,
		"rating":          doc.Rating,
		"comment":         doc.Comment,
		"contactEmail":    doc.ContactEmail,
		"photos":          doc.Photos,
		"tags":            doc.Tags,
		"helpfulCount":    doc.HelpfulCount,
		"updatedAt":       time.Now().UTC(),
	}
	return payload
}

// recalculateStoreStats は対象店舗のアンケートを集計し、レビュー件数や平均値を Store に反映する。
func (r *AdminSurveyRepository) recalculateStoreStats(ctx context.Context, storeID primitive.ObjectID) error {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"storeId": storeID}}},
		{{Key: "$group", Value: bson.M{
			"_id":             nil,
			"reviewCount":     bson.M{"$sum": 1},
			"avgRating":       bson.M{"$avg": "$rating"},
			"avgEarning":      bson.M{"$avg": "$averageEarning"},
			"avgWaitTimeMins": bson.M{"$avg": "$waitTimeMinutes"},
			"lastReviewedAt":  bson.M{"$max": "$createdAt"},
		}}},
	}

	cursor, err := r.reviews.Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	update := bson.M{
		"stats.reviewCount":    0,
		"stats.avgRating":      nil,
		"stats.avgEarning":     nil,
		"stats.avgWaitTime":    nil,
		"stats.lastReviewedAt": nil,
		"updatedAt":            time.Now().UTC(),
	}

	if cursor.Next(ctx) {
		var agg struct {
			ReviewCount     int        `bson:"reviewCount"`
			AvgRating       *float64   `bson:"avgRating"`
			AvgEarning      *float64   `bson:"avgEarning"`
			AvgWaitTimeMins *float64   `bson:"avgWaitTimeMins"`
			LastReviewedAt  *time.Time `bson:"lastReviewedAt"`
		}
		if err := cursor.Decode(&agg); err != nil {
			return err
		}
		update["stats.reviewCount"] = agg.ReviewCount
		update["stats.avgRating"] = agg.AvgRating
		update["stats.avgEarning"] = agg.AvgEarning
		if agg.AvgWaitTimeMins != nil {
			hours := *agg.AvgWaitTimeMins / 60.0
			update["stats.avgWaitTime"] = hours
		}
		update["stats.lastReviewedAt"] = agg.LastReviewedAt
	}
	if err := cursor.Err(); err != nil {
		return err
	}

	_, err = r.stores.UpdateByID(ctx, storeID, bson.M{"$set": update})
	return err
}
