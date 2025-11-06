import Link from 'next/link';

import { Pagination } from '@/components/common/pagination';
import { SearchForm } from '@/components/search/search-form';
import { ReviewCard } from '@/components/reviews/review-card';
import { fetchReviews } from '@/lib/reviews';

type ReviewsSearchParams = {
  prefecture?: string;
  category?: string;
  store?: string;
  page?: string;
  sort?: string;
};

const SORT_OPTIONS = [
  { value: undefined, label: '新着順' },
  { value: 'helpful', label: '役に立った順' },
  { value: 'earning', label: '平均稼ぎが高い順' },
] as const;

export default async function ReviewsPage({
  searchParams,
}: {
  searchParams: Promise<ReviewsSearchParams>;
}) {
  const resolved = await searchParams;

  const page = parseNumber(resolved.page) || 1;
  const { items, total, limit } = await fetchReviews({
    prefecture: resolved.prefecture,
    category: resolved.category,
    storeName: resolved.store,
    sort: resolved.sort,
    page,
  });

  return (
    <div className="space-y-8 pb-12">
      <header className="space-y-2">
        <h1 className="text-2xl font-semibold text-slate-900">アンケート一覧</h1>
        <p className="text-sm text-slate-600">
          実際に働いた女の子たちの体験談を、都道府県や業種で絞り込んで探せます。気になる店舗の詳細は各レビューページへ！
        </p>
      </header>

      <SearchForm
        redirectPath="/reviews"
        initialPrefecture={resolved.prefecture}
        initialCategory={resolved.category}
      />

      <div className="flex flex-wrap items-center gap-2 text-xs text-slate-600">
        <span className="font-semibold text-slate-700">並び替え:</span>
        {SORT_OPTIONS.map((option) => (
          <SortLink key={option.label} label={option.label} value={option.value} searchParams={resolved} />
        ))}
      </div>

      <section className="space-y-4">
        {items.length === 0 ? (
          <p className="rounded-2xl bg-slate-100 px-4 py-6 text-sm text-slate-500">
            条件に一致する口コミがまだありません。別の条件やエリアで探してみてください。
          </p>
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            {items.map((review) => (
              <ReviewCard key={review.id} review={review} />
            ))}
          </div>
        )}
      </section>

      <Pagination
        currentPage={page}
        totalItems={total}
        pageSize={limit}
        basePath="/reviews"
        searchParams={{
          prefecture: resolved.prefecture,
          category: resolved.category,
          store: resolved.store,
          sort: resolved.sort,
        }}
      />
    </div>
  );
}

const SortLink = ({
  label,
  value,
  searchParams,
}: {
  label: string;
  value: string | undefined;
  searchParams: ReviewsSearchParams;
}) => {
  const params = new URLSearchParams();
  if (searchParams.prefecture) params.set('prefecture', searchParams.prefecture);
  if (searchParams.category) params.set('category', searchParams.category);
  if (searchParams.store) params.set('store', searchParams.store);
  if (value) {
    params.set('sort', value);
  }
  const href = params.toString() ? `/reviews?${params.toString()}` : '/reviews';
  const isActive =
    (!value && !searchParams.sort) || (!!value && searchParams.sort === value);

  return (
    <Link
      href={href}
      className={`rounded-full px-3 py-1 font-semibold ${
        isActive
          ? 'bg-pink-500 text-white'
          : 'bg-slate-100 text-slate-600 hover:bg-pink-100 hover:text-pink-600'
      }`}
    >
      {label}
    </Link>
  );
};

const parseNumber = (value?: string) => {
  if (!value) return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
};
