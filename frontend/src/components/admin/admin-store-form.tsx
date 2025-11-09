'use client';

import { useEffect, useState } from 'react';

import { PREFECTURES, REVIEW_CATEGORIES } from '@/constants/filters';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

const AREA_OPTIONS = ['吉原', '雄琴', '福原', '金津園', 'すすきの', '中洲'];
const GENRE_OPTIONS = ['スタンダード', '人妻・熟女', '学園系', 'ギャル', 'OL・姉系', 'ぽっちゃり'];

const DEFAULT_FORM = {
  name: '',
  branchName: '',
  prefecture: '',
  area: '',
  industryCode: '',
  genre: '',
  businessHours: '',
  priceRange: '',
};

type StoreFormValues = typeof DEFAULT_FORM;

type AdminStoreFormProps = {
  initialValues?: Partial<StoreFormValues> & { id?: string };
  onSubmitted?: () => void;
  title?: string;
  submitLabel?: string;
  showReset?: boolean;
};

export function AdminStoreForm({
  initialValues,
  onSubmitted,
  title = '店舗を登録する',
  submitLabel = '登録する',
  showReset = true,
}: AdminStoreFormProps = {}) {
  const [form, setForm] = useState<StoreFormValues>(DEFAULT_FORM);
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const isEditing = Boolean(initialValues);

  useEffect(() => {
    if (initialValues) {
      setForm({ ...DEFAULT_FORM, ...initialValues });
    } else {
      setForm(DEFAULT_FORM);
    }
  }, [initialValues]);

  const updateField = (field: keyof StoreFormValues, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    setMessage(null);
    setError(null);

    try {
      const response = await fetch(`${API_BASE}/api/admin/stores`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: form.name,
          branchName: form.branchName,
          prefecture: form.prefecture,
          area: form.area,
          industryCode: form.industryCode,
          genre: form.genre,
          businessHours: form.businessHours,
          priceRange: form.priceRange,
        }),
      });
      if (!response.ok) {
        const payload = (await response.json().catch(() => null)) as { error?: string } | null;
        throw new Error(payload?.error ?? `登録に失敗しました (${response.status})`);
      }

      setMessage(isEditing ? '店舗情報を更新しました' : '店舗を登録しました');
      if (!isEditing) {
        setForm(DEFAULT_FORM);
      }
      if (onSubmitted) {
        onSubmitted();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '店舗登録に失敗しました');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6 rounded-2xl border border-slate-100 bg-white p-6 shadow-sm">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold text-slate-900">{title}</h1>
        <p className="text-sm text-slate-500">※ が付いた項目は必須です。その他は任意で入力できます。</p>
      </div>

      {message ? <p className="rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700">{message}</p> : null}
      {error ? <p className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p> : null}

      <div className="space-y-5">
        <label className="flex flex-col gap-1 text-sm text-slate-800">
          <span className="flex items-center gap-2 font-semibold">
            店名 <span className="text-pink-600">※</span>
          </span>
          <input
            required
            value={form.name}
            onChange={(event) => updateField('name', event.target.value)}
            className="rounded-lg border border-slate-200 px-3 py-3 text-base focus:border-pink-500 focus:outline-none"
            placeholder="例) ルミナス東京"
          />
        </label>

        <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
          支店名（任意）
          <input
            value={form.branchName}
            onChange={(event) => updateField('branchName', event.target.value)}
            className="rounded-lg border border-slate-200 px-3 py-3 text-base focus:border-pink-500 focus:outline-none"
            placeholder="例) 新宿本店"
          />
        </label>

        <label className="flex flex-col gap-1 text-sm text-slate-800">
          <span className="flex items-center gap-2 font-semibold">
            都道府県 <span className="text-pink-600">※</span>
          </span>
          <select
            value={form.prefecture}
            onChange={(event) => updateField('prefecture', event.target.value)}
            className="rounded-lg border border-slate-200 px-3 py-3 text-base focus:border-pink-500 focus:outline-none"
            required
          >
            <option value="">選択してください</option>
            {PREFECTURES.map((pref) => (
              <option key={pref} value={pref}>
                {pref}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
          エリア（任意）
          <select
            value={form.area}
            onChange={(event) => updateField('area', event.target.value)}
            className="rounded-lg border border-slate-200 px-3 py-3 text-base focus:border-pink-500 focus:outline-none"
          >
            <option value="">選択してください</option>
            {AREA_OPTIONS.map((area) => (
              <option key={area} value={area}>
                {area}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1 text-sm text-slate-800">
          <span className="flex items-center gap-2 font-semibold">
            業種 <span className="text-pink-600">※</span>
          </span>
          <select
            value={form.industryCode}
            onChange={(event) => updateField('industryCode', event.target.value)}
            className="rounded-lg border border-slate-200 px-3 py-3 text-base focus:border-pink-500 focus:outline-none"
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

        <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
          ジャンル（任意）
          <select
            value={form.genre}
            onChange={(event) => updateField('genre', event.target.value)}
            className="rounded-lg border border-slate-200 px-3 py-3 text-base focus:border-pink-500 focus:outline-none"
          >
            <option value="">選択してください</option>
            {GENRE_OPTIONS.map((genre) => (
              <option key={genre} value={genre}>
                {genre}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
          営業時間（任意）
          <input
            value={form.businessHours}
            onChange={(event) => updateField('businessHours', event.target.value)}
            className="rounded-lg border border-slate-200 px-3 py-3 text-base focus:border-pink-500 focus:outline-none"
            placeholder="例) 10:00〜翌5:00"
          />
        </label>

        <label className="flex flex-col gap-1 text-sm font-medium text-slate-700">
          料金目安（任意）
          <input
            value={form.priceRange}
            onChange={(event) => updateField('priceRange', event.target.value)}
            className="rounded-lg border border-slate-200 px-3 py-3 text-base focus:border-pink-500 focus:outline-none"
            placeholder="例) 60分 12,000円〜"
          />
        </label>
      </div>

      <div className="flex flex-col items-stretch gap-3 sm:flex-row sm:justify-end">
        {showReset ? (
          <button
            type="button"
            onClick={() => setForm(initialValues ? { ...DEFAULT_FORM, ...initialValues } : DEFAULT_FORM)}
            className="rounded-full border border-slate-200 px-4 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-50"
            disabled={submitting}
          >
            リセット
          </button>
        ) : null}
        <button
          type="submit"
          disabled={submitting}
          className="rounded-full bg-pink-600 px-6 py-2 text-sm font-semibold text-white shadow disabled:opacity-60"
        >
          {submitting ? '送信中…' : submitLabel}
        </button>
      </div>
    </form>
  );
}
