package common

import (
	"errors"
	"fmt"
	"strings"
)

var (
	AllowedStoreTags       = []string{"個室", "半個室", "裏", "講習無", "店泊可", "雑費無料"}
	AllowedEmploymentTypes = []string{"出稼ぎ", "在籍"}

	allowedStoreTagSet       = makeStringSet(AllowedStoreTags)
	allowedEmploymentTypeSet = makeStringSet(AllowedEmploymentTypes)
)

func makeStringSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		set[item] = struct{}{}
	}
	return set
}

// CanonicalIndustryCode normalises various aliases into canonical Japanese labels.
func CanonicalIndustryCode(input string) string {
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

// CanonicalIndustryCodes de-duplicates and cleans industry codes.
func CanonicalIndustryCodes(codes []string) []string {
	result := make([]string, 0, len(codes))
	seen := make(map[string]struct{})
	for _, code := range codes {
		canonical := CanonicalIndustryCode(code)
		if canonical == "" {
			continue
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	return result
}

// NormalizeIndustryList validates and normalizes industry inputs.
func NormalizeIndustryList(values []string) ([]string, error) {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))

	appendIndustry := func(raw string) error {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil
		}
		code := CanonicalIndustryCode(raw)
		if code == "" {
			return fmt.Errorf("無効な業種です: %s", raw)
		}
		if _, ok := seen[code]; ok {
			return nil
		}
		seen[code] = struct{}{}
		result = append(result, code)
		return nil
	}

	for _, raw := range values {
		if err := appendIndustry(raw); err != nil {
			return nil, err
		}
	}

	if len(result) == 0 {
		return nil, errors.New("業種を1つ以上指定してください")
	}

	return result, nil
}

// NormalizeEmploymentTypes validates employment type selections.
func NormalizeEmploymentTypes(types []string) ([]string, error) {
	result := make([]string, 0, len(types))
	seen := make(map[string]struct{})
	for _, t := range types {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := allowedEmploymentTypeSet[t]; !ok {
			return nil, fmt.Errorf("不正な勤務形態です: %s", t)
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		result = append(result, t)
	}
	return result, nil
}

// NormalizeStoreTags validates store tag selections.
func NormalizeStoreTags(tags []string) ([]string, error) {
	result := make([]string, 0, len(tags))
	seen := make(map[string]struct{})
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := allowedStoreTagSet[tag]; !ok {
			return nil, fmt.Errorf("不正なタグです: %s", tag)
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result, nil
}
