import { notFound } from 'next/navigation';

import { fetchReviewById } from '@/lib/reviews';
import { ReviewDetailContent } from '@/components/reviews/review-detail';
import Link from 'next/link';

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
    <div className="space-y-6 pb-12">
      <Link
        href="/reviews"
        className="inline-flex items-center gap-1 text-sm font-semibold text-pink-600 hover:text-pink-500"
      >
        ← アンケート一覧に戻る
      </Link>
      <ReviewDetailContent review={review} />
    </div>
  );
}
