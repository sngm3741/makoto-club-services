# DDD打ち合わせメモ (2025-11-13)

## ゴール
- ユーザー公開向けのアンケート閲覧/投稿ドメインをDDDスタイルへ整理し、オニオンアーキテクチャの中でPublic/Adminコンテキストを分離する。
- Admin機能は同一バイナリ内に残しつつ、責務を切り出し、店舗/アンケートの登録ワークフローと統計更新をサービス層で一元管理する。

## 主要決定事項

### ユビキタス言語
- 「レビュー」は廃止し「アンケート(Survey)」に統一。
- Helpfulボタンは `survey helpful vote` として扱い、1端末(Cookie)につきトグル可能。

### Public体験
- 検索フォーム: 都道府県 / 業種 / キーワード。ジャンルやタグは補助的に扱う。
- ソート: デフォルト新着順。補助ボタンで「総評が高い順」「平均稼ぎが高い順」を提供（店舗・アンケート共通）。
- 店舗一覧/詳細: 同条件でアンケート一覧も取得。店舗詳細では紐づくアンケートを全件掲載する想定。
- アンケート詳細: 紐づく店舗情報を常に付与。
- 匿名投稿: メールアドレス任意。投稿は `inbound_surveys` にストック＋Discord通知。公開モデルに状態は持たず、承認済みのみがユーザー面に出る。
- Helpful: ログイン不要のトグル。Cookie削除やブラウザ変更による重複は許容。helpful順ソートは提供しない。

### Admin体験
- 未承認投稿は閲覧のみ。Discord通知を見ながら、管理UIで店舗を検索→存在しなければ店舗作成→店舗詳細から「アンケート追加」。
- アンケートは必ず既存店舗に紐づく。登録後の `storeID` 変更は禁止。
- 店舗登録時、同名＋同支店（支店名が空ならnull扱い）が存在すれば重複エラーを返す。自動Upsertは禁止。

## データモデル

### stores
- フィールド例: `_id`, `name`, `branchName (nullable)`, `prefecture`, `area`, `industries[]`(ENUM相当だが文字列で保持), `genres[]`, `employmentTypes[]`, `pricePerHour`, `averageEarning`, `businessHours`, `tags[]`, `homepageURL`, `sns{twitter,...}`, `photoURLs[]`, `stats{avgRating, avgEarning, avgWaitTime, reviewCount, lastReviewedAt}`, `createdAt`, `updatedAt`.
- Prefecture/industryはコード化せず文字列で統一。タグ/業種は配列スタイル。
- 画像は最大10枚想定、VPSに保存しNginxで配信。DBにはパス/URLを保持。

### surveys (公開)
- ユーザー入力: 店舗名/支店名/系列、時期、年齢、スペック、待機時間、勤務形態(出稼ぎ or 在籍)、平均稼ぎ(60分単価)、客層、スタッフ、環境、総合評価(0-100)、helpfulCount、documentID(storeID)。
- 写真: 最大10枚、`photos[]` にファイル名やURLを保持。
- `helpfulCount` はHelpful投票の集計値。アンケートには状態を持たせず、公開されるのは管理者が登録したものだけ。

### inbound_surveys (未承認)
- 公開モデルと同じ構造だが、書き手の自由度を優先して `string` ベースで保存。Discord通知と同期する。
- 使い捨てコレクション扱いだが、後から参照できるようメールやIP/UAなども保持可。

### survey_helpful_votes
- フィールド: `_id`, `surveyId`, `voterId`, `createdAt`, `updatedAt`。
- ユニークキー: `surveyId + voterId`。Cookie名 `mc_helpful_voter`、HMAC-SHA256署名、`HttpOnly+Secure+SameSite=Lax+MaxAge=180d`。
- voterIdはCookie再発行で更新。IP/指紋は扱わずCookie頼り。

## インデックス/クエリ方針
- Stores: 優先インデックスは `stats.avgRating` 降順、`stats.avgEarning` 降順。検索フィルタ（都道府県・業種）は全走査で許容し、必要になれば複合インデックス追加。
- Surveys: `rating` 降順、`averageEarning` 降順、`storeId + createdAt desc` を用意。helpful順インデックスは不要。
- Admin検索は `name`, `branchName`, `prefecture`, `industries` で前方一致/部分一致を行う。都道府県/業種どちらか片方指定でも早く動くよう単項インデックスを検討。

## 通知フロー
- 匿名投稿受付 → Mongo `inbound_surveys` へ保存 → Discord Bot (既存 `/base-services/messenger-service/...` が受信) へ送信。
- Discord失敗時: 即リトライ（数回）→Slack通知→`failed_notifications` コレクションへ保存→ERRORログ。Slack/Discord共に失敗した場合もエラーで止めて良い。
- 将来の拡張余地としてメール等の別チャネルを想定するが、現状Slackで十分。

## 管理ドメイン制約
- 店舗重複判定は「店舗名トリム済み＋支店名（空 or null許容）」で一致したら同一店舗として扱う。支店名未入力同士は衝突とみなす。
- 店舗IDは一度アンケートに紐付けたら変更不可。PATCHで変更しようとしたら400を返す。
- Store Upsert禁止。InsertとUpdateを分離し、存在チェックは名前ベースでユーザーに警告を返す。

## Helpful仕様詳細
- Cookieをベースに端末識別。HMAC署名が不正なら破棄して新規発行。
- APIはトグル: `desired=true/false` でInsert/Delete。DB更新後の `helpfulCount` を応答に含める。
- helpfulソートやランキングは不要。店舗・アンケートとも rating/earning のみサポート。

## 未決定/今後の課題
- HTTP層を `internal/interfaces/http/{public,admin}` へ完全移行し、`cmd/api/main.go` をDI専用にする。
- Admin Survey CRUD のサービス責務分割（共通バリデーション、統計再計算のトリガー）。
- Discord/Slack通知を抽象化し、失敗レコードをどこで再送するか（別Workerか手動か）。
- Mongo Schema Migration（写真URL/SNS/タグをどう正規化するか）とデータクレンジング。

## 次アクション
1. `backend/docs/PLANS.md` の更新（今回反映済み）に沿い、Admin/Publicサービスを段階的に実装。
2. Store/Surveyリポジトリを現在の会話仕様へ合わせてリファクタ。
3. HTTPレイヤーを分割し、JWT/CORS/Helpful Cookie初期化を共通化。
4. 通知フローと失敗保存の実装後、curl + go test で一貫動作を確認。

---
Updated: 2025-11-13 — 11/13セッションの合意内容を記録。
