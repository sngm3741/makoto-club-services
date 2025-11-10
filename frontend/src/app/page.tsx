import Link from 'next/link';

import { ReviewCard } from '@/components/reviews/review-card';
import { SearchForm } from '@/components/search/search-form';
import { fetchFeaturedReviews } from '@/lib/reviews';

export default async function HomePage() {
  const { latest, highRated } = await fetchFeaturedReviews();

  return (
    <div className="space-y-12 pb-12">
      <section className="grid gap-8 lg:grid-cols-[1.1fr_0.9fr]">
        <div className="space-y-6 rounded-3xl bg-gradient-to-br from-pink-100 via-white to-white p-8 shadow-lg">
          <p className="inline-flex rounded-full bg-white/70 px-3 py-1 text-xs font-semibold text-pink-600">
            風俗で働く女の子のためのアンケートメディア
          </p>
          <h1 className="text-3xl leading-snug font-semibold text-slate-900">
            リアルなアンケートで、
            <br className="hidden sm:block" />
            自分にぴったりの店舗を探そう
          </h1>
          <p className="text-sm leading-relaxed text-slate-600">
            500件以上のアンケートを集約。待機時間や平均稼ぎなど、気になるポイント別に検索できます。
            X（旧Twitter）でログインすると投稿もできて、PayPay1,000円の特典をDMでご案内します。
          </p>
          <div className="flex flex-wrap gap-3 text-xs text-slate-500">
            <span className="rounded-full bg-white/90 px-3 py-1 font-medium">
              500件以上のアンケート
            </span>
            <span className="rounded-full bg-white/90 px-3 py-1 font-medium">
              全国47都道府県対応
            </span>
            <span className="rounded-full bg-white/90 px-3 py-1 font-medium">X連携で本人確認</span>
          </div>
        </div>
        <div className="space-y-4">
          <h2 className="text-lg font-semibold text-slate-800">条件から探す</h2>
          <SearchForm />
          <p className="text-xs text-slate-500">
            絞り込み後は「店舗一覧」ページに移動します。都道府県や業種の組み合わせで、狙い目のエリアをチェックしましょう。
          </p>
        </div>
      </section>

      <section className="space-y-6">
        <div className="flex items-baseline justify-between">
          <h2 className="text-xl font-semibold text-slate-900">新着アンケート</h2>
          <Link href="/reviews" className="text-sm font-semibold text-pink-600 hover:text-pink-500">
            もっと見る →
          </Link>
        </div>
        <div className="grid gap-4 md:grid-cols-3">
          {latest.map((review) => (
            <ReviewCard key={review.id} review={review} />
          ))}
        </div>
      </section>

      <section className="space-y-6">
        <div className="flex items-baseline justify-between">
          <h2 className="text-xl font-semibold text-slate-900">高評価のアンケート</h2>
          <Link
            href="/reviews?sort=helpful"
            className="text-sm font-semibold text-pink-600 hover:text-pink-500"
          >
            もっと見る →
          </Link>
        </div>
        <div className="grid gap-4 md:grid-cols-3">
          {highRated.map((review) => (
            <ReviewCard key={review.id} review={review} />
          ))}
        </div>
      </section>

      <section className="grid gap-4 md:grid-cols-2">
        <div className="rounded-3xl bg-gradient-to-br from-slate-900 to-slate-700 p-6 text-white shadow-lg">
          <p className="text-sm font-semibold tracking-[0.2em] text-white/70 uppercase">
            オフィシャルTwitter
          </p>
          <h3 className="mt-4 text-2xl font-semibold">最新イベント情報を毎日発信中</h3>
          <p className="mt-2 text-sm text-white/80">
            店舗特集やキャッシュバック、出稼ぎのコツなどをポップに紹介。フォローして最新情報をチェック！
          </p>
          <a
            href="https://twitter.com"
            target="_blank"
            rel="noreferrer"
            className="mt-4 inline-flex w-fit items-center gap-2 rounded-full bg-white px-4 py-2 text-sm font-semibold text-slate-900"
          >
            Twitterで見る
            <span aria-hidden>↗</span>
          </a>
        </div>
        <div className="rounded-3xl border border-pink-200 bg-white p-6 shadow-lg">
          <p className="inline-flex rounded-full bg-pink-100 px-3 py-1 text-xs font-semibold text-pink-600">
            投稿でPayPayをプレゼント
          </p>
          <h3 className="mt-4 text-2xl font-semibold text-slate-900">
            Xログインで投稿すると
            <br />
            PayPay 1,000円プレゼント
          </h3>
          <p className="mt-2 text-sm text-slate-600">
            「投稿してPayPayを受け取る」からXログインするとアンケートを投稿できます。審査完了後に公式アカウントからDMで受け取りリンクをお送りします。
          </p>
          <Link
            href="/reviews/new"
            className="mt-4 inline-flex w-fit items-center gap-2 rounded-full bg-gradient-to-r from-pink-500 to-violet-500 px-4 py-2 text-sm font-semibold text-white shadow-md hover:from-pink-400 hover:to-violet-400"
          >
            アンケート投稿ページへ
          </Link>
        </div>
      </section>
    </div>
  );
}
