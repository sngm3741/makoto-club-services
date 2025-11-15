package domain

import "time"

// Survey は公開済みアンケートの読み取りモデル。Store との結合済み情報を含む。
type Survey struct {
	ID              string
	StoreID         string
	StoreName       string
	BranchName      string
	Prefecture      string
	Area            string
	Industries      []string
	Genre           string
	Period          string
	Age             *int
	SpecScore       *int
	WaitTime        *int
	EmploymentType  string
	AverageEarning  *int
	CustomerNote    string
	StaffNote       string
	EnvironmentNote string
	Comment         string
	ContactEmail    string
	Rating          float64
	HelpfulCount    int
	Photos          []SurveyPhoto
	Tags            []string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SurveyPhoto はアンケートに紐づく画像メタデータ。
type SurveyPhoto struct {
	ID          string
	StoredPath  string
	PublicURL   string
	ContentType string
	UploadedAt  time.Time
}
