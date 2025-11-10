'use client';

import Link from 'next/link';
import { useCallback, useEffect, useMemo, useState } from 'react';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

type AdminReviewListItem = {
  id: string;
  storeId: string;
  storeName: string;
  branchName?: string;
  prefecture: string;
  category: string;
  visitedAt: string;
  rating: number;
  createdAt: string;
  contactEmail?: string;
};

function formatDate(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('ja-JP', { timeZone: 'Asia/Tokyo' });
}

export const AdminReviewDashboard = () => {
  const [reviews, setReviews] = useState<AdminReviewListItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [keyword, setKeyword] = useState('');

  const fetchReviews = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch(`${API_BASE}/api/admin/reviews`, {
        cache: 'no-store',
      });
      if (!response.ok) {
        throw new Error(`一覧取得に失敗しました (${response.status})`);
      }
      const data = (await response.json()) as { items: AdminReviewListItem[] };
      setReviews(Array.isArray(data.items) ? data.items : []);
    } catch (err) {
      setError(err instanceof Error ? err.message : '一覧取得に失敗しました');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void fetchReviews();
  }, [fetchReviews]);

  const filteredItems = useMemo(() => {
    if (!keyword.trim()) {
      return reviews;
    }
    const lower = keyword.trim().toLowerCase();
    return reviews.filter((item) => {
      const target = `${item.storeName} ${item.branchName ?? ''}`.toLowerCase();
      return target.includes(lower);
    });
  }, [keyword, reviews]);

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">アンケート一覧</h1>
          <p className="text-sm text-slate-500">
            最新の投稿から順に表示します。店舗名で絞り込み、詳細画面で編集してください。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <input
            type="text"
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            placeholder="店舗名で絞り込む"
            className="w-48 rounded-full border border-slate-200 px-4 py-2 text-sm focus:border-pink-400 focus:outline-none"
          />
          <button
            type="button"
            onClick={() => void fetchReviews()}
            className="rounded-full bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow disabled:opacity-60"
            disabled={loading}
          >
            再読み込み
          </button>
          <Link
            href="/admin/stores"
            className="rounded-full border border-pink-200 px-4 py-2 text-sm font-semibold text-pink-600 hover:bg-pink-50"
          >
            店舗一覧へ
          </Link>
        </div>
      </header>

      {error ? <p className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p> : null}

      <section className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
        {loading ? <p className="text-sm text-slate-500">読み込み中…</p> : null}
        {!loading && filteredItems.length === 0 ? (
          <p className="text-sm text-slate-500">該当するアンケートはありません。</p>
        ) : null}

        <ul className="divide-y divide-slate-100 text-sm">
          {filteredItems.map((item) => (
            <li key={item.id} className="space-y-2 px-2 py-3">
              <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                <Link
                  href={`/admin/reviews/${item.id}`}
                  className="text-base font-semibold text-slate-900 hover:text-pink-600"
                >
                  {item.branchName
                    ? `${item.storeName}（${item.branchName}）`
                    : item.storeName}
                </Link>
                <span className="text-xs text-slate-500">投稿: {formatDate(item.createdAt)}</span>
              </div>
              <div className="flex flex-wrap gap-3 text-xs text-slate-500">
                <span>{item.prefecture}</span>
                <span>業種: {item.category}</span>
                <span>訪問: {item.visitedAt || '-'}</span>
                <span>評価: {item.rating.toFixed(1)} / 5</span>
                <span>連絡先: {item.contactEmail || '—'}</span>
              </div>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
};
