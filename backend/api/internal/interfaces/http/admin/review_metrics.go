package admin

import (
	"errors"
	"math"
	"strings"
)

type reviewMetrics struct {
	VisitedAt      string
	Age            int
	SpecScore      int
	WaitTimeHours  int
	AverageEarning int
	Comment        string
	Rating         float64
	ContactEmail   string
}

// normalize は Admin 用入力の検証と補正を実施する。
func (m *reviewMetrics) normalize() error {
	m.VisitedAt = strings.TrimSpace(m.VisitedAt)
	if m.VisitedAt == "" {
		return errors.New("働いた時期を指定してください")
	}
	if m.Age < 18 {
		return errors.New("年齢は18歳以上で入力してください")
	}
	if m.Age > 60 {
		m.Age = 60
	}
	if m.SpecScore < 60 {
		return errors.New("スペックは60以上で入力してください")
	}
	if m.SpecScore > 140 {
		m.SpecScore = 140
	}
	if m.WaitTimeHours < 1 {
		return errors.New("待機時間は1時間以上で入力してください")
	}
	if m.WaitTimeHours > 24 {
		m.WaitTimeHours = 24
	}
	if m.AverageEarning < 0 {
		return errors.New("平均稼ぎは0以上で入力してください")
	}
	if m.AverageEarning > 20 {
		m.AverageEarning = 20
	}
	if m.Rating < 0 || m.Rating > 5 {
		return errors.New("総評は0〜5の範囲で入力してください")
	}
	m.Rating = math.Round(m.Rating*2) / 2
	m.Comment = strings.TrimSpace(m.Comment)
	return nil
}
