'use client';

import { FormEvent, useState } from 'react';
import { useRouter } from 'next/navigation';

import { PREFECTURES, REVIEW_CATEGORIES } from '@/constants/filters';

type SearchFormProps = {
  initialPrefecture?: string;
  initialCategory?: string;
  redirectPath?: string;
};

export const SearchForm = ({
  initialPrefecture = '',
  initialCategory = '',
  redirectPath = '/stores',
}: SearchFormProps) => {
  const router = useRouter();
  const [prefecture, setPrefecture] = useState(initialPrefecture);
  const [category, setCategory] = useState(initialCategory);

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const params = new URLSearchParams();
    if (prefecture) params.set('prefecture', prefecture);
    if (category) params.set('category', category);
    const queryString = params.toString();
    router.push(queryString ? `${redirectPath}?${queryString}` : redirectPath);
  };

  return (
    <form
      onSubmit={handleSubmit}
      className="flex flex-col gap-3 rounded-3xl border border-slate-100 bg-white p-6 shadow-lg"
    >
      <div className="space-y-1">
        <label className="text-sm font-semibold text-slate-700" htmlFor="prefecture">
          都道府県
        </label>
        <select
          id="prefecture"
          value={prefecture}
          onChange={(event) => setPrefecture(event.target.value)}
          className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm focus:border-pink-400 focus:outline-none focus:ring-2 focus:ring-pink-200"
        >
          <option value="">指定なし</option>
          {PREFECTURES.map((pref) => (
            <option key={pref} value={pref}>
              {pref}
            </option>
          ))}
        </select>
      </div>

      <div className="space-y-1">
        <label className="text-sm font-semibold text-slate-700" htmlFor="category">
          業種
        </label>
        <select
          id="category"
          value={category}
          onChange={(event) => setCategory(event.target.value)}
          className="w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm focus:border-pink-400 focus:outline-none focus:ring-2 focus:ring-pink-200"
        >
          <option value="">指定なし</option>
          {REVIEW_CATEGORIES.map((item) => (
            <option key={item.value} value={item.value}>
              {item.label}
            </option>
          ))}
        </select>
      </div>

      <button
        type="submit"
        className="mt-2 inline-flex items-center justify-center rounded-full bg-gradient-to-r from-pink-500 to-violet-500 px-4 py-2 text-sm font-semibold text-white shadow-md transition hover:from-pink-400 hover:to-violet-400"
      >
        条件で検索する
      </button>
    </form>
  );
};
