package domain

import "time"

// Store aggregates data required for admin operations.
type Store struct {
	ID              string
	Name            string
	BranchName      string
	GroupName       string
	Prefecture      Prefecture
	Area            string
	Genre           string
	Industries      IndustryList
	EmploymentTypes EmploymentTypeList
	PricePerHour    Money
	PriceRange      string
	AverageEarning  Money
	BusinessHours   string
	Tags            TagList
	HomepageURL     URL
	SNS             SNSLinks
	PhotoURLs       PhotoURLList
	Description     string
	ReviewCount     int
	LastReviewedAt  *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SNSLinks mirrors the structured SNS URLs for admin context.
type SNSLinks struct {
	Twitter   URL
	Line      URL
	Instagram URL
	TikTok    URL
	Official  URL
}

func NewSNSLinks(twitter, line string, instagram, tiktok, official string) (SNSLinks, error) {
	tw, err := NewURL(twitter)
	if err != nil {
		return SNSLinks{}, err
	}
	ln, err := NewURL(line)
	if err != nil {
		return SNSLinks{}, err
	}
	insta, err := NewURL(instagram)
	if err != nil {
		return SNSLinks{}, err
	}
	tk, err := NewURL(tiktok)
	if err != nil {
		return SNSLinks{}, err
	}
	of, err := NewURL(official)
	if err != nil {
		return SNSLinks{}, err
	}
	return SNSLinks{
		Twitter:   tw,
		Line:      ln,
		Instagram: insta,
		TikTok:    tk,
		Official:  of,
	}, nil
}
