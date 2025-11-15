package domain

import "time"

// Store represents a publicly visible store entity.
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
	Stats           StoreStats
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SNSLinks defines structured SNS URLs for a store.
type SNSLinks struct {
	Twitter   string
	Line      string
	Instagram string
	TikTok    string
	Official  string
}

// StoreStats aggregates review/earning metrics.
type StoreStats struct {
	ReviewCount    int
	AvgRating      *float64
	AvgEarning     *float64
	AvgWaitTime    *float64
	LastReviewedAt *time.Time
}
