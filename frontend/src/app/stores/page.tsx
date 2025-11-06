import { SearchForm } from '@/components/search/search-form';
import { StoreCard } from '@/components/stores/store-card';
import { Pagination } from '@/components/common/pagination';
import { fetchStores } from '@/lib/stores';

type StoresSearchParams = {
  prefecture?: string;
  category?: string;
  page?: string;
};

export default async function StoresPage({
  searchParams,
}: {
  searchParams: Promise<StoresSearchParams>;
}) {
  const resolved = await searchParams;

  const page = parseNumber(resolved.page) || 1;
  const { items, total, limit } = await fetchStores({
    prefecture: resolved.prefecture,
    category: resolved.category,
    page,
  });

  return (
    <div className="space-y-8 pb-12">
      <header className="space-y-2">
        <h1 className="text-2xl font-semibold text-slate-900">店舗一覧</h1>
        <p className="text-sm text-slate-600">
          都道府県と業種で絞り込んで、自分に合うお店をチェックできます。1ページ10件ずつ表示します。
        </p>
      </header>

      <SearchForm
        initialPrefecture={resolved.prefecture}
        initialCategory={resolved.category}
      />

      <section className="space-y-4">
        {items.length === 0 ? (
          <p className="rounded-2xl bg-slate-100 px-4 py-6 text-sm text-slate-500">
            条件に一致する店舗がまだありません。条件を緩めるか、別のエリアを探してみてください。
          </p>
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            {items.map((store) => (
              <StoreCard key={store.id} store={store} />
            ))}
          </div>
        )}
      </section>

      <Pagination
        currentPage={page}
        totalItems={total}
        pageSize={limit}
        basePath="/stores"
        searchParams={{
          prefecture: resolved.prefecture,
          category: resolved.category,
        }}
      />
    </div>
  );
}

const parseNumber = (value?: string) => {
  if (!value) return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
};
