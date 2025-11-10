'use client';

import Link from 'next/link';
import { useCallback, useEffect, useMemo, useState } from 'react';

import { PREFECTURES, REVIEW_CATEGORIES } from '@/constants/filters';

import { AdminStoreForm } from './admin-store-form';

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

type AdminStoreItem = {
  id: string;
  name: string;
  branchName?: string;
  prefecture?: string;
  area?: string;
  genre?: string;
  businessHours?: string;
  priceRange?: string;
  industryCodes: string[];
  reviewCount: number;
  lastReviewedAt?: string;
};

const INDUSTRY_LABEL: Record<string, string> = REVIEW_CATEGORIES.reduce(
  (acc, item) => ({ ...acc, [item.value]: item.label }),
  {} as Record<string, string>,
);

function formatDate(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString('ja-JP', { timeZone: 'Asia/Tokyo' });
}

const emptyFilters = { prefecture: '', industry: '', keyword: '' };

export function AdminStoreDashboard() {
  const [filterForm, setFilterForm] = useState(emptyFilters);
  const [query, setQuery] = useState(emptyFilters);
  const [stores, setStores] = useState<AdminStoreItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [editingStore, setEditingStore] = useState<AdminStoreItem | null>(null);

  const fetchStores = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const searchParams = new URLSearchParams();
      if (query.prefecture) searchParams.set('prefecture', query.prefecture);
      if (query.industry) searchParams.set('industry', query.industry);
      if (query.keyword) searchParams.set('q', query.keyword);
      const response = await fetch(`${API_BASE}/api/admin/stores?${searchParams.toString()}`, {
        cache: 'no-store',
      });
      if (!response.ok) {
        throw new Error(`店舗一覧の取得に失敗しました (${response.status})`);
      }
      const data = (await response.json()) as { items: AdminStoreItem[] };
      setStores(data.items ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : '店舗一覧の取得に失敗しました');
    } finally {
      setLoading(false);
    }
  }, [query]);

  useEffect(() => {
    void fetchStores();
  }, [fetchStores]);

  const handleSearch = () => {
    setQuery(filterForm);
  };

  const handleResetFilters = () => {
    setFilterForm(emptyFilters);
    setQuery(emptyFilters);
  };

  const editingInitialValues = useMemo(() => {
    if (!editingStore) return undefined;
    return {
      name: editingStore.name,
      branchName: editingStore.branchName ?? '',
      prefecture: editingStore.prefecture ?? '',
      area: editingStore.area ?? '',
      industryCode: editingStore.industryCodes[0] ?? '',
      genre: editingStore.genre ?? '',
      businessHours: editingStore.businessHours ?? '',
      priceRange: editingStore.priceRange ?? '',
    };
  }, [editingStore]);

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-slate-900">店舗一覧</h1>
          <p className="text-sm text-slate-500">登録済み店舗の絞り込み・照会・編集を行えます。</p>
        </div>
        <Link
          href="/admin/stores/new"
          className="rounded-full border border-pink-200 px-4 py-2 text-sm font-semibold text-pink-600 hover:bg-pink-50"
        >
          新規店舗を登録
        </Link>
      </header>

      <section className="space-y-4 rounded-2xl border border-slate-100 bg-white p-4 shadow-sm">
        <div className="grid gap-4 md:grid-cols-3">
          <label className="flex flex-col gap-1 text-xs font-semibold text-slate-700">
            都道府県
            <select
              value={filterForm.prefecture}
              onChange={(event) => setFilterForm((prev) => ({ ...prev, prefecture: event.target.value }))}
              className="rounded-lg border border-slate-200 px-3 py-2 text-base focus:border-pink-500 focus:outline-none"
            >
              <option value="">すべて</option>
              {PREFECTURES.map((pref) => (
                <option key={pref} value={pref}>
                  {pref}
                </option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-1 text-xs font-semibold text-slate-700">
            業種
            <select
              value={filterForm.industry}
              onChange={(event) => setFilterForm((prev) => ({ ...prev, industry: event.target.value }))}
              className="rounded-lg border border-slate-200 px-3 py-2 text-base focus:border-pink-500 focus:outline-none"
            >
              <option value="">すべて</option>
              {REVIEW_CATEGORIES.map((category) => (
                <option key={category.value} value={category.value}>
                  {category.label}
                </option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-1 text-xs font-semibold text-slate-700">
            キーワード
            <input
              value={filterForm.keyword}
              onChange={(event) => setFilterForm((prev) => ({ ...prev, keyword: event.target.value }))}
              className="rounded-lg border border-slate-200 px-3 py-2 text-base focus:border-pink-500 focus:outline-none"
              placeholder="店舗名・支店名で検索"
            />
          </label>
        </div>

        <div className="flex flex-wrap gap-3">
          <button
            type="button"
            onClick={handleSearch}
            className="rounded-full bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow disabled:opacity-60"
            disabled={loading}
          >
            検索
          </button>
          <button
            type="button"
            onClick={handleResetFilters}
            className="rounded-full border border-slate-200 px-4 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-50"
            disabled={loading}
          >
            条件をクリア
          </button>
        </div>
      </section>

      <section className="rounded-2xl border border-slate-100 bg-white shadow-sm">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-100 text-sm">
            <thead className="bg-slate-50 text-left text-xs font-semibold uppercase text-slate-500">
              <tr>
                <th className="px-4 py-3">店舗名</th>
                <th className="px-4 py-3">エリア</th>
                <th className="px-4 py-3">業種</th>
                <th className="px-4 py-3">ジャンル</th>
                <th className="px-4 py-3">アンケート数</th>
                <th className="px-4 py-3">最終更新</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {stores.map((store) => (
                <tr key={store.id}>
                  <td className="px-4 py-3">
                    <div className="font-semibold text-slate-900">
                      {store.name}
                      {store.branchName ? <span className="text-slate-500">（{store.branchName}）</span> : null}
                    </div>
                    <div className="text-xs text-slate-500">{store.prefecture ?? '-'}</div>
                  </td>
                  <td className="px-4 py-3 text-slate-700">{store.area || '-'}</td>
                  <td className="px-4 py-3 text-slate-700">
                    {store.industryCodes[0] ? INDUSTRY_LABEL[store.industryCodes[0]] ?? store.industryCodes[0] : '-'}
                  </td>
                  <td className="px-4 py-3 text-slate-700">{store.genre || '-'}</td>
                  <td className="px-4 py-3 text-slate-700">{store.reviewCount}</td>
                  <td className="px-4 py-3 text-slate-700">{formatDate(store.lastReviewedAt)}</td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex flex-wrap justify-end gap-2">
                      <Link
                        href={`/admin/stores/${store.id}/reviews/new`}
                        className="rounded-full border border-pink-200 px-3 py-1 text-xs font-semibold text-pink-600 hover:bg-pink-50"
                      >
                        アンケート追加
                      </Link>
                      <button
                        type="button"
                        onClick={() => setEditingStore(store)}
                        className="rounded-full border border-slate-200 px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50"
                      >
                        編集
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {loading ? <p className="px-4 py-3 text-sm text-slate-500">読み込み中…</p> : null}
        {!loading && stores.length === 0 ? <p className="px-4 py-3 text-sm text-slate-500">該当する店舗がありません。</p> : null}
        {error ? <p className="px-4 py-3 text-sm text-red-600">{error}</p> : null}
      </section>

      {editingStore ? (
        <section className="space-y-4 rounded-2xl border border-pink-100 bg-white p-4 shadow-sm">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold text-slate-900">店舗情報を編集</h2>
            <button
              type="button"
              onClick={() => setEditingStore(null)}
              className="text-sm font-semibold text-slate-500 hover:text-slate-700"
            >
              閉じる
            </button>
          </div>
          <AdminStoreForm
            initialValues={editingInitialValues}
            onSubmitted={() => {
              setEditingStore(null);
              void fetchStores();
            }}
            title={`${editingStore.name}${editingStore.branchName ? `（${editingStore.branchName}）` : ''}`}
            submitLabel="変更を保存"
            showReset={false}
          />
        </section>
      ) : null}
    </div>
  );
}
