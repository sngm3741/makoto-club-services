import Link from 'next/link';

export default function NotFound() {
  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 text-center">
      <h1 className="text-3xl font-semibold text-slate-900">ページが見つかりませんでした</h1>
      <p className="max-w-md text-sm text-slate-600">
        ご指定のページは存在しないか、移動した可能性があります。トップページに戻って検索をやり直してください。
      </p>
      <Link
        href="/"
        className="rounded-full bg-gradient-to-r from-pink-500 to-violet-500 px-6 py-2 text-sm font-semibold text-white shadow hover:from-pink-400 hover:to-violet-400"
      >
        トップへ戻る
      </Link>
    </div>
  );
}
