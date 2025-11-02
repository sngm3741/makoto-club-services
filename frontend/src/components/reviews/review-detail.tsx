import type { ReviewDetail } from '@/types/review';

type ReviewDetailProps = {
  review: ReviewDetail;
};

export const ReviewDetailContent = ({ review }: ReviewDetailProps) => {
  return (
    <article className="space-y-6 rounded-3xl border border-slate-100 bg-white p-6 shadow-sm">
      <header className="space-y-2">
        <span className="inline-block rounded-full bg-pink-50 px-3 py-1 text-xs font-semibold text-pink-600">
          {review.prefecture} / {translateCategory(review.category)}
        </span>
        <h1 className="text-2xl font-semibold text-slate-900">
          {review.storeName}
          {review.branchName ? (
            <span className="ml-2 text-base font-normal text-slate-500">（{review.branchName}）</span>
          ) : null}
        </h1>
        <p className="text-sm text-slate-500">
          訪問時期: {review.visitedAt} / 年齢: {review.age}歳 / スペック: {review.specScore}
        </p>
        <div className="flex items-center gap-2 text-xs text-slate-500">
          <StarDisplay value={review.rating} />
          <span>{review.rating.toFixed(1)} / 5</span>
        </div>
      </header>

      <dl className="grid gap-4 rounded-2xl bg-slate-50 p-4 text-sm sm:grid-cols-3">
        <div>
          <dt className="text-slate-500">平均稼ぎ</dt>
          <dd className="text-lg font-semibold text-pink-600">{review.averageEarning}万円</dd>
        </div>
        <div>
          <dt className="text-slate-500">待機時間</dt>
          <dd className="text-lg font-semibold text-slate-800">{review.waitTimeHours}時間</dd>
        </div>
        {typeof review.helpfulCount === 'number' ? (
          <div>
            <dt className="text-slate-500">役に立った</dt>
            <dd className="text-lg font-semibold text-violet-600">
              {review.helpfulCount}人が役立ったと回答
            </dd>
          </div>
        ) : null}
      </dl>

      <section className="space-y-2">
        <h2 className="text-lg font-semibold text-slate-800">レビュー本文</h2>
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-slate-700">
          {review.description ??
            '詳細な口コミは準備中です。バックエンドAPIが整備され次第、本文を掲載します。'}
        </p>
      </section>

      <footer className="flex items-center justify-between text-xs text-slate-500">
        <p>投稿日: {review.createdAt}</p>
        {review.authorDisplayName ? <p>投稿者: {review.authorDisplayName}</p> : null}
      </footer>
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

const translateCategory = (category: string) =>
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
