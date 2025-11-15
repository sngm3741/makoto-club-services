package domain

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"
)

var (
	allowedStoreTags       = []string{"個室", "半個室", "裏", "講習無", "店泊可", "雑費無料"}
	allowedEmploymentTypes = []string{"出稼ぎ", "在籍"}
)

type Prefecture string

func NewPrefecture(value string) (Prefecture, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("prefecture is required")
	}
	return Prefecture(trimmed), nil
}

func (p Prefecture) String() string {
	return string(p)
}

type Industry string

func NewIndustry(value string) (Industry, error) {
	code := canonicalIndustryCode(value)
	if code == "" {
		return "", fmt.Errorf("industry is required")
	}
	return Industry(code), nil
}

type IndustryList []Industry

func NewIndustryList(values []string) (IndustryList, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("industries must not be empty")
	}
	result := make([]Industry, 0, len(values))
	seen := make(map[Industry]struct{})
	for _, raw := range values {
		value, err := NewIndustry(raw)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return IndustryList(result), nil
}

func (l IndustryList) Strings() []string {
	result := make([]string, 0, len(l))
	for _, v := range l {
		result = append(result, string(v))
	}
	return result
}

type EmploymentType string

func NewEmploymentType(value string) (EmploymentType, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("employment type is required")
	}
	for _, allowed := range allowedEmploymentTypes {
		if allowed == trimmed {
			return EmploymentType(trimmed), nil
		}
	}
	return "", fmt.Errorf("invalid employment type: %s", trimmed)
}

type EmploymentTypeList []EmploymentType

func NewEmploymentTypeList(values []string) (EmploymentTypeList, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make([]EmploymentType, 0, len(values))
	seen := make(map[EmploymentType]struct{})
	for _, raw := range values {
		value, err := NewEmploymentType(raw)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return EmploymentTypeList(result), nil
}

func (l EmploymentTypeList) Strings() []string {
	result := make([]string, 0, len(l))
	for _, v := range l {
		result = append(result, string(v))
	}
	return result
}

type Tag string

func NewTag(value string) (Tag, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("tag is required")
	}
	for _, allowed := range allowedStoreTags {
		if allowed == trimmed {
			return Tag(trimmed), nil
		}
	}
	return "", fmt.Errorf("invalid tag: %s", trimmed)
}

type TagList []Tag

func NewTagList(values []string) (TagList, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make([]Tag, 0, len(values))
	seen := make(map[Tag]struct{})
	for _, raw := range values {
		tag, err := NewTag(raw)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return TagList(result), nil
}

func (l TagList) Strings() []string {
	result := make([]string, 0, len(l))
	for _, v := range l {
		result = append(result, string(v))
	}
	return result
}

type Email string

func NewEmail(value string) (Email, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > 254 {
		return "", fmt.Errorf("email too long")
	}
	if _, err := mail.ParseAddress(trimmed); err != nil {
		return "", fmt.Errorf("invalid email: %w", err)
	}
	return Email(trimmed), nil
}

func (e Email) String() string {
	return string(e)
}

type Money int

func NewMoney(value int) (Money, error) {
	if value < 0 {
		return 0, fmt.Errorf("money must be >= 0")
	}
	return Money(value), nil
}

func (m Money) Int() int {
	return int(m)
}

type Rating float64

func NewRating(value float64) (Rating, error) {
	if value < 0 || value > 5 {
		return 0, fmt.Errorf("rating must be between 0 and 5")
	}
	return Rating(value), nil
}

func (r Rating) Float64() float64 {
	return float64(r)
}

type URL string

func NewURL(value string) (URL, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if _, err := url.ParseRequestURI(trimmed); err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	return URL(trimmed), nil
}

func (u URL) String() string {
	return string(u)
}

type PhotoURL string

func NewPhotoURL(value string) (PhotoURL, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("photo URL is required")
	}
	if _, err := url.ParseRequestURI(trimmed); err != nil {
		return "", fmt.Errorf("invalid photo URL: %w", err)
	}
	return PhotoURL(trimmed), nil
}

func (u PhotoURL) String() string {
	return string(u)
}

type PhotoURLList []PhotoURL

func NewPhotoURLList(values []string, limit int) (PhotoURLList, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if limit > 0 && len(values) > limit {
		return nil, fmt.Errorf("photo URLs must be <= %d", limit)
	}
	result := make([]PhotoURL, 0, len(values))
	for _, raw := range values {
		urlValue, err := NewPhotoURL(raw)
		if err != nil {
			return nil, err
		}
		result = append(result, urlValue)
	}
	return PhotoURLList(result), nil
}

func (l PhotoURLList) Strings() []string {
	result := make([]string, 0, len(l))
	for _, v := range l {
		result = append(result, string(v))
	}
	return result
}

func canonicalIndustryCode(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	lower := strings.ToLower(trimmed)
	switch lower {
	case "deriheru", "delivery_health":
		return "デリヘル"
	case "hoteheru", "hotel_health":
		return "ホテヘル"
	case "hakoheru", "hako_heru", "hako-health":
		return "箱ヘル"
	case "sopu", "soap":
		return "ソープ"
	case "dc":
		return "DC"
	case "huesu", "fuesu":
		return "風エス"
	case "menesu", "mensu", "mens_es":
		return "メンエス"
	}

	switch trimmed {
	case "デリヘル", "ホテヘル", "箱ヘル", "ソープ", "DC", "風エス", "メンエス":
		return trimmed
	}

	return trimmed
}
