'use client';

import { useState, useMemo, useCallback } from 'react';
import { useRouter } from 'next/navigation';

import {
  AGE_OPTIONS,
  AVERAGE_EARNING_OPTIONS,
  REVIEW_CATEGORIES,
  SPEC_MAX,
  SPEC_MAX_LABEL,
  SPEC_MIN,
  SPEC_MIN_LABEL,
  WAIT_TIME_OPTIONS,
} from '@/constants/filters';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

export type AdminStoreSummary = {
  id: string;
  name: string;
  branchName?: string;
  prefecture?: string;
  area?: string;
  genre?: string;
  businessHours?: string;
  priceRange?: string;
  industryCodes: string[];
};

const canonicalCategoryValue = (input?: string) => {
  if (!input) return '';
  const byValue = REVIEW_CATEGORIES.find((item) => item.value === input);
  if (byValue) return byValue.value;
  const byLabel = REVIEW_CATEGORIES.find((item) => item.label === input);
  if (byLabel) return byLabel.value;
  return input;
};

const currentMonth = () => {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
};

export function AdminStoreReviewForm({ store }: { store: AdminStoreSummary }) {
  const router = useRouter();
  const defaultCategory = canonicalCategoryValue(store.industryCodes[0]);
  const [form, setForm] = useState({
    industryCode: defaultCategory,
    visitedAt: currentMonth(),
    age: String(AGE_OPTIONS[0]?.value ?? 18),
    specScore: '90',
    waitTimeHours: String(WAIT_TIME_OPTIONS[0]?.value ?? 1),
    averageEarning: String(AVERAGE_EARNING_OPTIONS[0]?.value ?? 0),
    comment: '',
    rating: '3',
    contactEmail: '',
  });
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const storeHeading = useMemo(() => {
    return `${store.name}${store.branchName ? `（${store.branchName}）` : ''}`;
  }, [store.name, store.branchName]);

  const handleChange = useCallback(
    (event: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => {
      const { name, value } = event.target;
      setForm((prev) => ({ ...prev, [name]: value }));
    },
    [],
  );

  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (!API_BASE) {
        setError('API_BASE_URL が未設定です。env を確認してください。');
        return;
      }
      setSubmitting(true);
      setMessage(null);
      setError(null);
      try {
        const payload = {
          industryCode: form.industryCode,
          visitedAt: form.visitedAt,
          age: Number(form.age),
          specScore: Number(form.specScore),
          waitTimeHours: Number(form.waitTimeHours),
          averageEarning: Number(form.averageEarning),
          comment: form.comment.trim(),
          rating: Number(form.rating),
          contactEmail: form.contactEmail.trim(),
        };
        const response = await fetch(`${API_BASE}/api/admin/stores/${store.id}/reviews`, {
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
              : `登録に失敗しました (${response.status})`;
          throw new Error(message);
        }
        const created = (await response.json()) as { id: string };
        setMessage('アンケートを登録しました');
        router.push(`/admin/reviews/${created.id}`);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'アンケートの登録に失敗しました');
      } finally {
        setSubmitting(false);
      }
    },
    [form, router, store.id],
  );

  return (
    <form onSubmit={handleSubmit} className="space-y-6 rounded-2xl border border-slate-100 bg-white p-6 shadow-sm">
      <div className="space-y-1">
        <h2 className="text-2xl font-semibold text-slate-900">{storeHeading}</h2>
        <p className="text-sm text-slate-500">店舗に紐づくアンケートを登録します。</p>
        <dl className="mt-3 grid gap-2 text-xs text-slate-500 sm:grid-cols-2">
          <div className="flex gap-2">
            <dt className="font-semibold">都道府県</dt>
            <dd>{store.prefecture || '-'}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="font-semibold">エリア</dt>
            <dd>{store.area || '-'}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="font-semibold">ジャンル</dt>
            <dd>{store.genre || '-'}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="font-semibold">業種</dt>
            <dd>{store.industryCodes.join(', ') || '-'}</dd>
          </div>
        </dl>
      </div>

      {message ? <p className="rounded-lg bg-emerald-50 px-4 py-3 text-sm text-emerald-700">{message}</p> : null}
      {error ? <p className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p> : null}

      <div className="grid gap-4 sm:grid-cols-2">
        <label className="space-y-1 text-sm">
          <span className="font-semibold text-slate-700">業種</span>
          <select
            name="industryCode"
            value={form.industryCode}
            onChange={handleChange}
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
          <span className="font-semibold text-slate-700">訪問時期</span>
          <input
            type="month"
            name="visitedAt"
            value={form.visitedAt}
            onChange={handleChange}
            className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
            required
          />
        </label>
        <label className="space-y-1 text-sm">
          <span className="font-semibold text-slate-700">年齢</span>
          <select
            name="age"
            value={form.age}
            onChange={handleChange}
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
        <label className="space-y-1 text-sm">
          <span className="font-semibold text-slate-700">待機時間</span>
          <select
            name="waitTimeHours"
            value={form.waitTimeHours}
            onChange={handleChange}
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
            onChange={handleChange}
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

      <label className="flex flex-col space-y-2 text-sm">
        <span className="font-semibold text-slate-700">スペック</span>
        <input
          type="range"
          name="specScore"
          value={Number(form.specScore) || SPEC_MIN}
          onChange={handleChange}
          min={SPEC_MIN}
          max={SPEC_MAX}
          className="w-full accent-pink-500"
        />
        <div className="flex items-center justify-between text-xs text-slate-500">
          <span>{SPEC_MIN_LABEL}</span>
          <span className="text-sm font-semibold text-slate-700">
            {Number(form.specScore) || SPEC_MIN}
          </span>
          <span>{SPEC_MAX_LABEL}</span>
        </div>
      </label>

      <label className="flex flex-col space-y-1 text-sm">
        <span className="font-semibold text-slate-700">満足度</span>
        <input
          type="range"
          name="rating"
          value={Number(form.rating)}
          onChange={handleChange}
          min={0}
          max={5}
          step={0.5}
          className="w-full accent-pink-500"
        />
        <span className="text-xs text-slate-500">{Number(form.rating).toFixed(1)} / 5</span>
      </label>

      <label className="flex flex-col space-y-1 text-sm">
        <span className="font-semibold text-slate-700">連絡先メール（任意）</span>
        <input
          type="email"
          name="contactEmail"
          value={form.contactEmail}
          onChange={handleChange}
          placeholder="example@example.com"
          className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
        />
      </label>

      <label className="flex flex-col space-y-1 text-sm">
        <span className="font-semibold text-slate-700">アンケート内容</span>
        <textarea
          name="comment"
          value={form.comment}
          onChange={handleChange}
          rows={6}
          placeholder="勤務実態や待遇などを記録してください"
          className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-pink-400 focus:outline-none"
        />
      </label>

      <div className="flex justify-end">
        <button
          type="submit"
          className="rounded-full bg-pink-600 px-6 py-2 text-sm font-semibold text-white shadow disabled:opacity-60"
          disabled={submitting}
        >
          {submitting ? '登録中…' : 'アンケートを登録'}
        </button>
      </div>
    </form>
  );
}
