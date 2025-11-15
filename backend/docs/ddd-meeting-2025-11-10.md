# DDD打ち合わせメモ (2025-11-10)

## ゴールとスコープ
- `backend/api/main.go` 直下に詰め込まれた機能をDDD/オニオンアーキテクチャで再設計する方針と前提条件を言語化する。
- 主戦場は一般ユーザー向けのアンケート閲覧・投稿体験。管理者機能は同一バイナリ内の別コンテキストとして扱う。

## ユビキタス言語
- レビューという語は廃止し、公開・管理双方で「アンケート(Survey)」に統一する。
- 「役に立った」ボタンは survey helpful vote と呼称し、匿名端末単位でトグル可能な評価として扱う。

## バウンデッドコンテキスト
1. **Public Survey Explorer**
   - 一般ユーザーが店舗／アンケートを検索・閲覧・投稿する領域。
   - 役に立ったトグルを含むクエリ/コマンド分離を行う。
2. **Admin Survey Operations**
   - 管理者が投稿を審査し、店舗情報・公開アンケートを登録／更新する領域。
   - 同一Goバイナリ内に配置するが、モジュールと依存方向は完全に分離。HTTPはJWT認証で制御。

## コア体験 (Public)
- 絞り込み：都道府県・業種・キーワード。評価値/平均稼ぎ額はソート指定で対応。
- 同条件でアンケート一覧の取得が可能。
- 店舗詳細は紐づくアンケート一覧を含める。アンケート詳細は対応店舗情報を必ず付与。
- 匿名アンケート投稿が可能（メール任意）。投稿内容はDiscord通知され、公開側では未承認状態を持たない。
- 役に立ったボタンはCookieベースで端末識別し、サーバー側に `survey_id × voter_id` 記録を永続化。トグル可、取消も可。

## コア体験 (Admin)
- Discord通知された投稿を参照しつつ、WEB管理画面からアンケート登録・店舗登録/更新を行う。
- アンケートは必ず既存店舗に紐付け。存在しない場合は先に店舗作成してから登録。
- 未承認投稿は使い捨てコレクション（またはDiscordログ）で保持し、承認後に公開コレクションへ書き込む。

## アーキテクチャ方針
- MongoDBのみで当面運用。件数は店舗/アンケートとも数千規模想定、複合インデックスで検索性能を確保する。
- 読み取り特化ストアや投影は導入しない（スケールが必要になった時点で再検討）。
- エントリーポイントを `cmd/api/main.go` 相当に分離し、
  - コンフィグ読み込み
  - Mongo/HTTP/Discordなどインフラ初期化
  - Repository実装の生成
  - アプリケーションサービス（Query/Command）の組み立て
  - ルーター起動
  を担当させる。
- アプリケーション層は `StoreQueryService`, `SurveyQueryService`, `SurveyCommandService` を最低限用意し、UI層(HTTPハンドラ)はこれらを経由。
- ドメイン層では店舗とアンケートを別集約とし、統計更新は管理側の登録時に行う（公開投稿時には行わない）。

## インテグレーション
- アンケート投稿時にDiscord通知を必ず送信。通知済みの投稿は管理者がDiscord上から参照して審査する。
- 報酬処理やメール対応はシステム外（管理者運用）と定義し、公開ドメインでは考慮しない。

## 未決定/今後詰める事項
- Mongoコレクション設計（公開用/未承認用/投票履歴）と具体的なスキーマ命名。
- HTTP APIのバージョニング方針と各エンドポイント再配置。
- 役に立ったトークンの署名方式とCookie属性(SameSite, Secure, TTL)。
- Discord通知失敗時のリトライ／バックプレッシャー処理。


## コレクション/インデックス決定 (Public)
### stores
- フィールド: `_id`, `name`, `branchName`, `prefecture`, `area`, `genre`, `employmentType`, `pricePerHour`, `businessHours`, `tags[]`, `keywords[]`, `homepageURL`, `sns{twitter?…}`, `stats{avgRating, avgEarning, avgWaitTime, reviewCount, lastReviewedAt}`, `createdAt`, `updatedAt`。
- 画像は持たない（店舗サムネが必要になったら別途検討）。
- インデックスは「全国ランキングを最優先」とし、最低限 `stats.avgRating` 降順と `stats.avgEarning` 降順の単独を貼る。都道府県/業種フィルタは全走査で妥協し、需要に応じて複合インデックスを追加する方針。

### surveys
- フィールド: `_id`, `storeId`, `storeName`, `branchName`, `period`, `age`, `specScore`, `waitTime`, `employmentType`, `averageEarning`, `customerNote`, `staffNote`, `environmentNote`, `rating(0-100)`, `helpfulCount`, `photos[]`, `createdAt`, `updatedAt`。
- `photos` は最大10枚。要素は `id`, `storedPath`, `publicURL`, `contentType`, `uploadedAt` を保持。実ファイルはVPS内ディレクトリ＋Nginx配信、`mediaBaseURL` 経由で公開。
- インデックス: `{ rating: -1 }`, `{ averageEarning: -1 }`, `{ storeId: 1, createdAt: -1 }`。

### inbound_surveys (未承認)
- フィールド: `_id`, `rawPayload`(string), `submittedAt`, `discordMessageID`, `clientIP`。
- エンティティは管理側のみ参照。公開面では扱わない。

### survey_helpful_votes
- フィールド: `_id`, `surveyId`, `voterId`, `createdAt`。
- インデックス: `{ surveyId: 1, voterId: 1 }` にユニーク制約。

## Helpful Vote 仕様
- Cookie に署名付き `voterId`(UUIDv4) を1件だけ保持。構造は `v=<uuid>&ts=<unix>&sig=<HMAC>`。
- Cookie属性: `HttpOnly`, `Secure`, `SameSite=Lax`, `MaxAge=180d`。端末削除でリセット可。
- APIはトグル式：ON時に `survey_helpful_votes` へInsertし `surveys.helpfulCount++`、OFF時にDelete＋`--`。競合はユニーク制約エラーで制御。
- 署名キーはサーバー設定で管理し、漏洩時はローテーション＋Cookie失効で対応。

## Public API (v1) ルーティング案
### GET /api/v1/stores
- Query: `prefecture`, `genre`, `keyword`, `tags[]`, `sort(new|rating|earning, default=new)`, `page`, `limit`。
- 都道府県コードは文字列受け取り→内部で正規化。tags は `?tags=foo&tags=bar` の配列形式。

### GET /api/v1/stores/{id}
- 店舗詳細 + 直近アンケート要約（最大N件）。

### GET /api/v1/surveys
- Query: `storeId`（任意）, `prefecture`, `genre`, `keyword`, `tags[]`, `sort(new|rating|earning, default=new)`, `page`, `limit`。
- 店舗個別ページは `storeId` 指定で利用。helpfulソートは導入しない。

### GET /api/v1/surveys/{id}
- アンケート詳細 + 紐づく店舗概要。

### POST /api/v1/surveys
- 匿名投稿。メールは任意。
- 受信後はDiscord通知＋`inbound_surveys` 保存のみ。公開用 `surveys` には書かない。

### POST /api/v1/surveys/{id}/helpful
- Cookie署名済み `voterId` を使ったトグルAPI。ON/OFFをボディで指定。

## 管理者向けドメイン制約
- 管理者が公開用アンケートを登録する際、必ず既存店舗IDに紐付ける。紐づけ先が存在しない場合は先に店舗を作成し、そのIDを使ってアンケート登録を実施する。
- 管理UIフロー: 未承認投稿閲覧 → 必要なら店舗検索/新規作成 → アンケート登録フォームで店舗IDを必須入力。

## Admin API (v1) ルーティング案
### Stores
- `GET /api/admin/v1/stores` : `prefecture`, `genre`, `keyword`, `page`, `limit`, `sort(new|name)` など管理用フィルタ。
- `GET /api/admin/v1/stores/{id}` : 詳細＋編集用データ。
- `POST /api/admin/v1/stores` : 店舗新規作成。
- `PATCH /api/admin/v1/stores/{id}` : 部分更新。

### Surveys
- `GET /api/admin/v1/surveys` : `storeId`, `prefecture`, `genre`, `keyword`, `page`, `limit`。
- `GET /api/admin/v1/surveys/{id}` : 詳細。
- `POST /api/admin/v1/surveys` : 店舗ID必須で公開アンケート登録。
- `PATCH /api/admin/v1/surveys/{id}` : 更新（例: コメント修正）。

### Inbound Surveys
- `GET /api/admin/v1/inbound-surveys` : 未承認投稿を参照するための読み取り専用API。

- すべてJWT認証必須。Publicとは別ルーター/ミドルウェアで guard する。

## Helpful Vote Cookie 実装方針
- Cookie名: `mc_helpful_voter`（仮）。値は `v=<uuid>&ts=<unix>&sig=<base64url>`。
- 署名: `sig = Base64URL( HMAC-SHA256(secret, "v=<uuid>&ts=<unix>") )`。`secret` は環境変数から供給し、定期ローテーション可能にする。
- TTL: 180日。`Set-Cookie` 属性 `HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=15552000`。
- 検証: Cookie未設定 or 期限切れ → 新規発行。署名不一致 → Cookie破棄→再発行。`ts` がTTL超過したら再発行。
- APIフロー: トグル要求を受けたら Cookie を検証→`voterId` を取得→`survey_helpful_votes` に対して Insert/Delete + `surveys.helpfulCount` のインクリメント/デクリメント。`surveyId+voterId` のユニーク制約違反で多重投票を弾く。

## Discord通知失敗時の扱い
- Discord Webhook送信は即時リトライ3回（1s, 3s, 10s）。
- すべて失敗した場合、`failed_notifications` コレクションに `inboundSurveyId`, `payload`, `error`, `attemptedAt` を保存し、人力リカバリを可能にする。
- 同時にSlack通知を発報して管理者に即座に伝える（Slack Webhookを別ルートとして用意）。
- Slackも失敗するような致命的障害時はアプリログにERRORで残し、監視で拾う前提。
