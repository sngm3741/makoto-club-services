package domain

import "time"

// Store aggregates data required for admin operations.
type Store struct {
	ID              string
	Name            string
	BranchName      string
	GroupName       string
	Prefecture      string
	Area            string
	Genre           string
	Industries      []string
	EmploymentTypes []string
	PricePerHour    int
	PriceRange      string
	AverageEarning  int
	BusinessHours   string
	Tags            []string
	HomepageURL     string
	SNS             SNSLinks
	PhotoURLs       []string
	Description     string
	ReviewCount     int
	LastReviewedAt  *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SNSLinks mirrors the structured SNS URLs for admin context.
type SNSLinks struct {
	Twitter   string
	Line      string
	Instagram string
	TikTok    string
	Official  string
}
