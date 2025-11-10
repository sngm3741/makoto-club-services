import Link from 'next/link';
import { notFound } from 'next/navigation';

import { AdminStoreReviewForm, type AdminStoreSummary } from '@/components/admin/admin-store-review-form';

const API_BASE_URL = process.env.API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? '';

async function fetchStore(id: string): Promise<AdminStoreSummary> {
  if (!API_BASE_URL) {
    throw new Error('API_BASE_URL が未設定です');
  }
  const response = await fetch(`${API_BASE_URL}/api/admin/stores/${id}`, {
    cache: 'no-store',
  });
  if (response.status === 404) {
    notFound();
  }
  if (!response.ok) {
    throw new Error('店舗情報の取得に失敗しました');
  }
  return (await response.json()) as AdminStoreSummary;
}

export const dynamic = 'force-dynamic';

type RouteParams = {
  id?: string;
};

type PageProps = {
  params: RouteParams | Promise<RouteParams>;
};

export default async function AdminStoreReviewCreatePage({ params }: PageProps) {
  const resolvedParams = params instanceof Promise ? await params : params;
  const id = resolvedParams?.id;
  if (!id) {
    notFound();
  }

  const store = await fetchStore(id);

  return (
    <div className="mx-auto w-full max-w-4xl space-y-6 px-4 py-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-900">アンケートを登録</h1>
        <Link href="/admin/stores" className="text-sm font-semibold text-pink-600 hover:text-pink-500">
          ⟵ 店舗一覧に戻る
        </Link>
      </div>
      <AdminStoreReviewForm store={store} />
    </div>
  );
}
