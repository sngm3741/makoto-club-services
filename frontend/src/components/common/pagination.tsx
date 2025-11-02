import Link from 'next/link';

type PaginationProps = {
  currentPage: number;
  totalItems: number;
  pageSize: number;
  basePath: string;
  searchParams?: Record<string, string | number | undefined>;
};

export const Pagination = ({
  currentPage,
  totalItems,
  pageSize,
  basePath,
  searchParams,
}: PaginationProps) => {
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
  if (totalPages <= 1) return null;

  const createHref = (page: number) => {
    const params = new URLSearchParams();
    if (searchParams) {
      Object.entries(searchParams).forEach(([key, value]) => {
        if (value !== undefined && value !== '') {
          params.set(key, String(value));
        }
      });
    }
    params.set('page', String(page));
    return `${basePath}?${params.toString()}`;
  };

  const pages = generatePages(currentPage, totalPages);

  return (
    <nav className="flex items-center justify-center gap-2 text-sm text-slate-600">
      <Link
        aria-disabled={currentPage === 1}
        href={createHref(Math.max(1, currentPage - 1))}
        className={`rounded-full border border-slate-200 px-3 py-1 ${
          currentPage === 1
            ? 'pointer-events-none opacity-50'
            : 'hover:border-pink-300 hover:text-pink-600'
        }`}
      >
        前へ
      </Link>
      {pages.map((page) => (
        <Link
          key={page}
          href={createHref(page)}
          className={`rounded-full px-3 py-1 ${
            page === currentPage
              ? 'bg-pink-500 text-white'
              : 'hover:bg-slate-100 hover:text-pink-600'
          }`}
        >
          {page}
        </Link>
      ))}
      <Link
        aria-disabled={currentPage === totalPages}
        href={createHref(Math.min(totalPages, currentPage + 1))}
        className={`rounded-full border border-slate-200 px-3 py-1 ${
          currentPage === totalPages
            ? 'pointer-events-none opacity-50'
            : 'hover:border-pink-300 hover:text-pink-600'
        }`}
      >
        次へ
      </Link>
    </nav>
  );
};

const generatePages = (currentPage: number, totalPages: number) => {
  const pages = new Set<number>([currentPage]);
  const addPage = (page: number) => {
    if (page >= 1 && page <= totalPages) {
      pages.add(page);
    }
  };

  addPage(currentPage - 1);
  addPage(currentPage + 1);
  addPage(1);
  addPage(totalPages);

  return Array.from(pages).sort((a, b) => a - b);
};
