package domain

import "time"

// Survey represents admin-managed survey entity.
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
	Tags            []string
	Photos          []SurveyPhoto
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SurveyPhoto stores uploaded file metadata.
type SurveyPhoto struct {
	ID          string
	StoredPath  string
	PublicURL   string
	ContentType string
	UploadedAt  time.Time
}
