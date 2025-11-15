package mongo

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// StoreStatsDocument は店舗ドキュメント内の stats 埋め込み構造を表す。
type StoreStatsDocument struct {
	ReviewCount    int        `bson:"reviewCount"`
	AvgRating      *float64   `bson:"avgRating,omitempty"`
	AvgEarning     *float64   `bson:"avgEarning,omitempty"`
	AvgWaitTime    *float64   `bson:"avgWaitTime,omitempty"`
	LastReviewedAt *time.Time `bson:"lastReviewedAt,omitempty"`
}

// StoreDocument は MongoDB 上での店舗スキーマを Go 構造体として表現したもの。
type StoreDocument struct {
	ID              primitive.ObjectID `bson:"_id"`
	Name            string             `bson:"name"`
	BranchName      string             `bson:"branchName,omitempty"`
	GroupName       string             `bson:"groupName,omitempty"`
	Prefecture      string             `bson:"prefecture,omitempty"`
	Area            string             `bson:"area,omitempty"`
	Genre           string             `bson:"genre,omitempty"`
	Industries      []string           `bson:"industries,omitempty"`
	EmploymentTypes []string           `bson:"employmentTypes,omitempty"`
	BusinessHours   string             `bson:"businessHours,omitempty"`
	PriceRange      string             `bson:"priceRange,omitempty"`
	PricePerHour    int                `bson:"pricePerHour,omitempty"`
	AverageEarning  int                `bson:"averageEarning,omitempty"`
	Tags            []string           `bson:"tags,omitempty"`
	HomepageURL     string             `bson:"homepageURL,omitempty"`
	SNS             StoreSNSDocument   `bson:"sns,omitempty"`
	PhotoURLs       []string           `bson:"photoURLs,omitempty"`
	Description     string             `bson:"description,omitempty"`
	Stats           StoreStatsDocument `bson:"stats"`
	CreatedAt       *time.Time         `bson:"createdAt,omitempty"`
	UpdatedAt       *time.Time         `bson:"updatedAt,omitempty"`
}

// StoreSNSDocument は SNS リンクを保持する埋め込みドキュメント。
type StoreSNSDocument struct {
	Twitter   string `bson:"twitter,omitempty"`
	Line      string `bson:"line,omitempty"`
	Instagram string `bson:"instagram,omitempty"`
	TikTok    string `bson:"tiktok,omitempty"`
	Official  string `bson:"official,omitempty"`
}

// ReviewDocument は公開・管理いずれのユースケースでも利用するアンケート/レビューのスキーマを表現する。
type ReviewDocument struct {
	ID              primitive.ObjectID    `bson:"_id"`
	StoreID         primitive.ObjectID    `bson:"storeId"`
	StoreName       string                `bson:"storeName"`
	BranchName      string                `bson:"branchName,omitempty"`
	Prefecture      string                `bson:"prefecture,omitempty"`
	Area            string                `bson:"area,omitempty"`
	Industries      []string              `bson:"industries,omitempty"`
	Genre           string                `bson:"genre,omitempty"`
	Period          string                `bson:"period,omitempty"`
	Age             *int                  `bson:"age,omitempty"`
	SpecScore       *int                  `bson:"specScore,omitempty"`
	WaitTimeMinutes *int                  `bson:"waitTimeMinutes,omitempty"`
	AverageEarning  *int                  `bson:"averageEarning,omitempty"`
	EmploymentType  string                `bson:"employmentType,omitempty"`
	CustomerNote    string                `bson:"customerNote,omitempty"`
	StaffNote       string                `bson:"staffNote,omitempty"`
	EnvironmentNote string                `bson:"environmentNote,omitempty"`
	Rating          float64               `bson:"rating"`
	Comment         string                `bson:"comment"`
	ContactEmail    string                `bson:"contactEmail,omitempty"`
	Photos          []SurveyPhotoDocument `bson:"photos,omitempty"`
	Tags            []string              `bson:"tags,omitempty"`
	HelpfulCount    int                   `bson:"helpfulCount,omitempty"`
	CreatedAt       time.Time             `bson:"createdAt"`
	UpdatedAt       time.Time             `bson:"updatedAt"`
}

// SurveyPhotoDocument はアンケート写真 1 枚分のメタデータを格納する埋め込みドキュメント。
type SurveyPhotoDocument struct {
	ID          string    `bson:"id"`
	StoredPath  string    `bson:"storedPath"`
	PublicURL   string    `bson:"publicURL"`
	ContentType string    `bson:"contentType"`
	UploadedAt  time.Time `bson:"uploadedAt"`
}
