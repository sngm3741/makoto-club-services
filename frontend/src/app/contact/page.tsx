export default function ContactPage() {
  return (
    <div className="space-y-4 pb-12">
      <h1 className="text-2xl font-semibold text-slate-900">お問い合わせ（削除依頼）</h1>
      <p className="text-sm text-slate-600">
        掲載内容の修正・削除、その他のお問い合わせは以下のフォームまたはLINEでご連絡ください。通常2営業日以内に返信いたします。
      </p>
      <div className="space-y-3 rounded-2xl bg-white p-6 text-sm text-slate-600 shadow">
        <p>
          ・LINE: 公式アカウント「まことクラブ」からメッセージで「削除依頼」とお送りください。担当者が順次対応します。
        </p>
        <p>
          ・メール: <a href="mailto:support@makoto-club.jp" className="text-pink-600">support@makoto-club.jp</a>
        </p>
        <p>
          ・記載いただきたい内容: 対象店舗名、掲載日時、修正/削除理由、LINE登録時のお名前（任意）。
        </p>
      </div>
      <p className="text-xs text-slate-400">最終更新日: 2025-10-23</p>
    </div>
  );
}
