export default function TermsPage() {
  return (
    <div className="space-y-4 pb-12">
      <h1 className="text-2xl font-semibold text-slate-900">利用規約</h1>
      <p className="text-sm text-slate-600">
        本サービス「まことクラブ」をご利用いただく前に、以下の利用規約をご確認ください。正式な文章は法務確認後に差し替え予定です。
      </p>
      <div className="space-y-3 rounded-2xl bg-white p-6 text-sm leading-relaxed text-slate-600 shadow">
        <p>
          1. 利用者は、本サービスに登録・投稿する情報について、真実かつ正確な内容を提供するものとします。
        </p>
        <p>
          2. 投稿内容に第三者の個人情報や誹謗中傷が含まれる場合、運営は削除する権利を有します。
        </p>
        <p>
          3. 投稿された内容については、サービスのプロモーション等で匿名のまま引用させていただく場合があります。
        </p>
        <p>
          4. 本規約は予告なく変更することがあります。変更後もサービスを利用された場合、改訂された内容に同意したものとみなします。
        </p>
      </div>
      <p className="text-xs text-slate-400">最終更新日: 2025-10-23</p>
    </div>
  );
}
