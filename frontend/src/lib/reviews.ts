"use server";

import { MOCK_REVIEWS } from '@/data/mock-reviews';
import type {
  ReviewDetail,
  ReviewListResponse,
  ReviewSummary,
} from '@/types/review';

const API_BASE_URL = process.env.API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL;

type ReviewSearchParams = {
  prefecture?: string;
  category?: string;
  avgEarning?: number;
  storeName?: string;
  sort?: string;
  page?: number;
  limit?: number;
};

const DEFAULT_LIMIT = 10;

function toSummary(review: ReviewDetail): ReviewSummary {
  return {
    id: review.id,
    storeName: review.storeName,
    branchName: review.branchName,
    prefecture: review.prefecture,
    category: review.category,
    visitedAt: review.visitedAt,
    age: review.age,
    specScore: review.specScore,
    waitTimeHours: review.waitTimeHours,
    averageEarning: review.averageEarning,
    rating: review.rating,
    createdAt: review.createdAt,
    helpfulCount: review.helpfulCount,
    excerpt: review.excerpt,
  };
}

function filterMockReviews({
  prefecture,
  category,
  avgEarning,
  storeName,
}: ReviewSearchParams) {
  return MOCK_REVIEWS.filter((review) => {
    if (prefecture && review.prefecture !== prefecture) {
      return false;
    }
    if (category && review.category !== category) {
      return false;
    }
    if (avgEarning !== undefined && review.averageEarning !== avgEarning) {
      return false;
    }
    if (storeName && !review.storeName.includes(storeName)) {
      return false;
    }
    return true;
  });
}

function sortMockReviews(reviews: ReviewDetail[], sortKey?: string) {
  if (sortKey === 'helpful') {
    return [...reviews].sort(
      (a, b) => (b.helpfulCount ?? 0) - (a.helpfulCount ?? 0) || (a.createdAt < b.createdAt ? 1 : -1),
    );
  }

  if (sortKey === 'earning') {
    return [...reviews].sort(
      (a, b) => b.averageEarning - a.averageEarning || (a.createdAt < b.createdAt ? 1 : -1),
    );
  }

  // default: newest first
  return [...reviews].sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1));
}

export async function fetchReviews(
  params: ReviewSearchParams,
): Promise<ReviewListResponse> {
  const { page = 1, limit = DEFAULT_LIMIT } = params;

  if (!API_BASE_URL) {
    const filtered = sortMockReviews(filterMockReviews(params), params.sort);
    const start = (page - 1) * limit;
    const items = filtered.slice(start, start + limit).map(toSummary);
    return {
      items,
      page,
      limit,
      total: filtered.length,
    };
  }

  const url = new URL('/api/reviews', API_BASE_URL);
  if (params.prefecture) url.searchParams.set('prefecture', params.prefecture);
  if (params.category) url.searchParams.set('category', params.category);
  if (params.avgEarning !== undefined) {
    url.searchParams.set('avgEarning', String(params.avgEarning));
  }
  if (params.storeName) url.searchParams.set('storeName', params.storeName);
  if (params.sort) url.searchParams.set('sort', params.sort);
  url.searchParams.set('page', String(page));
  url.searchParams.set('limit', String(limit));

  const response = await fetch(url, {
    cache: 'no-store',
  });

  if (!response.ok) {
    throw new Error('レビューの取得に失敗しました');
  }

  const data = (await response.json()) as ReviewListResponse;
  return data;
}

export async function fetchReviewById(id: string): Promise<ReviewDetail | null> {
  if (!API_BASE_URL) {
    return MOCK_REVIEWS.find((review) => review.id === id) ?? null;
  }

  const response = await fetch(`${API_BASE_URL}/api/reviews/${id}`, {
    cache: 'no-store',
  });

  if (!response.ok) {
    if (response.status === 404) {
      return null;
    }
    throw new Error('レビュー詳細の取得に失敗しました');
  }

  return (await response.json()) as ReviewDetail;
}

export async function fetchFeaturedReviews() {
  if (!API_BASE_URL) {
    const latest = [...MOCK_REVIEWS]
      .sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1))
      .slice(0, 3)
      .map(toSummary);

    const highRated = [...MOCK_REVIEWS]
      .sort((a, b) => (b.helpfulCount ?? 0) - (a.helpfulCount ?? 0))
      .slice(0, 3)
      .map(toSummary);

    return { latest, highRated };
  }

  const [latestResponse, highRatedResponse] = await Promise.all([
    fetch(`${API_BASE_URL}/api/reviews/new`, { cache: 'no-store' }),
    fetch(`${API_BASE_URL}/api/reviews/high-rated`, { cache: 'no-store' }),
  ]);

  if (!latestResponse.ok || !highRatedResponse.ok) {
    throw new Error('特集レビューの取得に失敗しました');
  }

  return {
    latest: (await latestResponse.json()) as ReviewSummary[],
    highRated: (await highRatedResponse.json()) as ReviewSummary[],
  };
}
