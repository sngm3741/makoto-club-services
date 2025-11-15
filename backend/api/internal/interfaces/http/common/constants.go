package common

const (
	// MaxStorePhotoCount represents the number of store photos admin can register.
	MaxStorePhotoCount = 10
	// MaxSurveyPhotoCount represents the number of review photos accepted per survey.
	MaxSurveyPhotoCount = 10
	// MaxStoreDescriptionRunes limits store description length to keep payloads sane.
	MaxStoreDescriptionRunes = 2000
	// MaxReviewRequestBody limits JSON request bodies for review/store endpoints.
	MaxReviewRequestBody = 1 << 20
)
