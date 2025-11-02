export type ReviewCategory =
  | 'delivery_health'
  | 'hotel_health'
  | 'box_health'
  | 'soap'
  | 'dc'
  | 'fu_es'
  | 'men_es';

export interface ReviewSummary {
  id: string;
  storeName: string;
  branchName?: string;
  prefecture: string;
  category: ReviewCategory;
  visitedAt: string;
  age: number;
  specScore: number;
  waitTimeHours: number;
  averageEarning: number;
  rating: number;
  createdAt: string;
  helpfulCount?: number;
  excerpt?: string;
}

export interface ReviewDetail extends ReviewSummary {
  description?: string;
  authorDisplayName?: string;
  authorAvatarUrl?: string;
}

export interface ReviewListResponse {
  items: ReviewSummary[];
  page: number;
  limit: number;
  total: number;
}

export interface StoreSummary {
  id: string;
  storeName: string;
  prefecture: string;
  category: ReviewCategory;
  averageEarning: number;
  averageEarningLabel?: string;
  waitTimeHours: number;
  waitTimeLabel?: string;
  reviewCount: number;
}
