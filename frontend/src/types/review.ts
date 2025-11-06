export type ReviewCategory =
  | 'deriheru'
  | 'hoteheru'
  | 'hakoheru'
  | 'sopu'
  | 'dc'
  | 'huesu'
  | 'menesu';

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
  averageRating: number;
  averageEarning: number;
  averageEarningLabel?: string;
  waitTimeHours: number;
  waitTimeLabel?: string;
  reviewCount: number;
}
