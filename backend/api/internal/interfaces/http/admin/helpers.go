package admin

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	mongodoc "github.com/sngm3741/makoto-club-services/api/internal/infrastructure/mongo"
	"github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (h *Handler) loadStoresMap(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]mongodoc.StoreDocument, error) {
	result := make(map[primitive.ObjectID]mongodoc.StoreDocument, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	cursor, err := h.stores.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc mongodoc.StoreDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		result[doc.ID] = doc
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (h *Handler) getStoreByID(ctx context.Context, id primitive.ObjectID) (mongodoc.StoreDocument, error) {
	var store mongodoc.StoreDocument
	err := h.stores.FindOne(ctx, bson.M{"_id": id}).Decode(&store)
	return store, err
}

func (h *Handler) recalculateStoreStats(ctx context.Context, storeID primitive.ObjectID) error {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"storeId": storeID}}},
		{{Key: "$group", Value: bson.M{
			"_id":            nil,
			"reviewCount":    bson.M{"$sum": 1},
			"avgRating":      bson.M{"$avg": "$rating"},
			"avgEarning":     bson.M{"$avg": "$averageEarning"},
			"avgWaitTime":    bson.M{"$avg": "$waitTimeHours"},
			"lastReviewedAt": bson.M{"$max": "$createdAt"},
		}}},
	}

	cursor, err := h.reviews.Aggregate(ctx, pipeline)
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
		"updatedAt":            time.Now().In(h.location),
	}

	if cursor.Next(ctx) {
		var agg struct {
			ReviewCount    int        `bson:"reviewCount"`
			AvgRating      *float64   `bson:"avgRating"`
			AvgEarning     *float64   `bson:"avgEarning"`
			AvgWaitTime    *float64   `bson:"avgWaitTime"`
			LastReviewedAt *time.Time `bson:"lastReviewedAt"`
		}
		if err := cursor.Decode(&agg); err != nil {
			return err
		}
		update["stats.reviewCount"] = agg.ReviewCount
		update["stats.avgRating"] = agg.AvgRating
		update["stats.avgEarning"] = agg.AvgEarning
		update["stats.avgWaitTime"] = agg.AvgWaitTime
		update["stats.lastReviewedAt"] = agg.LastReviewedAt
	}
	if err := cursor.Err(); err != nil {
		return err
	}

	_, err = h.stores.UpdateByID(ctx, storeID, bson.M{"$set": update})
	return err
}

func buildAdminReviewResponse(review mongodoc.ReviewDocument, store mongodoc.StoreDocument) adminReviewResponse {
	category := ""
	if len(review.Industries) > 0 {
		category = common.CanonicalIndustryCode(review.Industries[0])
	} else if len(store.Industries) > 0 {
		category = common.CanonicalIndustryCode(store.Industries[0])
	}
	if category == "" {
		category = "デリヘル"
	}

	industries := common.CanonicalIndustryCodes(review.Industries)
	if len(industries) == 0 {
		industries = common.CanonicalIndustryCodes(store.Industries)
	}

	genre := review.Genre
	if genre == "" {
		genre = store.Genre
	}

	visitedAt, _ := deriveDates(review.Period)

	waitMinutes := 0
	if review.WaitTimeMinutes != nil {
		waitMinutes = *review.WaitTimeMinutes
	}

	photos := convertSurveyPhotosForAdmin(review.Photos)

	return adminReviewResponse{
		ID:              review.ID.Hex(),
		StoreID:         review.StoreID.Hex(),
		StoreName:       store.Name,
		BranchName:      strings.TrimSpace(store.BranchName),
		Prefecture:      store.Prefecture,
		Area:            store.Area,
		Category:        category,
		Industries:      industries,
		Genre:           genre,
		VisitedAt:       visitedAt,
		Age:             intPtrValue(review.Age),
		SpecScore:       intPtrValue(review.SpecScore),
		WaitTimeMinutes: waitMinutes,
		AverageEarning:  intPtrValue(review.AverageEarning),
		EmploymentType:  review.EmploymentType,
		Rating:          review.Rating,
		Comment:         strings.TrimSpace(review.Comment),
		CustomerNote:    strings.TrimSpace(review.CustomerNote),
		StaffNote:       strings.TrimSpace(review.StaffNote),
		EnvironmentNote: strings.TrimSpace(review.EnvironmentNote),
		Tags:            append([]string{}, review.Tags...),
		ContactEmail:    review.ContactEmail,
		Photos:          photos,
		HelpfulCount:    review.HelpfulCount,
		CreatedAt:       review.CreatedAt,
		UpdatedAt:       review.UpdatedAt,
	}
}

func convertSurveyPhotosForAdmin(docs []mongodoc.SurveyPhotoDocument) []adminSurveyPhotoResponse {
	if len(docs) == 0 {
		return nil
	}
	result := make([]adminSurveyPhotoResponse, 0, len(docs))
	for _, doc := range docs {
		result = append(result, adminSurveyPhotoResponse{
			ID:          doc.ID,
			StoredPath:  doc.StoredPath,
			PublicURL:   doc.PublicURL,
			ContentType: doc.ContentType,
			UploadedAt:  doc.UploadedAt,
		})
	}
	return result
}

func deriveDates(period string) (visited string, created string) {
	period = strings.TrimSpace(period)
	if period == "" {
		now := time.Now()
		return now.Format("2006-01"), now.Format("2006-01-02")
	}

	replacer := strings.NewReplacer("年", "-", "月", "-01")
	normalized := replacer.Replace(period)
	t, err := time.Parse("2006-01-02", normalized)
	if err != nil {
		now := time.Now()
		return now.Format("2006-01"), now.Format("2006-01-02")
	}
	return t.Format("2006-01"), t.Format("2006-01-02")
}

func formatSurveyPeriod(visited string) (string, error) {
	value := strings.TrimSpace(visited)
	if value == "" {
		return "", errors.New("働いた時期を指定してください")
	}

	t, err := time.Parse("2006-01", value)
	if err != nil {
		return "", fmt.Errorf("働いた時期の形式が不正です: %w", err)
	}

	return fmt.Sprintf("%d年%d月", t.Year(), int(t.Month())), nil
}

func normalizeEmail(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > 254 {
		return "", errors.New("メールアドレスは254文字以内で入力してください")
	}
	if _, err := mail.ParseAddress(trimmed); err != nil {
		return "", errors.New("メールアドレスの形式が正しくありません")
	}
	return trimmed, nil
}

func intPtrValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) == target {
			return true
		}
	}
	return false
}
