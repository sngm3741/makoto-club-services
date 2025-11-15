package domain

// SurveySummary はトップページや一覧で使用するアンケートの簡易ビュー。
type SurveySummary struct {
	ID             string        `json:"id"`
	StoreID        string        `json:"storeId"`
	StoreName      string        `json:"storeName"`
	BranchName     string        `json:"branchName,omitempty"`
	Prefecture     string        `json:"prefecture"`
	Industries     []string      `json:"industries,omitempty"`
	VisitedAt      string        `json:"visitedAt"`
	Age            int           `json:"age"`
	SpecScore      int           `json:"specScore"`
	WaitTimeHours  int           `json:"waitTimeHours"`
	AverageEarning int           `json:"averageEarning"`
	Rating         float64       `json:"rating"`
	CreatedAt      string        `json:"createdAt"`
	HelpfulCount   int           `json:"helpfulCount,omitempty"`
	Excerpt        string        `json:"excerpt,omitempty"`
	Tags           []string      `json:"tags,omitempty"`
	Photos         []SurveyPhoto `json:"photos,omitempty"`
}

// SurveyDetail は個別ページで必要となる詳細属性を SurveySummary に付け足したもの。
type SurveyDetail struct {
	SurveySummary
	Description       string `json:"description"`
	AuthorDisplayName string `json:"authorDisplayName"`
	AuthorAvatarURL   string `json:"authorAvatarUrl,omitempty"`
	CustomerNote      string `json:"customerNote,omitempty"`
	StaffNote         string `json:"staffNote,omitempty"`
	EnvironmentNote   string `json:"environmentNote,omitempty"`
	Comment           string `json:"comment,omitempty"`
}
