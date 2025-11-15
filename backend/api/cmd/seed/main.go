package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type seedOptions struct {
	envName          string
	storeCount       int
	surveyCount      int
	inboundCount     int
	helpfulVoteCount int
	dropCollections  bool
	randomSeed       int64
}

type collections struct {
	stores              string
	surveys             string
	inboundSurveys      string
	helpfulVotes        string
	failedNotifications string
}

type storeDocument struct {
	ID              primitive.ObjectID     `bson:"_id"`
	Name            string                 `bson:"name"`
	BranchName      string                 `bson:"branchName,omitempty"`
	GroupName       string                 `bson:"groupName,omitempty"`
	Prefecture      string                 `bson:"prefecture"`
	Area            string                 `bson:"area"`
	Industries      []string               `bson:"industries,omitempty"`
	Genre           string                 `bson:"genre,omitempty"`
	EmploymentTypes []string               `bson:"employmentTypes,omitempty"`
	PricePerHour    int                    `bson:"pricePerHour,omitempty"`
	AverageEarning  int                    `bson:"averageEarning,omitempty"`
	BusinessHours   string                 `bson:"businessHours,omitempty"`
	Tags            []string               `bson:"tags,omitempty"`
	HomepageURL     string                 `bson:"homepageURL,omitempty"`
	SNS             map[string]string      `bson:"sns,omitempty"`
	PhotoURLs       []string               `bson:"photoURLs,omitempty"`
	Description     string                 `bson:"description,omitempty"`
	Stats           storeStatsDocument     `bson:"stats"`
	CreatedAt       time.Time              `bson:"createdAt"`
	UpdatedAt       time.Time              `bson:"updatedAt"`
	Metadata        map[string]interface{} `bson:"metadata,omitempty"`
}

type storeStatsDocument struct {
	ReviewCount    int        `bson:"reviewCount"`
	AvgRating      *float64   `bson:"avgRating,omitempty"`
	AvgEarning     *float64   `bson:"avgEarning,omitempty"`
	AvgWaitTime    *float64   `bson:"avgWaitTime,omitempty"`
	LastReviewedAt *time.Time `bson:"lastReviewedAt,omitempty"`
}

type surveyDocument struct {
	ID              primitive.ObjectID    `bson:"_id"`
	StoreID         primitive.ObjectID    `bson:"storeId"`
	StoreName       string                `bson:"storeName"`
	BranchName      string                `bson:"branchName,omitempty"`
	Prefecture      string                `bson:"prefecture"`
	Area            string                `bson:"area"`
	Industries      []string              `bson:"industries,omitempty"`
	Genre           string                `bson:"genre,omitempty"`
	Period          string                `bson:"period"`
	Age             *int                  `bson:"age,omitempty"`
	SpecScore       *int                  `bson:"specScore,omitempty"`
	WaitTimeMinutes *int                  `bson:"waitTimeMinutes,omitempty"`
	EmploymentType  string                `bson:"employmentType,omitempty"`
	AverageEarning  *int                  `bson:"averageEarning,omitempty"`
	CustomerNote    string                `bson:"customerNote,omitempty"`
	StaffNote       string                `bson:"staffNote,omitempty"`
	EnvironmentNote string                `bson:"environmentNote,omitempty"`
	Comment         string                `bson:"comment,omitempty"`
	ContactEmail    string                `bson:"contactEmail,omitempty"`
	Rating          float64               `bson:"rating"`
	HelpfulCount    int                   `bson:"helpfulCount"`
	Photos          []surveyPhotoDocument `bson:"photos,omitempty"`
	Tags            []string              `bson:"tags,omitempty"`
	CreatedAt       time.Time             `bson:"createdAt"`
	UpdatedAt       time.Time             `bson:"updatedAt"`
}

type surveyPhotoDocument struct {
	ID          string    `bson:"id"`
	StoredPath  string    `bson:"storedPath"`
	PublicURL   string    `bson:"publicURL"`
	ContentType string    `bson:"contentType"`
	UploadedAt  time.Time `bson:"uploadedAt"`
}

type inboundDocument struct {
	ID               primitive.ObjectID     `bson:"_id"`
	Payload          map[string]string      `bson:"payload"`
	SubmittedAt      time.Time              `bson:"submittedAt"`
	ClientIP         string                 `bson:"clientIp,omitempty"`
	UserAgent        string                 `bson:"userAgent,omitempty"`
	DiscordMessageID string                 `bson:"discordMessageId,omitempty"`
	Metadata         map[string]interface{} `bson:"metadata,omitempty"`
}

type helpfulVoteDocument struct {
	ID        primitive.ObjectID `bson:"_id"`
	SurveyID  primitive.ObjectID `bson:"surveyId"`
	VoterID   string             `bson:"voterId"`
	CreatedAt time.Time          `bson:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"`
}

type failedNotificationDocument struct {
	ID          primitive.ObjectID `bson:"_id"`
	Target      string             `bson:"target"`
	Payload     map[string]string  `bson:"payload"`
	Error       string             `bson:"error"`
	Attempts    int                `bson:"attempts"`
	Status      string             `bson:"status"`
	CreatedAt   time.Time          `bson:"createdAt"`
	LastTriedAt time.Time          `bson:"lastTriedAt"`
}

type storeMeta struct {
	ID         primitive.ObjectID
	Name       string
	BranchName string
	GroupName  string
	Prefecture string
	Area       string
	Industries []string
	Genre      string
	Tags       []string
}

type statsAccumulator struct {
	reviewCount int
	ratingSum   float64
	earningSum  float64
	waitSum     float64
	waitCount   int
	lastReview  time.Time
}

func main() {
	opts := parseFlags()

	if err := loadEnvFiles(opts.envName); err != nil {
		log.Fatalf("環境変数の読み込みに失敗しました: %v", err)
	}

	cfg := collections{
		stores:              envOrDefault("STORE_COLLECTION", "stores"),
		surveys:             firstNonEmpty(os.Getenv("SURVEY_COLLECTION"), os.Getenv("REVIEW_COLLECTION"), "surveys"),
		inboundSurveys:      envOrDefault("INBOUND_SURVEY_COLLECTION", "inbound_surveys"),
		helpfulVotes:        envOrDefault("HELPFUL_VOTE_COLLECTION", "survey_helpful_votes"),
		failedNotifications: envOrDefault("FAILED_NOTIFICATION_COLLECTION", "failed_notifications"),
	}

	mongoURI := envOrDefault("MONGO_URI", "mongodb://localhost:27017")
	dbName := envOrDefault("MONGO_DB", "makoto-club")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("MongoDB 接続に失敗しました: %v", err)
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	db := client.Database(dbName)

	if opts.dropCollections {
		if err := dropCollections(ctx, db, cfg); err != nil {
			log.Fatalf("コレクション削除に失敗しました: %v", err)
		}
		log.Printf("既存コレクションを削除しました")
	}

	if err := ensureIndexes(ctx, db, cfg); err != nil {
		log.Fatalf("インデックス作成に失敗しました: %v", err)
	}

	rng := rand.New(rand.NewSource(opts.randomSeed))

	storeDocs, metas := generateStores(rng, opts.storeCount)
	if len(storeDocs) == 0 {
		log.Fatal("store docs が生成されませんでした")
	}
	if err := insertMany(ctx, db.Collection(cfg.stores), toAnySlice(storeDocs)); err != nil {
		log.Fatalf("店舗データの挿入に失敗しました: %v", err)
	}

	surveyDocs, stats := generateSurveys(rng, metas, opts.surveyCount)
	if len(surveyDocs) == 0 {
		log.Fatal("survey docs が生成されませんでした")
	}
	if err := insertMany(ctx, db.Collection(cfg.surveys), toAnySlice(surveyDocs)); err != nil {
		log.Fatalf("アンケートデータの挿入に失敗しました: %v", err)
	}

	if err := applyStats(ctx, db.Collection(cfg.stores), stats); err != nil {
		log.Fatalf("店舗統計の更新に失敗しました: %v", err)
	}

	inboundDocs := generateInboundDrafts(rng, metas, opts.inboundCount)
	if len(inboundDocs) > 0 {
		if err := insertMany(ctx, db.Collection(cfg.inboundSurveys), toAnySlice(inboundDocs)); err != nil {
			log.Fatalf("未承認アンケートの挿入に失敗しました: %v", err)
		}
	}

	helpfulDocs := generateHelpfulVotes(rng, surveyDocs, opts.helpfulVoteCount)
	if len(helpfulDocs) > 0 {
		if err := insertMany(ctx, db.Collection(cfg.helpfulVotes), toAnySlice(helpfulDocs)); err != nil {
			log.Fatalf("Helpful投票データの挿入に失敗しました: %v", err)
		}
	}

	failedDocs := generateFailedNotifications(rng, inboundDocs)
	if len(failedDocs) > 0 {
		if err := insertMany(ctx, db.Collection(cfg.failedNotifications), toAnySlice(failedDocs)); err != nil {
			log.Fatalf("通知失敗データの挿入に失敗しました: %v", err)
		}
	}

	log.Printf("Seed 完了: stores=%d surveys=%d inbound=%d helpfulVotes=%d failedNotifications=%d",
		len(storeDocs), len(surveyDocs), len(inboundDocs), len(helpfulDocs), len(failedDocs))
	log.Printf("Mongo: %s / %s (env=%s)", mongoURI, dbName, opts.envName)
}

func parseFlags() seedOptions {
	var opts seedOptions
	flag.StringVar(&opts.envName, "env", "local", "backend/env 内の env ファイル名 (例: local, staging)")
	flag.IntVar(&opts.storeCount, "stores", 10, "生成する店舗数")
	flag.IntVar(&opts.surveyCount, "surveys", 100, "生成する公開アンケート総数")
	flag.IntVar(&opts.inboundCount, "inbound", 5, "生成する未承認アンケート数")
	flag.IntVar(&opts.helpfulVoteCount, "helpful", 30, "生成するHelpful投票数")
	flag.BoolVar(&opts.dropCollections, "drop", true, "既存コレクションを削除してから投入する")
	defaultSeed := time.Now().UnixNano()
	flag.Int64Var(&opts.randomSeed, "seed", defaultSeed, "乱数シード（再現用）")
	flag.Parse()

	if opts.storeCount <= 0 {
		log.Fatal("stores は 1 以上を指定してください")
	}
	if opts.surveyCount < opts.storeCount {
		opts.surveyCount = opts.storeCount
	}
	if opts.helpfulVoteCount < 0 {
		opts.helpfulVoteCount = 0
	}
	if opts.inboundCount < 0 {
		opts.inboundCount = 0
	}
	return opts
}

func loadEnvFiles(envName string) error {
	base := filepath.Clean(filepath.Join("..", "env"))
	files := []string{
		filepath.Join(base, "shared.env"),
		filepath.Join(base, fmt.Sprintf("%s.env", envName)),
	}
	for _, file := range files {
		if err := loadEnvFile(file); err != nil {
			return err
		}
	}
	return nil
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%s の読み込みに失敗しました: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func dropCollections(ctx context.Context, db *mongo.Database, cfg collections) error {
	for _, name := range []string{
		cfg.stores, cfg.surveys, cfg.inboundSurveys, cfg.helpfulVotes, cfg.failedNotifications,
	} {
		if err := db.Collection(name).Drop(ctx); err != nil {
			// Drop は存在しない場合も err を返すので warning ログにとどめる
			log.Printf("WARN: コレクション %s の削除に失敗: %v", name, err)
		}
	}
	return nil
}

func ensureIndexes(ctx context.Context, db *mongo.Database, cfg collections) error {
	storeIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "stats.avgRating", Value: -1}},
			Options: options.Index().SetName("idx_store_avgRating"),
		},
		{
			Keys:    bson.D{{Key: "stats.avgEarning", Value: -1}},
			Options: options.Index().SetName("idx_store_avgEarning"),
		},
		{
			Keys:    bson.D{{Key: "name", Value: 1}, {Key: "branchName", Value: 1}},
			Options: options.Index().SetName("uniq_store_name_branch").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "prefecture", Value: 1}},
			Options: options.Index().SetName("idx_store_prefecture"),
		},
	}
	if _, err := db.Collection(cfg.stores).Indexes().CreateMany(ctx, storeIndexes); err != nil {
		return err
	}

	surveyIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "rating", Value: -1}},
			Options: options.Index().SetName("idx_survey_rating"),
		},
		{
			Keys:    bson.D{{Key: "averageEarning", Value: -1}},
			Options: options.Index().SetName("idx_survey_avgEarning"),
		},
		{
			Keys:    bson.D{{Key: "storeId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("idx_survey_store_created"),
		},
	}
	if _, err := db.Collection(cfg.surveys).Indexes().CreateMany(ctx, surveyIndexes); err != nil {
		return err
	}

	if _, err := db.Collection(cfg.helpfulVotes).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "surveyId", Value: 1}, {Key: "voterId", Value: 1}},
		Options: options.Index().SetName("uniq_helpful_vote").SetUnique(true),
	}); err != nil {
		return err
	}

	if _, err := db.Collection(cfg.inboundSurveys).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "submittedAt", Value: -1}},
		Options: options.Index().SetName("idx_inbound_submitted"),
	}); err != nil {
		return err
	}

	if _, err := db.Collection(cfg.failedNotifications).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("idx_failed_status_created"),
		},
	}); err != nil {
		return err
	}

	return nil
}

func generateStores(rng *rand.Rand, count int) ([]storeDocument, []storeMeta) {
	now := time.Now().UTC()
	docs := make([]storeDocument, 0, count)
	metas := make([]storeMeta, 0, count)

	for i := 0; i < count; i++ {
		name := storeNames[i%len(storeNames)]
		branch := randomBranch(rng)
		group := randomGroup(rng)
		pref := prefectures[rng.Intn(len(prefectures))]
		area := areaForPrefecture(pref, rng)
		industries := pickUnique(rng, industryOptions, 1+rng.Intn(2))
		genre := genreOptions[rng.Intn(len(genreOptions))]
		employmentTypes := pickUnique(rng, employmentTypeOptions, 1+rng.Intn(len(employmentTypeOptions)))
		price := 9000 + rng.Intn(15000)
		avgEarn := price + rng.Intn(8000)
		tags := pickUnique(rng, tagOptions, 1+rng.Intn(3))
		photos := generatePhotoURLs(rng, name, 3)
		description := randomDescription(rng)
		sns := map[string]string{
			"twitter": fmt.Sprintf("https://twitter.com/%s", slugify(name, branch)),
		}
		if rng.Intn(3) == 0 {
			sns["instagram"] = fmt.Sprintf("https://instagram.com/%s", slugify(name, "official"))
		}

		created := now.Add(-time.Duration(rng.Intn(365*2)) * 24 * time.Hour)
		doc := storeDocument{
			ID:              primitive.NewObjectID(),
			Name:            name,
			BranchName:      branch,
			GroupName:       group,
			Prefecture:      pref,
			Area:            area,
			Industries:      industries,
			Genre:           genre,
			EmploymentTypes: employmentTypes,
			PricePerHour:    price,
			AverageEarning:  avgEarn,
			BusinessHours:   randomBusinessHours(rng),
			Tags:            tags,
			HomepageURL:     fmt.Sprintf("https://example.com/stores/%s", slugify(name, branch)),
			SNS:             sns,
			PhotoURLs:       photos,
			Description:     description,
			Stats: storeStatsDocument{
				ReviewCount:    0,
				AvgRating:      nil,
				AvgEarning:     nil,
				AvgWaitTime:    nil,
				LastReviewedAt: nil,
			},
			CreatedAt: created,
			UpdatedAt: created,
			Metadata: map[string]interface{}{
				"areaLabel": area,
			},
		}
		docs = append(docs, doc)
		metas = append(metas, storeMeta{
			ID:         doc.ID,
			Name:       doc.Name,
			BranchName: doc.BranchName,
			GroupName:  doc.GroupName,
			Prefecture: doc.Prefecture,
			Area:       doc.Area,
			Industries: doc.Industries,
			Genre:      doc.Genre,
			Tags:       doc.Tags,
		})
	}
	return docs, metas
}

func generateSurveys(rng *rand.Rand, stores []storeMeta, total int) ([]surveyDocument, map[primitive.ObjectID]*statsAccumulator) {
	if total < len(stores) {
		total = len(stores)
	}
	counts := distribute(total, len(stores), 1, 18, rng)
	now := time.Now().UTC()
	stats := make(map[primitive.ObjectID]*statsAccumulator, len(stores))
	surveys := make([]surveyDocument, 0, total)

	for idx, store := range stores {
		for j := 0; j < counts[idx]; j++ {
			created := now.Add(-time.Duration(rng.Intn(120*24)) * time.Hour)
			rating := 60 + rng.Intn(41)
			wait := 10 + rng.Intn(80)
			waitPtr := wait
			if rng.Intn(4) == 0 {
				waitPtr = 0
			}
			earning := 8000 + rng.Intn(15000)
			earningPtr := earning

			var agePtr *int
			if rng.Intn(3) != 0 {
				age := 18 + rng.Intn(15)
				agePtr = &age
			}

			var waitPointer *int
			if waitPtr > 0 {
				waitPointer = &waitPtr
			}

			var specPtr *int
			if rng.Intn(4) != 0 {
				spec := 60 + rng.Intn(41)
				specPtr = &spec
			}

			var contact string
			if rng.Intn(3) == 0 {
				contact = fmt.Sprintf("user%d@example.com", rng.Intn(9000)+1000)
			}

			photoCount := rng.Intn(4)
			photos := generateSurveyPhotos(rng, store.Name, photoCount, created)

			survey := surveyDocument{
				ID:              primitive.NewObjectID(),
				StoreID:         store.ID,
				StoreName:       store.Name,
				BranchName:      store.BranchName,
				Prefecture:      store.Prefecture,
				Area:            store.Area,
				Industries:      store.Industries,
				Genre:           store.Genre,
				Period:          randomPeriod(rng),
				Age:             agePtr,
				SpecScore:       specPtr,
				WaitTimeMinutes: waitPointer,
				EmploymentType:  employmentTypeOptions[rng.Intn(len(employmentTypeOptions))],
				AverageEarning:  &earningPtr,
				CustomerNote:    randomCustomerNote(rng),
				StaffNote:       randomStaffNote(rng),
				EnvironmentNote: randomEnvironmentNote(rng),
				Comment:         randomComment(rng),
				ContactEmail:    contact,
				Rating:          float64(rating),
				HelpfulCount:    rng.Intn(40),
				Photos:          photos,
				Tags:            store.Tags,
				CreatedAt:       created,
				UpdatedAt:       created,
			}
			surveys = append(surveys, survey)

			acc := stats[store.ID]
			if acc == nil {
				acc = &statsAccumulator{}
				stats[store.ID] = acc
			}
			acc.reviewCount++
			acc.ratingSum += float64(rating)
			acc.earningSum += float64(earning)
			if waitPointer != nil {
				acc.waitSum += float64(*waitPointer)
				acc.waitCount++
			}
			if created.After(acc.lastReview) {
				acc.lastReview = created
			}
		}
	}

	return surveys, stats
}

func generateInboundDrafts(rng *rand.Rand, stores []storeMeta, count int) []inboundDocument {
	if count == 0 {
		return nil
	}
	docs := make([]inboundDocument, 0, count)
	for i := 0; i < count; i++ {
		store := stores[rng.Intn(len(stores))]
		payload := map[string]string{
			"storeName":       store.Name,
			"branchName":      store.BranchName,
			"prefecture":      store.Prefecture,
			"area":            store.Area,
			"industry":        store.Industries[0],
			"genre":           store.Genre,
			"employmentType":  employmentTypeOptions[rng.Intn(len(employmentTypeOptions))],
			"period":          randomPeriod(rng),
			"averageEarning":  fmt.Sprintf("%d", 9000+rng.Intn(12000)),
			"rating":          fmt.Sprintf("%d", 60+rng.Intn(40)),
			"customerNote":    randomCustomerNote(rng),
			"staffNote":       randomStaffNote(rng),
			"environmentNote": randomEnvironmentNote(rng),
			"comment":         randomComment(rng),
			"email":           fmt.Sprintf("draft%d@example.com", rng.Intn(9999)),
			"photos":          strings.Join(generatePhotoURLs(rng, store.Name, 2), ","),
			"extra":           "未整形フィールド",
		}
		doc := inboundDocument{
			ID:               primitive.NewObjectID(),
			Payload:          payload,
			SubmittedAt:      time.Now().Add(-time.Duration(rng.Intn(96)) * time.Hour),
			ClientIP:         fmt.Sprintf("192.168.%d.%d", rng.Intn(255), rng.Intn(255)),
			UserAgent:        userAgents[rng.Intn(len(userAgents))],
			DiscordMessageID: fmt.Sprintf("MSG-%04d", rng.Intn(9999)),
			Metadata: map[string]interface{}{
				"rawText": "Discord経由で受信した生テキスト",
			},
		}
		docs = append(docs, doc)
	}
	return docs
}

func generateHelpfulVotes(rng *rand.Rand, surveys []surveyDocument, desired int) []helpfulVoteDocument {
	if desired <= 0 || len(surveys) == 0 {
		return nil
	}
	type key struct {
		Survey primitive.ObjectID
		Voter  string
	}
	used := make(map[key]struct{})
	var docs []helpfulVoteDocument
	for len(docs) < desired {
		survey := surveys[rng.Intn(len(surveys))]
		voter := fmt.Sprintf("voter-%06d", rng.Intn(999999))
		k := key{Survey: survey.ID, Voter: voter}
		if _, exists := used[k]; exists {
			continue
		}
		used[k] = struct{}{}
		timestamp := time.Now().Add(-time.Duration(rng.Intn(240)) * time.Hour)
		docs = append(docs, helpfulVoteDocument{
			ID:        primitive.NewObjectID(),
			SurveyID:  survey.ID,
			VoterID:   voter,
			CreatedAt: timestamp,
			UpdatedAt: timestamp,
		})
	}
	return docs
}

func generateFailedNotifications(rng *rand.Rand, inbound []inboundDocument) []failedNotificationDocument {
	if len(inbound) == 0 {
		return nil
	}
	var docs []failedNotificationDocument
	limit := 2
	if len(inbound) < limit {
		limit = len(inbound)
	}
	for i := 0; i < limit; i++ {
		d := inbound[rng.Intn(len(inbound))]
		payload := map[string]string{
			"storeName":  d.Payload["storeName"],
			"branchName": d.Payload["branchName"],
			"rating":     d.Payload["rating"],
		}
		created := time.Now().Add(-time.Duration(rng.Intn(24)) * time.Hour)
		docs = append(docs, failedNotificationDocument{
			ID:          primitive.NewObjectID(),
			Target:      "discord",
			Payload:     payload,
			Error:       "429 Too Many Requests",
			Attempts:    3,
			Status:      "pending",
			CreatedAt:   created,
			LastTriedAt: created.Add(-15 * time.Minute),
		})
	}
	return docs
}

func insertMany(ctx context.Context, col *mongo.Collection, docs []interface{}) error {
	if len(docs) == 0 {
		return nil
	}
	_, err := col.InsertMany(ctx, docs)
	return err
}

func applyStats(ctx context.Context, col *mongo.Collection, stats map[primitive.ObjectID]*statsAccumulator) error {
	now := time.Now().UTC()
	for id, agg := range stats {
		if agg.reviewCount == 0 {
			continue
		}
		update := bson.M{
			"stats.reviewCount":    agg.reviewCount,
			"stats.lastReviewedAt": agg.lastReview,
			"updatedAt":            now,
		}
		if agg.reviewCount > 0 {
			avgRating := round(agg.ratingSum/float64(agg.reviewCount), 1)
			update["stats.avgRating"] = avgRating
		}
		if agg.reviewCount > 0 {
			avgEarning := round(agg.earningSum/float64(agg.reviewCount), 1)
			update["stats.avgEarning"] = avgEarning
		}
		if agg.waitCount > 0 {
			update["stats.avgWaitTime"] = round(agg.waitSum/float64(agg.waitCount), 1)
		}
		if agg.lastReview.IsZero() {
			update["stats.lastReviewedAt"] = time.Now().UTC()
		}
		if _, err := col.UpdateByID(ctx, id, bson.M{"$set": update}); err != nil {
			return err
		}
	}
	return nil
}

func toAnySlice[T any](in []T) []interface{} {
	out := make([]interface{}, len(in))
	for i := range in {
		out[i] = in[i]
	}
	return out
}

func distribute(total, buckets, minPerBucket, maxPerBucket int, rng *rand.Rand) []int {
	if buckets <= 0 {
		return nil
	}
	if maxPerBucket < minPerBucket {
		maxPerBucket = minPerBucket
	}
	counts := make([]int, buckets)
	for i := range counts {
		counts[i] = minPerBucket
	}
	remaining := total - minPerBucket*buckets
	if remaining < 0 {
		remaining = 0
	}
	for remaining > 0 {
		i := rng.Intn(buckets)
		if counts[i] >= maxPerBucket {
			continue
		}
		counts[i]++
		remaining--
	}
	return counts
}

func pickUnique(rng *rand.Rand, source []string, count int) []string {
	if count >= len(source) {
		cp := make([]string, len(source))
		copy(cp, source)
		return cp
	}
	seen := make(map[int]struct{}, count)
	result := make([]string, 0, count)
	for len(result) < count {
		idx := rng.Intn(len(source))
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		result = append(result, source[idx])
	}
	return result
}

func randomBranch(rng *rand.Rand) string {
	branches := []string{"", "本店", "新宿店", "池袋店", "梅田店", "中洲店", "すすきの店", "難波店", "博多店"}
	return branches[rng.Intn(len(branches))]
}

func randomGroup(rng *rand.Rand) string {
	groups := []string{"桜会グループ", "百花繚乱グループ", "レジーナチェーン", "Aroma Collective", "Luxe Holdings", ""}
	return groups[rng.Intn(len(groups))]
}

func areaForPrefecture(pref string, rng *rand.Rand) string {
	if areas, ok := areaCandidates[pref]; ok && len(areas) > 0 {
		return areas[rng.Intn(len(areas))]
	}
	return pref + "中心部"
}

func randomBusinessHours(rng *rand.Rand) string {
	open := 8 + rng.Intn(6)
	closeHour := 22 + rng.Intn(6)
	return fmt.Sprintf("%02d:00-%02d:00", open, closeHour%24)
}

func generatePhotoURLs(rng *rand.Rand, name string, max int) []string {
	if max <= 0 {
		return nil
	}
	count := 1 + rng.Intn(max)
	urls := make([]string, 0, count)
	for i := 0; i < count; i++ {
		size := "600x400"
		bg := colorCodes[rng.Intn(len(colorCodes))]
		fg := colorCodes[rng.Intn(len(colorCodes))]
		text := url.QueryEscape(fmt.Sprintf("%s %d", name, i+1))
		typ := imageTypes[rng.Intn(len(imageTypes))]
		font := fonts[rng.Intn(len(fonts))]
		fontSize := 18 + rng.Intn(10)
		urls = append(urls, fmt.Sprintf("https://dummyjson.com/image/%s/%s/%s/?text=%s&type=%s&fontFamily=%s&fontSize=%d",
			size, bg, fg, text, typ, font, fontSize))
	}
	return urls
}

func generateSurveyPhotos(rng *rand.Rand, name string, max int, uploaded time.Time) []surveyPhotoDocument {
	if max <= 0 {
		return nil
	}
	count := max
	if count == 0 {
		return nil
	}
	photos := make([]surveyPhotoDocument, 0, count)
	for i := 0; i < count; i++ {
		url := generatePhotoURLs(rng, name, 1)[0]
		photos = append(photos, surveyPhotoDocument{
			ID:          fmt.Sprintf("photo-%s-%d", strings.ToLower(slugify(name, "")), i+1),
			StoredPath:  fmt.Sprintf("stores/%s/surveys/%s/%d.jpg", slugify(name, ""), primitive.NewObjectID().Hex(), i+1),
			PublicURL:   url,
			ContentType: "image/jpeg",
			UploadedAt:  uploaded,
		})
	}
	return photos
}

func randomDescription(rng *rand.Rand) string {
	return descriptionFragments[rng.Intn(len(descriptionFragments))]
}

func randomCustomerNote(rng *rand.Rand) string {
	return customerPatterns[rng.Intn(len(customerPatterns))]
}

func randomStaffNote(rng *rand.Rand) string {
	return staffPatterns[rng.Intn(len(staffPatterns))]
}

func randomEnvironmentNote(rng *rand.Rand) string {
	return environmentPatterns[rng.Intn(len(environmentPatterns))]
}

func randomComment(rng *rand.Rand) string {
	return overallComments[rng.Intn(len(overallComments))]
}

func randomPeriod(rng *rand.Rand) string {
	return periodOptions[rng.Intn(len(periodOptions))]
}

func round(val float64, precision int) float64 {
	mul := math.Pow(10, float64(precision))
	return math.Round(val*mul) / mul
}

func slugify(parts ...string) string {
	builder := strings.Builder{}
	for _, part := range parts {
		for _, r := range strings.ToLower(part) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				builder.WriteRune(r)
			} else if unicode.IsSpace(r) || r == '-' || r == '_' {
				builder.WriteRune('-')
			}
		}
	}
	out := builder.String()
	out = strings.Trim(out, "-")
	if out == "" {
		return fmt.Sprintf("store-%d", time.Now().UnixNano())
	}
	return out
}

var (
	storeNames = []string{
		"月ノ雫", "艶姫", "夜桜御殿", "白百合倶楽部", "香りの園", "Luxe Palace", "Aroma Myth", "クラブ翔", "凛 -Rin-", "紅椿", "Crystal Muse", "Enigma", "雅ラウンジ", "Moonlight Atelier", "Velvet Salon",
	}

	prefectures = []string{
		"北海道", "宮城県", "東京都", "神奈川県", "千葉県", "埼玉県", "静岡県", "愛知県", "京都府", "大阪府", "兵庫県", "広島県", "福岡県",
	}

	areaCandidates = map[string][]string{
		"東京都": {"吉原", "歌舞伎町", "渋谷", "池袋"},
		"北海道": {"すすきの"},
		"兵庫県": {"福原", "三宮"},
		"福岡県": {"中洲", "天神"},
		"大阪府": {"梅田", "難波"},
	}

	industryOptions       = []string{"デリヘル", "ホテヘル", "箱ヘル", "ソープ", "DC", "風エス", "メンエス"}
	genreOptions          = []string{"スタンダード", "熟女", "学園", "ぽっちゃり", "格安", "大衆", "高級"}
	tagOptions            = []string{"個室", "半個室", "裏", "講習無", "店泊可", "雑費無料"}
	employmentTypeOptions = []string{"出稼ぎ", "在籍"}

	colorCodes = []string{"111111", "222222", "333333", "444444", "555555", "666666", "999999", "cccccc", "f5f5f5"}
	imageTypes = []string{"png", "jpeg", "webp"}
	fonts      = []string{"Inter", "NotoSans", "Roboto", "Arial"}

	descriptionFragments = []string{
		"完全個室・アロマ特化の空間で、在籍キャストは20代中心。講習なしで自由なスタイルが魅力。",
		"熟女専門。講習は最小限で、ヘルプ体制が厚い。稼ぎは安定し、店泊設備も清潔。",
		"吉原で老舗クラスの箱。裏オプションは無く、接客品質で勝負。待機所は半個室。",
		"メンエスとしては高単価帯。写真研修とSNSサポートが手厚く、雑費控除が少ない。",
	}

	customerPatterns = []string{
		"30〜40代の常連サラリーマンが多く落ち着いた雰囲気。",
		"観光客の比率が高いが、身なりの良い方が多く治安は良い。",
		"可愛い系より落ち着いた女性を好むお客様ばかり。",
	}

	staffPatterns = []string{
		"教育担当が常駐していて質問しやすい。LINEレスも早い。",
		"店長は厳しいが公平。女性スタッフが多く話しやすい。",
		"送りドライバーが丁寧で、終電ギリギリでも柔軟に対応してくれる。",
	}

	environmentPatterns = []string{
		"待機場は半個室。Wifi・充電完備で店泊も可能。",
		"雑費は完全無料。ロッカーも鍵付きで安心。",
		"衣装は貸し出し有り、メイクルームも広い。",
	}

	overallComments = []string{
		"総合的にかなり働きやすい。初めてでも安心できた。",
		"稼ぎやすさ重視なら候補に入れて損はない。",
		"講習がなく自由度が高い反面、自己管理は必須。",
	}

	periodOptions = []string{
		"2024年9月", "2024年10月", "2024年11月", "2024年12月", "2025年1月", "2025年2月",
	}

	userAgents = []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 13_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	}
)
