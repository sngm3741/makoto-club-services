import Link from 'next/link';
import { notFound } from 'next/navigation';

import { ReviewCard } from '@/components/reviews/review-card';
import { REVIEW_CATEGORIES } from '@/constants/filters';
import { fetchReviews } from '@/lib/reviews';
import { fetchStoreDetail } from '@/lib/stores';

type RouteParams = {
  id?: string;
};

type PageProps = {
  params: RouteParams | Promise<RouteParams>;
};

const toCategoryLabel = (value: string) =>
  REVIEW_CATEGORIES.find((item) => item.value === value)?.label ?? value;

const formatOptional = (value?: string | number | null) => {
  if (value === null || value === undefined) return '-';
  if (typeof value === 'string' && value.trim() === '') return '-';
  return value;
};

export default async function StoreDetailPage({ params }: PageProps) {
  const resolvedParams = params instanceof Promise ? await params : params;
  const storeId = resolvedParams?.id;
  if (!storeId) {
    notFound();
  }

  const [store, reviewResponse] = await Promise.all([
    fetchStoreDetail(storeId),
    fetchReviews({ storeId, limit: 20 }),
  ]);

  if (!store) {
    notFound();
  }

  const reviews = reviewResponse.items ?? [];
  const averageRatingLabel = store.averageRating
    ? store.averageRating.toFixed(1)
    : '0.0';
  const averageEarningLabel =
    store.averageEarningLabel ??
    (store.averageEarning > 0 ? `${store.averageEarning}万円` : '-');
  const waitTimeLabel =
    store.waitTimeLabel ??
    (store.waitTimeHours > 0 ? `${store.waitTimeHours}時間` : '-');

  return (
    <div className="space-y-8 pb-12">
      <nav className="text-sm text-slate-500">
        <Link href="/stores" className="hover:text-pink-600">
          店舗一覧
        </Link>
        <span className="mx-2 text-xs text-slate-400">/</span>
        <span className="text-slate-700">
          {store.storeName}
          {store.branchName ? `（${store.branchName}）` : ''}
        </span>
      </nav>

      <header className="space-y-3 rounded-3xl border border-slate-100 bg-white p-6 shadow-sm">
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div>
            <p className="text-xs uppercase tracking-wide text-pink-500">
              {formatOptional(store.prefecture)}
            </p>
            <h1 className="text-3xl font-semibold text-slate-900">
              {store.storeName}
              {store.branchName ? (
                <span className="ml-2 text-lg font-normal text-slate-500">
                  （{store.branchName}）
                </span>
              ) : null}
            </h1>
            <p className="mt-1 text-sm text-slate-600">
              業種: {store.industryCodes}
            </p>
          </div>
        </div>
        <div className="grid gap-3 md:grid-cols-3">
          <RatingStat value={Number(averageRatingLabel)} />
          <StatCard label="平均稼ぎ" value={averageEarningLabel} />
          <StatCard label="平均待機時間" value={waitTimeLabel} />
        </div>
      </header>

      <section className="space-y-4 rounded-3xl border border-slate-100 bg-white p-6 shadow-sm">
        <h2 className="text-xl font-semibold text-slate-900">店舗基本情報</h2>
        <dl className="grid gap-4 text-sm text-slate-700 md:grid-cols-2">
          <InfoRow label="都道府県" value={formatOptional(store.prefecture)} />
          <InfoRow label="エリア" value={formatOptional(store.area)} />
          <InfoRow label="ジャンル" value={formatOptional(store.genre)} />
          <InfoRow label="営業時間" value={formatOptional(store.businessHours)} />
          <InfoRow label="料金目安" value={formatOptional(store.priceRange)} />
          <InfoRow
            label="業種"
            value={
              store.industryCodes && store.industryCodes.length > 0
                ? store.industryCodes.join(', ')
                : '-'
            }
          />
        </dl>
      </section>

      <section className="space-y-4">
        <div className="flex items-center justify-between ml-1">
          <p className="text-sm text-slate-500">
          匿名アンケート: {store.reviewCount}件
          </p>
        </div>
        {reviews.length === 0 ? (
          <p className="rounded-2xl border border-dashed border-slate-200 px-4 py-6 text-sm text-slate-500">
            まだアンケートがありません。最初のアンケートを投稿してみませんか？
          </p>
        ) : (
          <div className="space-y-4">
            {reviews.map((review) => (
              <ReviewCard key={review.id} review={review} />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

const StatCard = ({
  label,
  value,
  highlight,
}: {
  label: string;
  value: string;
  highlight?: boolean;
}) => (
  <div
    className={`rounded-2xl border border-slate-100 px-4 py-3 ${
      highlight ? 'bg-pink-50 text-pink-700' : 'bg-slate-50 text-slate-700'
    }`}
  >
    <p className="text-sm">{label}</p>
    <p className="text-2xl font-semibold">{value}</p>
  </div>
);

const RatingStat = ({ value }: { value: number }) => {
  const formatted = () => {
    if (!Number.isFinite(value)) return '0';
    const num = Number(value.toFixed(2));
    return num % 1 === 0 ? num.toFixed(0) : num.toFixed(2);
  };
  return (
    <div className="rounded-2xl border border-slate-100 bg-pink-50 px-4 py-3 text-pink-700">
      <p className="text-sm">平均評価</p>
      <div className="mt-2 flex flex-col gap-2 text-3xl font-semibold">
        <span>{formatted()}</span>
        <StarDisplay value={value} />
      </div>
    </div>
  );
};

const StarDisplay = ({ value }: { value: number }) => {
  const clamped = Math.max(0, Math.min(5, value || 0));
  return (
    <span className="relative inline-block text-3xl leading-none text-yellow-400">
      <span className="text-slate-200">★★★★★</span>
      <span
        className="absolute left-0 top-0 overflow-hidden"
        style={{ width: `${(clamped / 5) * 100}%` }}
      >
        ★★★★★
      </span>
    </span>
  );
};

const InfoRow = ({ label, value }: { label: string; value: string | number }) => (
  <div className="flex flex-col rounded-2xl bg-slate-50 p-4">
    <dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</dt>
    <dd className="mt-1 text-base text-slate-900">{value}</dd>
  </div>
);
