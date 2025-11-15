package public

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	mongodoc "github.com/sngm3741/makoto-club-services/api/internal/infrastructure/mongo"
	"github.com/sngm3741/makoto-club-services/api/internal/interfaces/http/common"
	publicapp "github.com/sngm3741/makoto-club-services/api/internal/public/application"
	"github.com/sngm3741/makoto-club-services/api/internal/public/domain"
)

const (
	helpfulCookieName   = "mc_helpful_voter"
	helpfulCookieTTL    = 180 * 24 * time.Hour
	helpfulCookieMaxAge = int(helpfulCookieTTL / time.Second)
)

type createReviewRequest struct {
	StoreName       string               `json:"storeName"`
	BranchName      string               `json:"branchName"`
	Prefecture      string               `json:"prefecture"`
	Industries      []string             `json:"industries"`
	VisitedAt       string               `json:"visitedAt"`
	Age             int                  `json:"age"`
	SpecScore       int                  `json:"specScore"`
	WaitTimeHours   int                  `json:"waitTimeHours"`
	AverageEarning  int                  `json:"averageEarning"`
	Comment         string               `json:"comment"`
	Rating          float64              `json:"rating"`
	ContactEmail    string               `json:"contactEmail,omitempty"`
	CustomerNote    string               `json:"customerNote,omitempty"`
	StaffNote       string               `json:"staffNote,omitempty"`
	EnvironmentNote string               `json:"environmentNote,omitempty"`
	Tags            []string             `json:"tags"`
	Photos          []reviewPhotoPayload `json:"photos"`
}

type createReviewResponse struct {
	Status string                `json:"status"`
	Review reviewSummaryResponse `json:"review"`
	Detail reviewDetailResponse  `json:"detail"`
}

type reviewPhotoPayload struct {
	ID          string `json:"id"`
	StoredPath  string `json:"storedPath"`
	PublicURL   string `json:"publicUrl"`
	ContentType string `json:"contentType"`
}

type reviewMetrics struct {
	VisitedAt      string
	Age            int
	SpecScore      int
	WaitTimeHours  int
	AverageEarning int
	Comment        string
	Rating         float64
	ContactEmail   string
}

func (m *reviewMetrics) normalize() error {
	m.VisitedAt = strings.TrimSpace(m.VisitedAt)
	if m.VisitedAt == "" {
		return errors.New("働いた時期を指定してください")
	}
	if m.Age < 18 {
		return errors.New("年齢は18歳以上で入力してください")
	}
	if m.Age > 60 {
		m.Age = 60
	}
	if m.SpecScore < 60 {
		return errors.New("スペックは60以上で入力してください")
	}
	if m.SpecScore > 140 {
		m.SpecScore = 140
	}
	if m.WaitTimeHours < 1 {
		return errors.New("待機時間は1時間以上で入力してください")
	}
	if m.WaitTimeHours > 24 {
		m.WaitTimeHours = 24
	}
	if m.AverageEarning < 0 {
		return errors.New("平均稼ぎは0以上で入力してください")
	}
	if m.AverageEarning > 20 {
		m.AverageEarning = 20
	}
	if m.Rating < 0 || m.Rating > 5 {
		return errors.New("総評は0〜5の範囲で入力してください")
	}
	m.Rating = math.Round(m.Rating*2) / 2
	comment := strings.TrimSpace(m.Comment)
	if utf8.RuneCountInString(comment) > 4000 {
		return errors.New("コメントは4000文字以内で入力してください")
	}
	m.Comment = comment

	email, err := normalizeEmail(m.ContactEmail)
	if err != nil {
		return err
	}
	m.ContactEmail = email

	return nil
}

func (req *createReviewRequest) validate() error {
	if strings.TrimSpace(req.StoreName) == "" {
		return errors.New("店舗名は必須です")
	}
	if strings.TrimSpace(req.Prefecture) == "" {
		return errors.New("都道府県は必須です")
	}
	if len(req.Industries) == 0 {
		return errors.New("業種は1件以上指定してください")
	}
	if len(req.Photos) > common.MaxSurveyPhotoCount {
		return fmt.Errorf("写真は最大%d枚までです", common.MaxSurveyPhotoCount)
	}
	metrics := reviewMetrics{
		VisitedAt:      req.VisitedAt,
		Age:            req.Age,
		SpecScore:      req.SpecScore,
		WaitTimeHours:  req.WaitTimeHours,
		AverageEarning: req.AverageEarning,
		Comment:        req.Comment,
		Rating:         req.Rating,
		ContactEmail:   req.ContactEmail,
	}
	if err := metrics.normalize(); err != nil {
		return err
	}
	req.VisitedAt = metrics.VisitedAt
	req.Age = metrics.Age
	req.SpecScore = metrics.SpecScore
	req.WaitTimeHours = metrics.WaitTimeHours
	req.AverageEarning = metrics.AverageEarning
	req.Comment = metrics.Comment
	req.Rating = metrics.Rating
	req.ContactEmail = metrics.ContactEmail
	req.BranchName = strings.TrimSpace(req.BranchName)
	return nil
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

func (h *Handler) reviewCreateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := common.UserFromContext(r.Context())
		if !ok {
			common.WriteJSON(h.logger, w, http.StatusInternalServerError, map[string]string{"error": "認証情報を取得できませんでした"})
			return
		}

		defer r.Body.Close()

		var req createReviewRequest
		decoder := json.NewDecoder(io.LimitReader(r.Body, common.MaxReviewRequestBody))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("リクエストの形式が不正です: %v", err),
			})
			return
		}

		if err := req.validate(); err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		period, err := formatSurveyPeriod(req.VisitedAt)
		if err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		industries, err := common.NormalizeIndustryList(req.Industries)
		if err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		category := ""
		if len(industries) > 0 {
			category = industries[0]
		}
		comment := strings.TrimSpace(req.Comment)

		storeName := strings.TrimSpace(req.StoreName)
		branchName := strings.TrimSpace(req.BranchName)
		prefecture := strings.TrimSpace(req.Prefecture)

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		store, err := h.findOrCreateStore(ctx, storeName, branchName, prefecture, category)
		if err != nil {
			h.logger.Printf("店舗の取得/作成に失敗: %v", err)
			http.Error(w, "店舗情報の処理に失敗しました", http.StatusInternalServerError)
			return
		}

		tags, err := common.NormalizeStoreTags(req.Tags)
		if err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		photos, err := normalizeReviewPhotos(req.Photos, common.MaxSurveyPhotoCount)
		if err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		cmd := publicapp.SubmitSurveyCommand{
			StoreID:         store.ID.Hex(),
			StoreName:       store.Name,
			BranchName:      strings.TrimSpace(store.BranchName),
			Prefecture:      store.Prefecture,
			Area:            store.Area,
			Industries:      industries,
			Period:          period,
			Age:             common.IntPtr(req.Age),
			SpecScore:       common.IntPtr(req.SpecScore),
			WaitTime:        common.IntPtr(req.WaitTimeHours),
			AverageEarning:  common.IntPtr(req.AverageEarning),
			EmploymentType:  "",
			CustomerNote:    strings.TrimSpace(req.CustomerNote),
			StaffNote:       strings.TrimSpace(req.StaffNote),
			EnvironmentNote: strings.TrimSpace(req.EnvironmentNote),
			Comment:         comment,
			Rating:          req.Rating,
			ContactEmail:    req.ContactEmail,
			Tags:            tags,
			Photos:          photos,
		}

		createdSurvey, err := h.surveyCommands.Submit(ctx, cmd)
		if err != nil {
			h.logger.Printf("レビューの保存に失敗: %v", err)
			http.Error(w, "レビューの保存に失敗しました", http.StatusInternalServerError)
			return
		}

		if category != "" {
			_, err := h.stores.UpdateByID(ctx, store.ID, bson.M{"$addToSet": bson.M{"industries": category}})
			if err != nil {
				h.logger.Printf("店舗業種の更新に失敗: %v", err)
			}
		}

		if err := h.recalculateStoreStats(ctx, store.ID); err != nil {
			h.logger.Printf("店舗統計の更新に失敗: %v", err)
		}
		if refreshed, err := h.getStoreByID(ctx, store.ID); err == nil {
			store = refreshed
		}

		createdSurvey.StoreName = store.Name
		createdSurvey.BranchName = strings.TrimSpace(store.BranchName)
		createdSurvey.Prefecture = store.Prefecture
		createdSurvey.Area = store.Area

		summary := buildReviewSummaryFromDomain(*createdSurvey)
		detail := buildReviewDetailFromDomain(*createdSurvey, reviewerDisplayName(user), user.Picture)

		go h.notifyReviewReceipt(context.Background(), user, summary, comment)

		common.WriteJSON(h.logger, w, http.StatusCreated, createReviewResponse{
			Status: "ok",
			Review: summary,
			Detail: detail,
		})
	}
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

func normalizeReviewPhotos(payloads []reviewPhotoPayload, max int) ([]domain.SurveyPhoto, error) {
	if len(payloads) == 0 {
		return nil, nil
	}
	result := make([]domain.SurveyPhoto, 0, len(payloads))
	for _, payload := range payloads {
		id := strings.TrimSpace(payload.ID)
		publicURL := strings.TrimSpace(payload.PublicURL)
		if id == "" {
			return nil, errors.New("写真IDは必須です")
		}
		if publicURL == "" {
			return nil, fmt.Errorf("写真 %s の公開URLを指定してください", id)
		}
		storedPath := strings.TrimSpace(payload.StoredPath)
		if storedPath == "" {
			storedPath = id
		}
		result = append(result, domain.SurveyPhoto{
			ID:          id,
			StoredPath:  storedPath,
			PublicURL:   publicURL,
			ContentType: strings.TrimSpace(payload.ContentType),
			UploadedAt:  time.Now().UTC(),
		})
		if len(result) > max {
			return nil, fmt.Errorf("写真は最大%d枚までです", max)
		}
	}
	return result, nil
}

func (h *Handler) findOrCreateStore(ctx context.Context, name, branch, prefecture, category string) (mongodoc.StoreDocument, error) {
	name = strings.TrimSpace(name)
	branch = strings.TrimSpace(branch)
	prefecture = strings.TrimSpace(prefecture)
	category = common.CanonicalIndustryCode(category)
	if name == "" {
		return mongodoc.StoreDocument{}, errors.New("店舗名が指定されていません")
	}

	filter := bson.M{"name": name}
	if branch != "" {
		filter["branchName"] = branch
	}
	if prefecture != "" {
		filter["prefecture"] = prefecture
	}

	var store mongodoc.StoreDocument
	err := h.stores.FindOne(ctx, filter).Decode(&store)
	if err == nil {
		return store, nil
	}
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return mongodoc.StoreDocument{}, err
	}

	now := time.Now().In(h.location)
	newID := primitive.NewObjectID()
	doc := bson.M{
		"_id":       newID,
		"name":      name,
		"createdAt": now,
		"updatedAt": now,
		"stats": bson.M{
			"reviewCount": 0,
		},
	}
	if branch != "" {
		doc["branchName"] = branch
	}
	if prefecture != "" {
		doc["prefecture"] = prefecture
	}
	if category != "" {
		doc["industries"] = bson.A{category}
	}

	if _, err := h.stores.InsertOne(ctx, doc); err != nil {
		return mongodoc.StoreDocument{}, err
	}

	return h.getStoreByID(ctx, newID)
}

func (h *Handler) recalculateStoreStats(ctx context.Context, storeID primitive.ObjectID) error {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"storeId": storeID,
		}}},
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

func (h *Handler) getStoreByID(ctx context.Context, id primitive.ObjectID) (mongodoc.StoreDocument, error) {
	var store mongodoc.StoreDocument
	err := h.stores.FindOne(ctx, bson.M{"_id": id}).Decode(&store)
	return store, err
}

func (h *Handler) reviewHelpfulToggleHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idParam := strings.TrimSpace(chi.URLParam(r, "id"))
		if idParam == "" {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "IDが指定されていません"})
			return
		}
		if _, err := primitive.ObjectIDFromHex(idParam); err != nil {
			common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "不正なIDです"})
			return
		}

		payload := struct {
			Helpful *bool `json:"helpful"`
		}{}
		desired := true
		if r.Body != nil {
			defer r.Body.Close()
			decoder := json.NewDecoder(io.LimitReader(r.Body, 1024))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&payload); err != nil && err != io.EOF {
				common.WriteJSON(h.logger, w, http.StatusBadRequest, map[string]string{"error": "リクエストの形式が不正です"})
				return
			}
		}
		if payload.Helpful != nil {
			desired = *payload.Helpful
		}

		voterID, err := h.ensureHelpfulVoterID(w, r)
		if err != nil {
			h.logger.Printf("helpful voter cookie error: %v", err)
			http.Error(w, "役に立った投票処理に失敗しました", http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		count, err := h.surveyCommands.ToggleHelpful(ctx, idParam, voterID, desired)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				http.NotFound(w, r)
				return
			}
			h.logger.Printf("helpful toggle failed: survey=%s voter=%s err=%v", idParam, voterID, err)
			http.Error(w, "役に立った情報の更新に失敗しました", http.StatusInternalServerError)
			return
		}

		common.WriteJSON(h.logger, w, http.StatusOK, map[string]any{
			"helpfulCount": count,
			"helpful":      desired,
		})
	}
}

func (h *Handler) ensureHelpfulVoterID(w http.ResponseWriter, r *http.Request) (string, error) {
	if len(h.helpfulCookieSecret) == 0 {
		return "", errors.New("helpful voter secret not configured")
	}
	if cookie, err := r.Cookie(helpfulCookieName); err == nil {
		if voterID, issuedAt, ok := h.parseHelpfulCookie(cookie.Value); ok && time.Since(issuedAt) < helpfulCookieTTL {
			return voterID, nil
		}
	}
	voterID := primitive.NewObjectID().Hex()
	h.issueHelpfulCookie(w, voterID)
	return voterID, nil
}

func (h *Handler) issueHelpfulCookie(w http.ResponseWriter, voterID string) {
	value := h.signHelpfulCookie(voterID, time.Now().UTC())
	http.SetCookie(w, &http.Cookie{
		Name:     helpfulCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.helpfulCookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   helpfulCookieMaxAge,
	})
}

func (h *Handler) signHelpfulCookie(voterID string, issuedAt time.Time) string {
	payload := fmt.Sprintf("v=%s&ts=%d", voterID, issuedAt.Unix())
	mac := hmac.New(sha256.New, h.helpfulCookieSecret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "&sig=" + sig
}

func (h *Handler) parseHelpfulCookie(raw string) (string, time.Time, bool) {
	parts := strings.Split(raw, "&")
	if len(parts) < 3 {
		return "", time.Time{}, false
	}
	values := make(map[string]string, len(parts))
	for _, part := range parts {
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) != 2 {
			continue
		}
		values[keyValue[0]] = keyValue[1]
	}
	voterID := values["v"]
	timestamp := values["ts"]
	sig := values["sig"]
	if voterID == "" || timestamp == "" || sig == "" {
		return "", time.Time{}, false
	}

	payload := fmt.Sprintf("v=%s&ts=%s", voterID, timestamp)
	mac := hmac.New(sha256.New, h.helpfulCookieSecret)
	mac.Write([]byte(payload))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expectedSig), []byte(sig)) {
		return "", time.Time{}, false
	}

	tsInt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return "", time.Time{}, false
	}
	return voterID, time.Unix(tsInt, 0), true
}
