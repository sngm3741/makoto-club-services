import Link from 'next/link';
import { notFound } from 'next/navigation';

import { fetchReviewById } from '@/lib/reviews';
import { translateCategory } from '@/components/stores/store-card';

type ReviewDetailPageProps = {
  params: Promise<{ id: string }>;
};

export default async function ReviewDetailPage({ params }: ReviewDetailPageProps) {
  const { id } = await params;

  const review = await fetchReviewById(id);
  if (!review) {
    notFound();
  }

  return (
    <article className="space-y-8 pb-12">
      <header className="space-y-2">
        <Link
          href="/reviews"
          className="inline-flex items-center gap-1 text-sm font-semibold text-pink-600 hover:text-pink-500"
        >
          ← アンケート一覧に戻る
        </Link>
        <div className="space-y-1">
          <p className="text-xs text-slate-500">{review.prefecture}</p>
          <h1 className="text-2xl font-semibold text-slate-900">{review.storeName}</h1>
          <p className="text-sm text-slate-600">{translateCategory(review.category)}</p>
        </div>
      </header>

      <section className="grid gap-4 rounded-3xl border border-slate-100 bg-white p-6 shadow-sm md:grid-cols-2">
        <InfoGroup
          items={[
            { label: '訪問時期', value: review.visitedAt },
            { label: '平均稼ぎ', value: `${review.averageEarning}万円` },
            { label: '平均待機時間', value: `${review.waitTimeHours}時間` },
          ]}
        />
        <InfoGroup
          items={[
            { label: '年齢', value: `${review.age}歳` },
            { label: 'スペック', value: `${review.specScore}` },
            { label: '役に立った件数', value: `${review.helpfulCount ?? 0}` },
          ]}
        />
      </section>

      <section className="rounded-3xl border border-slate-100 bg-white p-6 shadow-sm">
        <h2 className="text-lg font-semibold text-slate-900">投稿者のコメント</h2>
        <p className="mt-4 text-base leading-relaxed whitespace-pre-line text-slate-700">
          {review.description && review.description.trim().length > 0
            ? review.description
            : '詳細コメントは準備中です。'}
        </p>
      </section>
    </article>
  );
}

const InfoGroup = ({ items }: { items: { label: string; value: string }[] }) => (
  <div className="space-y-4">
    {items.map((item) => (
      <div key={item.label}>
        <p className="text-xs font-semibold tracking-wide text-slate-400 uppercase">{item.label}</p>
        <p className="mt-1 text-base font-semibold text-slate-900">{item.value}</p>
      </div>
    ))}
  </div>
);
