package domain

import "time"

// Store は一般公開向けに表示する店舗アグリゲートの読み取りモデル。
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

// SNSLinks は店舗に紐づく SNS リンク群をまとめたサブ構造体。
type SNSLinks struct {
	Twitter   string
	Line      string
	Instagram string
	TikTok    string
	Official  string
}

// StoreStats はアンケートから算出された統計情報をまとめるビュー専用構造体。
type StoreStats struct {
	ReviewCount    int
	AvgRating      *float64
	AvgEarning     *float64
	AvgWaitTime    *float64
	LastReviewedAt *time.Time
}
