package domain

import "time"

// InboundSurvey is a raw submission awaiting admin review.
type InboundSurvey struct {
	ID          string
	RawPayload  string
	SubmittedAt time.Time
	DiscordMsg  string
	ClientIP    string
}
