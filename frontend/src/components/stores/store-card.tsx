import Link from 'next/link';

import type { StoreSummary } from '@/types/review';

type StoreCardProps = {
  store: StoreSummary;
};

export const StoreCard = ({ store }: StoreCardProps) => {
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
          レビュー件数: <strong className="font-semibold text-slate-700">{store.reviewCount}</strong>
        </p>
      </div>
      <dl className="grid grid-cols-2 gap-3 text-xs text-slate-500">
        <div className="rounded-xl bg-slate-50 p-3">
          <dt className="font-medium text-slate-700">平均稼ぎ</dt>
          <dd className="mt-1 text-lg font-semibold text-pink-600">
            {store.averageEarningLabel ?? `${store.averageEarning}万円`}
          </dd>
        </div>
        <div className="rounded-xl bg-slate-50 p-3">
          <dt className="font-medium text-slate-700">平均待機時間</dt>
          <dd className="mt-1 text-lg font-semibold text-slate-800">
            {store.waitTimeLabel ?? `${store.waitTimeHours}時間`}
          </dd>
        </div>
      </dl>
      <Link
        href={`/reviews?store=${encodeURIComponent(store.storeName)}`}
        className="inline-flex w-fit items-center gap-1 text-sm font-semibold text-pink-600 hover:text-pink-500"
      >
        この店舗のレビューを見る
        <span aria-hidden>→</span>
      </Link>
    </article>
  );
};

const CATEGORY_LABEL_MAP: Record<string, string> = {
  delivery_health: 'デリヘル',
  hotel_health: 'ホテヘル',
  box_health: '箱ヘル',
  soap: 'ソープ',
  dc: 'DC',
  fu_es: '風エス',
  men_es: 'メンエス',
};

export const translateCategory = (category: string) =>
  CATEGORY_LABEL_MAP[category] ?? category;
