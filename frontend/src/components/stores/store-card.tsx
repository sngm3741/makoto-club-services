import Link from 'next/link';

import type { StoreSummary } from '@/types/review';

type StoreCardProps = {
  store: StoreSummary;
};

export const StoreCard = ({ store }: StoreCardProps) => {
  const hasReviews = store.reviewCount > 0;
  const averageDisplay = hasReviews
    ? store.averageEarningLabel && store.averageEarningLabel !== '-'
      ? store.averageEarningLabel
      : `${store.averageEarning}万円`
    : '-';
  const waitDisplay = hasReviews
    ? store.waitTimeLabel && store.waitTimeLabel !== '-'
      ? store.waitTimeLabel
      : `${store.waitTimeHours}時間`
    : '-';
  const ratingDisplay = store.averageRating.toFixed(1);

  return (
    <article className="flex flex-col gap-4 rounded-2xl border border-slate-100 bg-white p-5 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md">
      <div className="flex items-center justify-between text-xs text-slate-500">
        <span className="rounded-full bg-pink-50 px-3 py-1 text-pink-600">
          {store.prefecture}
        </span>
        <span>{translateCategory(store.category)}</span>
      </div>
      <div>
        <h3 className="text-lg font-semibold text-slate-900">{store.storeName}</h3>
        <p className="mt-1 text-sm text-slate-500">
          アンケート件数: <strong className="font-semibold text-slate-700">{store.reviewCount}</strong>
        </p>
        <div className="mt-2 flex items-center gap-2 text-xs text-slate-500">
          <StarDisplay value={store.averageRating} />
          <span>{ratingDisplay} / 5</span>
        </div>
      </div>
      <dl className="grid grid-cols-2 gap-3 text-xs text-slate-500">
        <div className="rounded-xl bg-slate-50 p-3">
          <dt className="font-medium text-slate-700">平均稼ぎ</dt>
          <dd className="mt-1 text-lg font-semibold text-pink-600">
            {averageDisplay}
          </dd>
        </div>
        <div className="rounded-xl bg-slate-50 p-3">
          <dt className="font-medium text-slate-700">平均待機時間</dt>
          <dd className="mt-1 text-lg font-semibold text-slate-800">
            {waitDisplay}
          </dd>
        </div>
      </dl>
      <Link
        href={`/stores/${store.id}`}
        className="inline-flex w-fit items-center gap-1 text-sm font-semibold text-pink-600 hover:text-pink-500"
      >
        この店舗の情報を見る
        <span aria-hidden>→</span>
      </Link>
    </article>
  );
};

const CATEGORY_LABEL_MAP: Record<string, string> = {
  deriheru: 'デリヘル',
  hoteheru: 'ホテヘル',
  hakoheru: '箱ヘル',
  sopu: 'ソープ',
  dc: 'DC',
  huesu: '風エス',
  menesu: 'メンエス',
};

export const translateCategory = (category: string) =>
  CATEGORY_LABEL_MAP[category] ?? category;

const StarDisplay = ({ value }: { value: number }) => {
  const clamped = Math.max(0, Math.min(5, value));
  return (
    <span className="relative inline-block text-base leading-none">
      <span className="text-slate-300">★★★★★</span>
      <span
        className="absolute left-0 top-0 overflow-hidden text-yellow-400"
        style={{ width: `${(clamped / 5) * 100}%` }}
      >
        ★★★★★
      </span>
    </span>
  );
};
