package mongo

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/sngm3741/makoto-club-services/api/internal/public/application"
	"github.com/sngm3741/makoto-club-services/api/internal/public/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SurveyRepository はパブリック向けアンケート集約を MongoDB で扱う実装リポジトリ。
type SurveyRepository struct {
	reviews *mongo.Collection
	stores  *mongo.Collection
	votes   *HelpfulVoteRepository
}

// NewSurveyRepository はレビュー・店舗・Helpful 投票のコレクションを束縛したリポジトリを構築する。
func NewSurveyRepository(db *mongo.Database, reviewCollection, storeCollection, helpfulCollection string) *SurveyRepository {
	return &SurveyRepository{
		reviews: db.Collection(reviewCollection),
		stores:  db.Collection(storeCollection),
		votes:   NewHelpfulVoteRepository(db, helpfulCollection),
	}
}

// Find は Store/Pefecture/キーワード/タグの複合条件を Mongo クエリへ落とし込み、Store ドキュメントと突き合わせたアンケート一覧を返す。
func (r *SurveyRepository) Find(ctx context.Context, filter application.SurveyFilter, paging application.Paging) ([]domain.Survey, error) {
	mongoFilter := bson.M{}
	andClauses := make([]bson.M, 0)

	if filter.StoreID != "" {
		id, err := primitive.ObjectIDFromHex(strings.TrimSpace(filter.StoreID))
		if err != nil {
			return nil, err
		}
		mongoFilter["storeId"] = id
	}

	if filter.Genre != "" {
		genre := canonicalIndustryCode(filter.Genre)
		andClauses = append(andClauses, bson.M{"$or": bson.A{
			bson.M{"genre": genre},
			bson.M{"industries": genre},
		}})
	}

	if len(filter.Tags) > 0 {
		andClauses = append(andClauses, bson.M{"tags": bson.M{"$all": filter.Tags}})
	}

	if filter.Keyword != "" {
		pattern := primitive.Regex{Pattern: regexp.QuoteMeta(filter.Keyword), Options: "i"}
		andClauses = append(andClauses, bson.M{"$or": bson.A{
			bson.M{"comment": pattern},
			bson.M{"customerNote": pattern},
			bson.M{"staffNote": pattern},
			bson.M{"environmentNote": pattern},
		}})
	}

	if filter.StoreID == "" && (filter.Prefecture != "" || filter.StoreName != "") {
		storeIDs, err := r.lookupStoreIDs(ctx, filter.Prefecture, filter.StoreName)
		if err != nil {
			return nil, err
		}
		if len(storeIDs) == 0 {
			return []domain.Survey{}, nil
		}
		mongoFilter["storeId"] = bson.M{"$in": storeIDs}
	}

	if len(andClauses) == 1 {
		for k, v := range andClauses[0] {
			mongoFilter[k] = v
		}
	} else if len(andClauses) > 1 {
		mongoFilter["$and"] = andClauses
	}

	cursor, err := r.reviews.Find(ctx, mongoFilter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	reviews := make([]ReviewDocument, 0)
	storeSet := make(map[primitive.ObjectID]struct{})
	for cursor.Next(ctx) {
		var doc ReviewDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		reviews = append(reviews, doc)
		storeSet[doc.StoreID] = struct{}{}
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	storeIDs := make([]primitive.ObjectID, 0, len(storeSet))
	for id := range storeSet {
		storeIDs = append(storeIDs, id)
	}

	storeMap, err := r.loadStoreMap(ctx, storeIDs)
	if err != nil {
		return nil, err
	}

	surveys := make([]domain.Survey, 0, len(reviews))
	for _, review := range reviews {
		store, ok := storeMap[review.StoreID]
		if !ok {
			continue
		}
		surveys = append(surveys, mapSurveyDocument(review, store))
	}
	return surveys, nil
}

// FindByID はアンケート ID から単一レビューを取得し、関連店舗情報と合わせてドメイン Survey を返す。
func (r *SurveyRepository) FindByID(ctx context.Context, id string) (*domain.Survey, error) {
	objectID, err := primitive.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}

	var reviewDoc ReviewDocument
	if err := r.reviews.FindOne(ctx, bson.M{"_id": objectID}).Decode(&reviewDoc); err != nil {
		return nil, err
	}

	storeDoc := StoreDocument{}
	if err := r.stores.FindOne(ctx, bson.M{"_id": reviewDoc.StoreID}).Decode(&storeDoc); err != nil {
		return nil, err
	}

	survey := mapSurveyDocument(reviewDoc, storeDoc)
	return &survey, nil
}

// Create はアンケート投稿を Mongo に追加し、ドメインモデルへ採番結果を反映する。
func (r *SurveyRepository) Create(ctx context.Context, survey *domain.Survey) error {
	storeID, err := primitive.ObjectIDFromHex(strings.TrimSpace(survey.StoreID))
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	createdAt := survey.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	updatedAt := survey.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	industries := append([]string{}, survey.Industries...)
	if len(industries) == 0 {
		return errors.New("industries must not be empty")
	}

	photoDocs := mapSurveyPhotoDocuments(survey.Photos)

	doc := ReviewDocument{
		ID:              primitive.NewObjectID(),
		StoreID:         storeID,
		StoreName:       survey.StoreName,
		BranchName:      survey.BranchName,
		Prefecture:      survey.Prefecture,
		Area:            survey.Area,
		Industries:      industries,
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
		Rating:          survey.Rating,
		Comment:         survey.Comment,
		ContactEmail:    survey.ContactEmail,
		Photos:          photoDocs,
		Tags:            append([]string{}, survey.Tags...),
		HelpfulCount:    survey.HelpfulCount,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}

	if _, err := r.reviews.InsertOne(ctx, doc); err != nil {
		return err
	}

	survey.ID = doc.ID.Hex()
	survey.CreatedAt = doc.CreatedAt
	survey.UpdatedAt = doc.UpdatedAt
	return nil
}

// IncrementHelpful は Helpful 投票のトグルを記録し、実際に変化があった場合のみカウンタを増減する。
func (r *SurveyRepository) IncrementHelpful(ctx context.Context, surveyID, voterID string, inc bool) (int, error) {
	surveyObjID, err := primitive.ObjectIDFromHex(strings.TrimSpace(surveyID))
	if err != nil {
		return 0, err
	}
	voterObjID, err := primitive.ObjectIDFromHex(strings.TrimSpace(voterID))
	if err != nil {
		return 0, err
	}

	changed, err := r.votes.Upsert(ctx, surveyObjID, voterObjID, inc)
	if err != nil {
		return 0, err
	}

	update := bson.M{}
	if changed {
		delta := 1
		if !inc {
			delta = -1
		}
		update["$inc"] = bson.M{"helpfulCount": delta}
	}

	var updated ReviewDocument
	if len(update) == 0 {
		if err := r.reviews.FindOne(ctx, bson.M{"_id": surveyObjID}).Decode(&updated); err != nil {
			return 0, err
		}
		return updated.HelpfulCount, nil
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	if err := r.reviews.FindOneAndUpdate(ctx, bson.M{"_id": surveyObjID}, update, opts).Decode(&updated); err != nil {
		return 0, err
	}
	return updated.HelpfulCount, nil
}

// lookupStoreIDs は店名/都道府県から Store ObjectID を逆引きする補助関数。
func (r *SurveyRepository) lookupStoreIDs(ctx context.Context, prefecture, storeName string) ([]primitive.ObjectID, error) {
	storeFilter := bson.M{}
	if prefecture != "" {
		storeFilter["prefecture"] = strings.TrimSpace(prefecture)
	}
	if storeName != "" {
		storeFilter["name"] = bson.M{"$regex": storeName, "$options": "i"}
	}

	cursor, err := r.stores.Find(ctx, storeFilter, optionsFindIDProjection())
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	ids := make([]primitive.ObjectID, 0)
	for cursor.Next(ctx) {
		var doc struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		ids = append(ids, doc.ID)
	}
	return ids, cursor.Err()
}

// loadStoreMap は ID 群を一括取得して StoreDocument のマップへ変換する。
func (r *SurveyRepository) loadStoreMap(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]StoreDocument, error) {
	result := make(map[primitive.ObjectID]StoreDocument, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	cursor, err := r.stores.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc StoreDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		result[doc.ID] = doc
	}
	return result, cursor.Err()
}

// mapSurveyDocument はレビュー＋店舗ドキュメントを統合し、公開ドメイン Survey へマッピングする。
func mapSurveyDocument(review ReviewDocument, store StoreDocument) domain.Survey {
	industries := review.Industries
	if len(industries) == 0 {
		industries = store.Industries
	}

	genre := review.Genre
	if genre == "" {
		genre = store.Genre
	}

	waitTime := review.WaitTimeMinutes

	photos := mapSurveyPhotosFromDocs(review.Photos)

	tags := review.Tags
	if len(tags) == 0 {
		tags = store.Tags
	}

	customerNote := review.CustomerNote
	if customerNote == "" {
		customerNote = review.Comment
	}
	staffNote := review.StaffNote
	if staffNote == "" {
		staffNote = review.Comment
	}
	environmentNote := review.EnvironmentNote
	if environmentNote == "" {
		environmentNote = review.Comment
	}

	storeName := review.StoreName
	if storeName == "" {
		storeName = store.Name
	}
	branchName := review.BranchName
	if branchName == "" {
		branchName = store.BranchName
	}
	prefecture := review.Prefecture
	if prefecture == "" {
		prefecture = store.Prefecture
	}
	area := review.Area
	if area == "" {
		area = store.Area
	}

	return domain.Survey{
		ID:              review.ID.Hex(),
		StoreID:         review.StoreID.Hex(),
		StoreName:       storeName,
		BranchName:      strings.TrimSpace(branchName),
		Prefecture:      prefecture,
		Area:            area,
		Industries:      append([]string{}, industries...),
		Genre:           genre,
		Period:          review.Period,
		Age:             review.Age,
		SpecScore:       review.SpecScore,
		WaitTime:        waitTime,
		AverageEarning:  review.AverageEarning,
		EmploymentType:  review.EmploymentType,
		CustomerNote:    customerNote,
		StaffNote:       staffNote,
		EnvironmentNote: environmentNote,
		Comment:         review.Comment,
		ContactEmail:    review.ContactEmail,
		Rating:          review.Rating,
		HelpfulCount:    review.HelpfulCount,
		Tags:            append([]string{}, tags...),
		Photos:          photos,
		CreatedAt:       review.CreatedAt,
		UpdatedAt:       review.UpdatedAt,
	}
}

// mapSurveyPhotosFromDocs は Mongo 写真ドキュメントを公開 SurveyPhoto に復元する。
func mapSurveyPhotosFromDocs(docs []SurveyPhotoDocument) []domain.SurveyPhoto {
	if len(docs) == 0 {
		return nil
	}
	result := make([]domain.SurveyPhoto, 0, len(docs))
	for _, doc := range docs {
		result = append(result, domain.SurveyPhoto{
			ID:          doc.ID,
			StoredPath:  doc.StoredPath,
			PublicURL:   doc.PublicURL,
			ContentType: doc.ContentType,
			UploadedAt:  doc.UploadedAt,
		})
	}
	return result
}

// mapSurveyPhotoDocuments は公開 SurveyPhoto を Mongo の SurveyPhotoDocument へ変換する。
func mapSurveyPhotoDocuments(photos []domain.SurveyPhoto) []SurveyPhotoDocument {
	if len(photos) == 0 {
		return nil
	}
	result := make([]SurveyPhotoDocument, 0, len(photos))
	for _, photo := range photos {
		result = append(result, SurveyPhotoDocument{
			ID:          photo.ID,
			StoredPath:  photo.StoredPath,
			PublicURL:   photo.PublicURL,
			ContentType: photo.ContentType,
			UploadedAt:  photo.UploadedAt,
		})
	}
	return result
}

// canonicalIndustryCode はユーザー入力を正規化して既知の業種ラベルに合わせる。
func canonicalIndustryCode(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	lower := strings.ToLower(trimmed)
	switch lower {
	case "deriheru", "delivery_health":
		return "デリヘル"
	case "hoteheru", "hotel_health":
		return "ホテヘル"
	case "hakoheru", "hako_heru", "hako-health":
		return "箱ヘル"
	case "sopu", "soap":
		return "ソープ"
	case "dc":
		return "DC"
	case "huesu", "fuesu":
		return "風エス"
	case "menesu", "mensu", "mens_es":
		return "メンエス"
	}

	switch trimmed {
	case "デリヘル", "ホテヘル", "箱ヘル", "ソープ", "DC", "風エス", "メンエス":
		return trimmed
	}

	return trimmed
}

// optionsFindIDProjection は _id のみの軽量クエリを作るためのヘルパー。
func optionsFindIDProjection() *options.FindOptions {
	opt := options.Find()
	opt.SetProjection(bson.M{"_id": 1})
	return opt
}
