'use client';

import { type ChangeEvent, type FormEvent, useCallback, useMemo, useState } from 'react';
import Link from 'next/link';

import {
  AGE_OPTIONS,
  AVERAGE_EARNING_OPTIONS,
  PREFECTURES,
  REVIEW_CATEGORIES,
  SPEC_MAX,
  SPEC_MAX_LABEL,
  SPEC_MIN,
  SPEC_MIN_LABEL,
  WAIT_TIME_OPTIONS,
} from '@/constants/filters';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

type AdminReview = {
  id: string;
  storeId: string;
  storeName: string;
  branchName?: string;
  prefecture: string;
  category: string;
  visitedAt: string;
  age: number;
  specScore: number;
  waitTimeHours: number;
  averageEarning: number;
  status: string;
  statusNote?: string;
  reviewedBy?: string;
  reviewedAt?: string;
  comment?: string;
  rewardStatus: string;
  rewardNote?: string;
  rewardSentAt?: string;
  reviewerId?: string;
  reviewerName?: string;
  reviewerHandle?: string;
  createdAt: string;
  updatedAt: string;
  rating: number;
};

type StoreCandidate = {
  id: string;
  name: string;
  branchName?: string;
  prefecture?: string;
  industryCodes: string[];
  reviewCount: number;
  lastReviewedAt?: string;
};

const STATUS_OPTIONS = [
  { value: 'pending', label: '審査中' },
  { value: 'approved', label: '掲載OK' },
  { value: 'rejected', label: '掲載不可' },
];

const REWARD_STATUS_OPTIONS = [
  { value: 'pending', label: '未処理' },
  { value: 'ready', label: '送付準備中' },
  { value: 'sent', label: '送付済み' },
];

function formatDate(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('ja-JP', { timeZone: 'Asia/Tokyo' });
}

const RATING_MIN = 0;
const RATING_MAX = 5;
const RATING_STEP = 0.5;

const formatSpecScoreLabel = (value: number) => {
  if (value <= SPEC_MIN) return SPEC_MIN_LABEL;
  if (value >= SPEC_MAX) return SPEC_MAX_LABEL;
  return `${value}`;
};

const StarDisplay = ({ value }: { value: number }) => (
  <span className="relative inline-block text-lg leading-none">
    <span className="text-slate-300">★★★★★</span>
    <span
      className="absolute left-0 top-0 overflow-hidden text-yellow-400"
      style={{ width: `${(value / RATING_MAX) * 100}%` }}
    >
      ★★★★★
    </span>
  </span>
);

export function AdminReviewEditor({ initialReview }: { initialReview: AdminReview }) {
  const [review, setReview] = useState<AdminReview>(initialReview);
  const [form, setForm] = useState({
    storeId: initialReview.storeId ?? '',
    storeName: initialReview.storeName,
    branchName: initialReview.branchName ?? '',
    prefecture: initialReview.prefecture,
    category: initialReview.category,
    visitedAt: initialReview.visitedAt,
    age: String(initialReview.age),
    specScore: String(initialReview.specScore),
    waitTimeHours: String(initialReview.waitTimeHours),
    averageEarning: String(initialReview.averageEarning),
    comment: initialReview.comment ?? '',
    rating: initialReview.rating.toString(),
  });
  const [statusForm, setStatusForm] = useState({
    status: initialReview.status,
    statusNote: initialReview.statusNote ?? '',
    reviewedBy: initialReview.reviewedBy ?? '',
    rewardStatus: initialReview.rewardStatus,
    rewardNote: initialReview.rewardNote ?? '',
  });
  const [savingContent, setSavingContent] = useState(false);
  const [savingStatus, setSavingStatus] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [storeCandidates, setStoreCandidates] = useState<StoreCandidate[]>([]);
  const [storeSearchQuery, setStoreSearchQuery] = useState(initialReview.storeName);
  const [storeSearchLoading, setStoreSearchLoading] = useState(false);
  const [storeSearchError, setStoreSearchError] = useState<string | null>(null);
  const [storeSearchExecuted, setStoreSearchExecuted] = useState(false);

  const contentBaseline = useMemo(
    () => ({
      storeId: review.storeId ?? '',
      storeName: review.storeName,
      branchName: review.branchName ?? '',
      prefecture: review.prefecture,
      category: review.category,
      visitedAt: review.visitedAt,
      age: String(review.age),
      specScore: String(review.specScore),
      waitTimeHours: String(review.waitTimeHours),
      averageEarning: String(review.averageEarning),
      comment: review.comment ?? '',
      rating: review.rating.toString(),
    }),
    [review],
  );

  const statusBaseline = useMemo(
    () => ({
      status: review.status,
      statusNote: review.statusNote ?? '',
      reviewedBy: review.reviewedBy ?? '',
      rewardStatus: review.rewardStatus,
      rewardNote: review.rewardNote ?? '',
    }),
    [review],
  );

  const isContentDirty = useMemo(() => {
    return Object.entries(contentBaseline).some(([key, value]) => {
      const formValue = form[key as keyof typeof form];
      return formValue !== value;
    });
  }, [contentBaseline, form]);

  const isStatusDirty = useMemo(() => {
    return Object.entries(statusBaseline).some(([key, value]) => {
      const formValue = statusForm[key as keyof typeof statusForm];
      return formValue !== value;
    });
  }, [statusBaseline, statusForm]);

  const handleContentChange = useCallback(
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) => {
      const { name, value } = event.target;
      setForm((prev) => ({ ...prev, [name]: value }));
    },
    [],
  );

  const handleStatusChange = useCallback(
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) => {
      const { name, value } = event.target;
      setStatusForm((prev) => ({ ...prev, [name]: value }));
    },
    [],
  );

  const handleStoreSearch = useCallback(async () => {
    if (!API_BASE) {
      setError('API_BASE_URL が未設定です');
      return;
    }
    if (!form.prefecture) {
      setStoreSearchError('先に都道府県を選択してください');
      return;
    }
    if (!form.category) {
      setStoreSearchError('業種を選択してください');
      return;
    }

    setStoreSearchLoading(true);
    setStoreSearchError(null);
    setStoreSearchExecuted(true);
    try {
      const params = new URLSearchParams();
      params.set('prefecture', form.prefecture);
      params.set('industry', form.category);
      if (storeSearchQuery.trim()) {
        params.set('q', storeSearchQuery.trim());
      }
      params.set('limit', '20');

      const response = await fetch(`${API_BASE}/api/admin/stores?${params.toString()}`, {
        cache: 'no-store',
      });
      if (!response.ok) {
        const data = await response.json().catch(() => null);
        const message =
          data && typeof data === 'object' && data !== null && 'error' in data
            ? (data as { error: string }).error
            : `店舗候補の取得に失敗しました (${response.status})`;
        throw new Error(message);
      }
      const payload = (await response.json()) as { items: StoreCandidate[] };
      setStoreCandidates(payload.items ?? []);
      if ((payload.items ?? []).length === 0) {
        setStoreSearchError('該当する店舗が見つかりませんでした。');
      }
    } catch (err) {
      setStoreSearchError(err instanceof Error ? err.message : '店舗候補の取得に失敗しました');
    } finally {
      setStoreSearchLoading(false);
    }
  }, [form.prefecture, form.category, storeSearchQuery]);

  const handleStoreSelect = useCallback((candidate: StoreCandidate) => {
    setForm((prev) => ({
      ...prev,
      storeId: candidate.id,
      storeName: candidate.name,
      branchName: candidate.branchName ?? '',
      prefecture: candidate.prefecture ?? prev.prefecture,
      category: candidate.industryCodes[0] ?? prev.category,
    }));
    setStoreSearchQuery(candidate.name);
    setStoreSearchError(null);
    setMessage(`店舗を「${candidate.name}${candidate.branchName ? ` ${candidate.branchName}` : ''}」に設定しました。`);
    setError(null);
  }, []);

  const handleStoreCreate = useCallback(async () => {
    if (!API_BASE) {
      setError('API_BASE_URL が未設定です');
      return;
    }
    if (!form.storeName.trim()) {
      setStoreSearchError('店舗名を入力してください');
      return;
    }
    if (!form.prefecture) {
      setStoreSearchError('都道府県を選択してください');
      return;
    }
    if (!form.category) {
      setStoreSearchError('業種を選択してください');
      return;
    }

    setStoreSearchLoading(true);
    setStoreSearchError(null);
    try {
      const payload = {
        name: form.storeName.trim(),
        branchName: form.branchName.trim(),
        prefecture: form.prefecture,
        industryCode: form.category,
      };
      const response = await fetch(`${API_BASE}/api/admin/stores`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      });
      if (!response.ok) {
        const data = await response.json().catch(() => null);
        const message =
          data && typeof data === 'object' && data !== null && 'error' in data
            ? (data as { error: string }).error
            : `店舗の登録に失敗しました (${response.status})`;
        throw new Error(message);
      }

      const data = (await response.json()) as { store: StoreCandidate; created: boolean };
      const createdStore = data.store;
      setForm((prev) => ({
        ...prev,
        storeId: createdStore.id,
        storeName: createdStore.name,
        branchName: createdStore.branchName ?? '',
        prefecture: createdStore.prefecture ?? prev.prefecture,
        category: createdStore.industryCodes[0] ?? prev.category,
      }));
      setStoreSearchQuery(createdStore.name);
      setStoreCandidates((prev) => {
        const filtered = prev.filter((item) => item.id !== createdStore.id);
        return [createdStore, ...filtered];
      });
      setStoreSearchExecuted(true);
      setMessage(
        data.created
          ? `店舗「${createdStore.name}${createdStore.branchName ? ` ${createdStore.branchName}` : ''}」を新規登録しました。`
          : `店舗「${createdStore.name}${createdStore.branchName ? ` ${createdStore.branchName}` : ''}」を選択しました。`,
      );
      setError(null);
    } catch (err) {
      setStoreSearchError(err instanceof Error ? err.message : '店舗の登録に失敗しました');
    } finally {
      setStoreSearchLoading(false);
    }
  }, [form.storeName, form.branchName, form.prefecture, form.category]);

  const handleContentSave = useCallback(
    async (event: FormEvent) => {
      event.preventDefault();
      if (!API_BASE) {
        setError('API_BASE_URL が未設定です');
        return;
      }
      if (!form.storeId) {
        setError('店舗候補から該当店舗を選択するか、新規店舗を登録してください');
        return;
      }
      setSavingContent(true);
      setMessage(null);
      setError(null);
      try {
        const payload: Record<string, unknown> = {
          storeId: form.storeId,
          storeName: form.storeName.trim(),
          branchName: form.branchName.trim(),
          prefecture: form.prefecture.trim(),
          category: form.category,
          visitedAt: form.visitedAt,
          age: Number(form.age),
          specScore: Number(form.specScore),
          waitTimeHours: Number(form.waitTimeHours),
          averageEarning: Number(form.averageEarning),
          comment: form.comment.trim(),
          rating: Number(form.rating),
        };

        const response = await fetch(`${API_BASE}/api/admin/reviews/${review.id}`, {
          method: 'PATCH',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(payload),
        });

        if (!response.ok) {
          const data = await response.json().catch(() => null);
          const message =
            data && typeof data === 'object' && data !== null && 'error' in data
              ? (data as { error: string }).error
              : `内容の更新に失敗しました (${response.status})`;
          throw new Error(message);
        }

        const updated = (await response.json()) as AdminReview;
        setReview(updated);
        setForm({
          storeId: updated.storeId ?? '',
          storeName: updated.storeName,
          branchName: updated.branchName ?? '',
          prefecture: updated.prefecture,
          category: updated.category,
          visitedAt: updated.visitedAt,
          age: String(updated.age),
          specScore: String(updated.specScore),
          waitTimeHours: String(updated.waitTimeHours),
          averageEarning: String(updated.averageEarning),
          comment: updated.comment ?? '',
          rating: updated.rating.toString(),
        });
        setStoreSearchQuery(updated.storeName);
        setMessage('アンケート内容を更新しました');
      } catch (err) {
        setError(err instanceof Error ? err.message : '内容の更新に失敗しました');
      } finally {
        setSavingContent(false);
      }
    },
    [form, review.id],
  );

  const handleStatusSave = useCallback(
    async (event: FormEvent) => {
      event.preventDefault();
      if (!API_BASE) {
        setError('API_BASE_URL が未設定です');
        return;
      }
      setSavingStatus(true);
      setMessage(null);
      setError(null);
      try {
        const payload = {
          status: statusForm.status,
          statusNote: statusForm.statusNote,
          reviewedBy: statusForm.reviewedBy,
          rewardStatus: statusForm.rewardStatus,
          rewardNote: statusForm.rewardNote,
        };

        const response = await fetch(`${API_BASE}/api/admin/reviews/${review.id}/status`, {
          method: 'PATCH',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(payload),
        });

        if (!response.ok) {
          const data = await response.json().catch(() => null);
          const message =
            data && typeof data === 'object' && data !== null && 'error' in data
              ? (data as { error: string }).error
              : `ステータスの更新に失敗しました (${response.status})`;
          throw new Error(message);
        }

        const updated = (await response.json()) as AdminReview;
        setReview(updated);
        setStatusForm({
          status: updated.status,
          statusNote: updated.statusNote ?? '',
          reviewedBy: updated.reviewedBy ?? '',
          rewardStatus: updated.rewardStatus,
          rewardNote: updated.rewardNote ?? '',
        });
        setMessage('ステータスを更新しました');
      } catch (err) {
        setError(err instanceof Error ? err.message : 'ステータスの更新に失敗しました');
      } finally {
        setSavingStatus(false);
      }
    },
    [review.id, statusForm],
  );

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-900">アンケート編集</h1>
        <Link
          href="/admin/reviews"
          className="text-sm font-semibold text-pink-600 hover:text-pink-500"
        >
          ⟵ 一覧に戻る
        </Link>
      </div>

      {message ? <p className="rounded-lg bg-emerald-50 px-4 py-3 text-sm text-emerald-700">{message}</p> : null}
      {error ? <p className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p> : null}

      <section className="space-y-4 rounded-2xl border border-slate-100 bg-white p-6 shadow-sm">
        <header className="space-y-1">
          <h2 className="text-lg font-semibold text-slate-900">アンケート内容</h2>
          <p className="text-sm text-slate-500">投稿内容を編集し、保存してください。</p>
        </header>

        <form className="grid gap-4" onSubmit={handleContentSave}>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">店舗名</span>
              <input
                name="storeName"
                value={form.storeName}
                onChange={handleContentChange}
                placeholder="例: やりすぎ娘"
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
                required
              />
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">支店名</span>
              <input
                name="branchName"
                value={form.branchName}
                onChange={handleContentChange}
                placeholder="例: 新宿店"
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
              />
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">都道府県</span>
              <select
                name="prefecture"
                value={form.prefecture}
                onChange={handleContentChange}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
                required
              >
                <option value="">選択してください</option>
                {PREFECTURES.map((prefecture) => (
                  <option key={prefecture} value={prefecture}>
                    {prefecture}
                  </option>
                ))}
              </select>
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">業種</span>
              <select
                name="category"
                value={form.category}
                onChange={handleContentChange}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
                required
              >
                <option value="">選択してください</option>
                {REVIEW_CATEGORIES.map((category) => (
                  <option key={category.value} value={category.value}>
                    {category.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">働いた時期</span>
              <input
                type="month"
                name="visitedAt"
                value={form.visitedAt}
                onChange={handleContentChange}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
                required
              />
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">年齢</span>
              <select
                name="age"
                value={form.age}
                onChange={handleContentChange}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
                required
              >
                {AGE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="space-y-2 text-sm sm:col-span-2">
              <span className="font-semibold text-slate-700">スペック</span>
              <input
                type="range"
                name="specScore"
                value={Number(form.specScore) || SPEC_MIN}
                onChange={handleContentChange}
                min={SPEC_MIN}
                max={SPEC_MAX}
                step={1}
                className="w-full accent-pink-500"
              />
              <div className="flex items-center justify-between text-xs text-slate-500">
                <span>{SPEC_MIN_LABEL}</span>
                <span className="text-sm font-semibold text-slate-700">
                  {formatSpecScoreLabel(Number(form.specScore) || SPEC_MIN)}
                </span>
                <span>{SPEC_MAX_LABEL}</span>
              </div>
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">待機時間</span>
              <select
                name="waitTimeHours"
                value={form.waitTimeHours}
                onChange={handleContentChange}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
                required
              >
                {WAIT_TIME_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">平均稼ぎ</span>
              <select
                name="averageEarning"
                value={form.averageEarning}
                onChange={handleContentChange}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
                required
              >
                {AVERAGE_EARNING_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <div className="space-y-3 rounded-xl border border-slate-200 bg-slate-50/60 p-4">
            <div className="flex flex-wrap items-end gap-3">
              <label className="flex-1 min-w-[220px] space-y-1 text-sm">
                <span className="font-semibold text-slate-700">店舗検索キーワード</span>
                <input
                  value={storeSearchQuery}
                  onChange={(event) => setStoreSearchQuery(event.target.value)}
                  placeholder="店舗名や一部キーワードを入力"
                  className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
                />
              </label>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={handleStoreSearch}
                  className="rounded-full bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow disabled:opacity-60"
                  disabled={storeSearchLoading}
                >
                  {storeSearchLoading ? '検索中…' : '候補を表示'}
                </button>
                <button
                  type="button"
                  onClick={handleStoreCreate}
                  className="rounded-full border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 transition hover:border-pink-400 hover:text-pink-600 disabled:opacity-60"
                  disabled={storeSearchLoading}
                >
                  新規店舗を登録
                </button>
              </div>
            </div>

            <p className="text-xs text-slate-500">
              現在の選択:{' '}
              {form.storeId
                ? `${form.storeName}${form.branchName ? `（${form.branchName}）` : ''} / ${form.prefecture} / ${form.category}`
                : '未選択です。候補から選ぶか新規店舗を登録してください。'}
            </p>

            {storeSearchError ? (
              <p className="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700">{storeSearchError}</p>
            ) : null}

            {storeSearchLoading ? (
              <p className="text-xs text-slate-500">店舗候補を取得しています…</p>
            ) : storeCandidates.length > 0 ? (
              <ul className="divide-y divide-slate-200 rounded-lg border border-slate-200 bg-white">
                {storeCandidates.map((candidate) => {
                  const selected = form.storeId === candidate.id;
                  return (
                    <li key={candidate.id} className="p-3">
                      <button
                        type="button"
                        onClick={() => handleStoreSelect(candidate)}
                        className={`flex w-full flex-col items-start gap-1 rounded-md px-2 py-1 text-left transition ${
                          selected ? 'bg-pink-50 text-pink-700' : 'hover:bg-slate-100'
                        }`}
                      >
                        <span className="text-sm font-semibold">
                          {candidate.name}
                          {candidate.branchName ? `（${candidate.branchName}）` : ''}
                        </span>
                        <span className="text-xs text-slate-500">
                          {candidate.prefecture ?? '都道府県不明'} / 評価件数 {candidate.reviewCount}
                          {candidate.industryCodes.length > 0 ? ` / 業種: ${candidate.industryCodes.join(', ')}` : ''}
                        </span>
                      </button>
                    </li>
                  );
                })}
              </ul>
            ) : storeSearchExecuted ? (
              <p className="text-xs text-slate-500">条件に一致する店舗が見つかりませんでした。</p>
            ) : (
              <p className="text-xs text-slate-500">
                都道府県と業種を選び、検索ボタンから既存店舗を探してください。
              </p>
            )}
          </div>

          <label className="space-y-1 text-sm">
            <span className="font-semibold text-slate-700">客層・スタッフ・環境等について</span>
            <textarea
              name="comment"
              value={form.comment}
              onChange={handleContentChange}
              rows={6}
              placeholder="客層、スタッフ対応、待機環境などの気付きや感想を入力してください"
              className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
            />
          </label>

          <label className="space-y-2 text-sm">
            <span className="font-semibold text-slate-700">満足度</span>
            <div className="flex items-center gap-3">
              <StarDisplay value={Number(form.rating) || 0} />
              <span className="text-xs text-slate-500">
                {(Number(form.rating) || 0).toFixed(1)} / {RATING_MAX.toFixed(1)}
              </span>
            </div>
            <input
              type="range"
              name="rating"
              value={Number(form.rating) || 0}
              onChange={handleContentChange}
              min={RATING_MIN}
              max={RATING_MAX}
              step={RATING_STEP}
              className="w-full accent-pink-500"
            />
            <div className="flex justify-between text-xs text-slate-500">
              <span>0</span>
              <span>2.5</span>
              <span>5.0</span>
            </div>
          </label>

          <div className="flex justify-end">
            <button
              type="submit"
              className="rounded-full bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow disabled:opacity-60"
              disabled={savingContent || !isContentDirty}
            >
              {savingContent ? '保存中…' : '更新する'}
            </button>
          </div>
        </form>
      </section>

      <section className="space-y-4 rounded-2xl border border-slate-100 bg-white p-6 shadow-sm">
        <header className="space-y-1">
          <h2 className="text-lg font-semibold text-slate-900">ステータス・報酬管理</h2>
          <p className="text-sm text-slate-500">審査メモや PayPay 送付状況を更新してください。</p>
        </header>

        <form className="grid gap-4" onSubmit={handleStatusSave}>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">審査ステータス</span>
              <select
                name="status"
                value={statusForm.status}
                onChange={handleStatusChange}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
                required
              >
                {STATUS_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">審査担当</span>
              <input
                name="reviewedBy"
                value={statusForm.reviewedBy}
                onChange={handleStatusChange}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
              />
            </label>
            <label className="space-y-1 text-sm sm:col-span-2">
              <span className="font-semibold text-slate-700">審査メモ</span>
              <textarea
                name="statusNote"
                value={statusForm.statusNote}
                onChange={handleStatusChange}
                rows={3}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
              />
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">報酬ステータス</span>
              <select
                name="rewardStatus"
                value={statusForm.rewardStatus}
                onChange={handleStatusChange}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
                required
              >
                {REWARD_STATUS_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-semibold text-slate-700">報酬メモ</span>
              <textarea
                name="rewardNote"
                value={statusForm.rewardNote}
                onChange={handleStatusChange}
                rows={3}
                className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
              />
            </label>
          </div>

          <div className="flex justify-end">
            <button
              type="submit"
              className="rounded-full bg-pink-600 px-4 py-2 text-sm font-semibold text-white shadow hover:bg-pink-500 disabled:opacity-60"
              disabled={savingStatus || !isStatusDirty}
            >
              {savingStatus ? '更新中…' : 'ステータスを更新する'}
            </button>
          </div>
        </form>
      </section>

      <section className="rounded-2xl border border-slate-100 bg-white p-6 shadow-sm">
        <h2 className="text-lg font-semibold text-slate-900">メタ情報</h2>
        <dl className="mt-4 grid gap-2 text-sm text-slate-600">
          <div className="flex gap-3">
            <dt className="w-32 font-semibold">投稿ID</dt>
            <dd>{review.reviewerId ?? '—'}</dd>
          </div>
          <div className="flex gap-3">
            <dt className="w-32 font-semibold">投稿者</dt>
            <dd>
              {review.reviewerHandle ? `@${review.reviewerHandle}` : review.reviewerName ?? '匿名'}
            </dd>
          </div>
          <div className="flex gap-3">
            <dt className="w-32 font-semibold">総評</dt>
            <dd>{review.rating.toFixed(1)} / 5</dd>
          </div>
          <div className="flex gap-3">
            <dt className="w-32 font-semibold">投稿日時</dt>
            <dd>{formatDate(review.createdAt)}</dd>
          </div>
          <div className="flex gap-3">
            <dt className="w-32 font-semibold">最終更新</dt>
            <dd>{formatDate(review.updatedAt)}</dd>
          </div>
          <div className="flex gap-3">
            <dt className="w-32 font-semibold">審査日時</dt>
            <dd>{formatDate(review.reviewedAt)}</dd>
          </div>
          <div className="flex gap-3">
            <dt className="w-32 font-semibold">報酬送付日時</dt>
            <dd>{formatDate(review.rewardSentAt)}</dd>
          </div>
        </dl>
      </section>
    </div>
  );
}
