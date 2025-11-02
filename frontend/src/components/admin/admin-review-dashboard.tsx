'use client';

import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

type StatusOption = 'pending' | 'approved' | 'rejected' | 'all';

type AdminReviewListItem = {
  id: string;
  storeName: string;
  branchName?: string;
  prefecture: string;
  category: string;
  status: string;
  rewardStatus: string;
  createdAt: string;
  reviewerHandle?: string;
  rating: number;
};

const STATUS_LABEL: Record<string, string> = {
  pending: '審査中',
  approved: '掲載OK',
  rejected: '掲載不可',
};

const REWARD_STATUS_LABEL: Record<string, string> = {
  pending: '未処理',
  ready: '送付準備中',
  sent: '送付済み',
};

const statusOptions: { value: StatusOption; label: string }[] = [
  { value: 'pending', label: '審査中' },
  { value: 'approved', label: '掲載OK' },
  { value: 'rejected', label: '掲載不可' },
  { value: 'all', label: 'すべて' },
];

function formatDate(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('ja-JP', { timeZone: 'Asia/Tokyo' });
}

export const AdminReviewDashboard = () => {
  const [statusFilter, setStatusFilter] = useState<StatusOption>('pending');
  const [reviews, setReviews] = useState<AdminReviewListItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchReviews = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const query = statusFilter === 'all' ? '' : `?status=${statusFilter}`;
      const response = await fetch(`${API_BASE}/api/admin/reviews${query}`, {
        cache: 'no-store',
      });
      if (!response.ok) {
        throw new Error(`一覧取得に失敗しました (${response.status})`);
      }
      const data = (await response.json()) as { items: AdminReviewListItem[] };
      setReviews(data.items);
    } catch (err) {
      setError(err instanceof Error ? err.message : '一覧取得に失敗しました');
    } finally {
      setLoading(false);
    }
  }, [statusFilter]);

  useEffect(() => {
    void fetchReviews();
  }, [fetchReviews]);

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">アンケート一覧</h1>
          <p className="text-sm text-slate-500">
            審査状況で絞り込み、詳細ページから内容の確認・編集を行ってください。
          </p>
        </div>
        <div className="flex items-center gap-3">
          <select
            value={statusFilter}
            onChange={(event) => setStatusFilter(event.target.value as StatusOption)}
            className="rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
          >
            {statusOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <button
            type="button"
            onClick={() => void fetchReviews()}
            className="rounded-full bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow disabled:opacity-60"
            disabled={loading}
          >
            再読み込み
          </button>
        </div>
      </header>

      {error ? <p className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p> : null}

      <section className="rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
        {loading ? <p className="text-sm text-slate-500">読み込み中…</p> : null}
        {!loading && reviews.length === 0 ? (
          <p className="text-sm text-slate-500">該当するアンケートはありません。</p>
        ) : null}

        <ul className="divide-y divide-slate-100 text-sm">
          {reviews.map((item) => (
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
                <div className="flex flex-wrap gap-2 text-xs text-slate-500">
                  <span className="rounded-full bg-slate-100 px-3 py-1 font-medium">
                    {STATUS_LABEL[item.status] ?? item.status}
                  </span>
                  <span className="rounded-full bg-slate-100 px-3 py-1 font-medium">
                    報酬: {REWARD_STATUS_LABEL[item.rewardStatus] ?? item.rewardStatus}
                  </span>
                </div>
              </div>
              <div className="flex flex-wrap gap-3 text-xs text-slate-500">
                <span>{item.prefecture}</span>
                <span>投稿: {formatDate(item.createdAt)}</span>
                <span>評価: {item.rating.toFixed(1)} / 5</span>
                {item.reviewerHandle ? <span>@{item.reviewerHandle}</span> : null}
              </div>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
};
