"use server";

import { MOCK_REVIEWS } from '@/data/mock-reviews';
import type { StoreSummary } from '@/types/review';

const API_BASE_URL = process.env.API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL;

type StoreSearchParams = {
  prefecture?: string;
  category?: string;
  avgEarning?: number;
  page?: number;
  limit?: number;
};

const DEFAULT_LIMIT = 10;

function aggregateMockStores(): StoreSummary[] {
  const storeMap = new Map<string, StoreSummary>();

  MOCK_REVIEWS.forEach((review) => {
    const existing = storeMap.get(review.storeName);
    if (existing) {
      existing.reviewCount += 1;
      existing.averageEarning =
        Math.round(((existing.averageEarning * (existing.reviewCount - 1) + review.averageEarning) /
          existing.reviewCount) *
          10) / 10;
      existing.averageEarningLabel = `${existing.averageEarning}万円`;
      existing.waitTimeHours =
        Math.round(((existing.waitTimeHours * (existing.reviewCount - 1) + review.waitTimeHours) /
          existing.reviewCount) *
          10) / 10;
      existing.waitTimeLabel = `${existing.waitTimeHours}時間`;
    } else {
      storeMap.set(review.storeName, {
        id: `store-${storeMap.size + 1}`,
        storeName: review.storeName,
        prefecture: review.prefecture,
        category: review.category,
        averageEarning: review.averageEarning,
        averageEarningLabel: `${review.averageEarning}万円`,
        waitTimeHours: review.waitTimeHours,
        waitTimeLabel: `${review.waitTimeHours}時間`,
        reviewCount: 1,
      });
    }
  });

  return Array.from(storeMap.values());
}

function filterStores(stores: StoreSummary[], params: StoreSearchParams) {
  const { prefecture, category, avgEarning } = params;
  return stores.filter((store) => {
    if (prefecture && store.prefecture !== prefecture) return false;
    if (category && store.category !== category) return false;
    if (avgEarning !== undefined && Math.round(store.averageEarning) !== avgEarning) return false;
    return true;
  });
}

export async function fetchStores(params: StoreSearchParams) {
  const { page = 1, limit = DEFAULT_LIMIT } = params;

  if (!API_BASE_URL) {
    const stores = aggregateMockStores();
    const filtered = filterStores(stores, params);
    const start = (page - 1) * limit;
    return {
      items: filtered.slice(start, start + limit),
      page,
      limit,
      total: filtered.length,
    };
  }

  const url = new URL('/api/stores', API_BASE_URL);
  if (params.prefecture) url.searchParams.set('prefecture', params.prefecture);
  if (params.category) url.searchParams.set('category', params.category);
  if (params.avgEarning !== undefined) {
    url.searchParams.set('avgEarning', String(params.avgEarning));
  }
  url.searchParams.set('page', String(page));
  url.searchParams.set('limit', String(limit));

  const response = await fetch(url, {
    cache: 'no-store',
  });

  if (!response.ok) {
    throw new Error('店舗情報の取得に失敗しました');
  }

  return (await response.json()) as {
    items: StoreSummary[];
    page: number;
    limit: number;
    total: number;
  };
}
