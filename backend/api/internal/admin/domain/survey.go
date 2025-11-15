package domain

import "time"

// Survey represents admin-managed survey entity.
type Survey struct {
	ID              string
	StoreID         string
	StoreName       string
	BranchName      string
	Prefecture      Prefecture
	Area            string
	Industries      IndustryList
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
	ContactEmail    Email
	Rating          Rating
	HelpfulCount    int
	Tags            TagList
	Photos          []SurveyPhoto
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SurveyPhoto stores uploaded file metadata.
type SurveyPhoto struct {
	ID          string
	StoredPath  string
	PublicURL   PhotoURL
	ContentType string
	UploadedAt  time.Time
}
