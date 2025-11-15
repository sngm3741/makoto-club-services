# Makoto Club Backend ExecPlan

このExecPlanはリポジトリ内の `backend/docs/PLANS.md` として維持し、常に最新状態に更新する。

## Purpose / Big Picture
- `backend/api/main.go` に集約されていた処理をDDD/オニオンアーキテクチャへ移行し、Public/Admin両コンテキストを明確に分離する。
- ユーザーは `/api/v1/stores` / `/api/v1/surveys` で全国ランキングと条件検索を高速に利用でき、匿名アンケート投稿と役に立ったトグルが安定動作する。
- 管理者は `/api/admin/v1/...` 経由で店舗・アンケートを登録/更新でき、未承認投稿とDiscord通知の状態を把握できる。
- 動作確認はGoのユニットテスト＋APIエンドポイントの手動/自動テスト（curl or integration tests）で行う。

## Progress
- [x] (2025-11-13T13:37Z) DDD用ディレクトリ構成と `cmd/api/main.go` へのエントリ分離
- [x] (2025-11-13T14:45Z) HelpfulトグルCookie実装＋`survey_helpful_votes` ユニーク制約＋テスト
- [x] (2025-11-13T15:40Z) Mongo Seed CLI（`cmd/seed`）でサンプルデータ投入＆主要インデックス自動作成
- [x] Mongoリポジトリ実装の仕上げ (`stores`, `surveys`, `inbound_surveys`, `survey_helpful_votes`) と集約マッピングの欠落補完
- [x] Public APIハンドラをQuery/Commandサービス経由に完全移行（Store/Survey一覧・詳細・投稿）＋業種/タグ/SNS/写真10枚を露出
- [x] (2025-11-14T13:05Z) HTTP起動処理を `internal/server` へ移動し、`cmd/api/main.go` を設定読込＋起動の薄いブートストラップに整理
- [x] (2025-11-14T14:10Z) Public HTTPハンドラを `internal/interfaces/http/public` に移し、`server.Run` からDIする構成を確立（匿名投稿/Helpful/CORS含む）
- [x] (2025-11-14T15:10Z) Admin HTTPハンドラを `internal/interfaces/http/admin` に移し、Mongo直接操作/バリデーション/統計更新を新パッケージへ集約
- [x] (2025-11-15T02:10Z) Admin Store Service：検索/詳細/作成/更新＋重複検知＋業種/タグ配列更新
- [x] (2025-11-15T04:05Z) Admin Survey Service：一覧/詳細/作成/更新と統計再計算の責務整理（Mongoリポジトリ＋VO/DTO変換）
- [x] (2025-11-15T04:20Z) Admin HTTPレビュー系をSurveyService経由に差し替え（一覧/詳細/更新/店舗紐付け投稿）
- [x] (2025-11-15T06:10Z) HTTP層再配置（`internal/interfaces/http/{public,admin}`）: `internal/server` からlegacyハンドラ・DTOを完全撤去し、DIのみの薄い層へ整理
- [x] (2025-11-15T07:05Z) Discord→Slack通知フロー + 失敗レコード保管（Messenger送信リトライ→Slackフォールバック→`failed_notifications` 永続化）
- [ ] エンドツーエンド検証（API, ドキュメント更新, 会議メモ追記）

## Surprises & Discoveries
- Observation: 旧Admin/Public双方でストア統計の集計が `waitTimeHours` フィールドを参照しており、Mongo上に存在しないため平均待機時間が常に0扱いになっていた。
  Evidence: `internal/interfaces/http/admin/helpers.go` / `internal/server/server.go` の集計パイプラインが `waitTimeHours` を指定、ReviewDocumentには `waitTimeMinutes` のみ存在。

## Decision Log
- Decision: Public/API v1 ルーティング＆Query/Command分離を採用
  Rationale: ユーザー向け閲覧/投稿と管理機能を疎結合化し、拡張耐性を高めるため
  Date/Author: 2025-11-10 Codex
- Decision: `stores` は全国ランキング優先で `avgRating` / `avgEarning` 単独インデックスのみを貼る
  Rationale: 現状件数が数千規模のため、ソート体験を最優先
  Date/Author: 2025-11-10 Codex
- Decision: Helpful投票は署名付きCookie + `survey_helpful_votes` ユニーク制約で管理
  Rationale: 匿名性と不正防止のバランス
  Date/Author: 2025-11-10 Codex
- Decision: Discord失敗時はSlack通知＋失敗コレクションに保管
  Rationale: 投稿取りこぼし防止
  Date/Author: 2025-11-10 Codex
- Decision: 店舗作成は「店舗名 + 支店名(空ならnull)」の完全一致で重複を拒否し、曖昧投稿は管理者が手動で突き合わせる
  Rationale: 自動Upsertを避けてドメインマスターの判断を尊重するため
  Date/Author: 2025-11-13 Codex
- Decision: Adminアンケートは店舗詳細画面からのみ作成し、`storeID` は登録後に不変。UI上でID指定入力はさせない
  Rationale: 「アンケートが店舗に紐づかない」ケースをドメインとして禁止するため
  Date/Author: 2025-11-13 Codex
- Decision: 都道府県・業種ともコード化せずフリーテキスト（管理画面での正規化）で扱い、業種は複数選択の配列にする
  Rationale: コストを抑えつつ柔軟な入力を許容するため
  Date/Author: 2025-11-13 Codex
- Decision: Public検索のソートは「新着/評価順/稼ぎ順」の3種のみ、helpful順は提供しない。アンケートも同様
  Rationale: ユーザーが最も多用する優先順位に集中し、インデックス設計をシンプルに保つため
  Date/Author: 2025-11-13 Codex
- Decision: 未承認投稿(inbound)は公開スキーマと同じフィールドを`string`中心で受け取り、Discord通知と同一内容を保管する
  Rationale: 書き手の制約を減らしつつ後工程で正規化できるようにするため
  Date/Author: 2025-11-13 Codex
- Decision: Public API では業種・タグ・SNS・写真10枚までをそのままJSONで露出し、匿名投稿も同じ構造で受け付ける
  Rationale: フロント実装とデータモデルを完全一致させ、後からの変換コストを排除するため
  Date/Author: 2025-11-13 Codex

## Outcomes & Retrospective
- Pending（実装完了後に追記）

## Context and Orientation
- ルート: `/Users/sngm3741/Workspace/develop/makoto-club`
- 現行構造: `backend/api/main.go` にHTTPルーター・Mongo接続・業務ロジックが集中。
- 新構造: `cmd/api/main.go`（エントリ）、`internal/{public,admin}` などのパッケージでコンテキスト分離。リポジトリ層は `internal/infrastructure/mongo/...` に配置予定。
- 依存: chi, MongoDB driver, jwt, Discord/Slack messenger。
- 管理フロー: Discord経由で未承認アンケートを確認→管理UIで店舗検索→存在しなければ店舗登録→店舗詳細画面からアンケート登録。登録済みアンケートの `storeID` 変更は禁止。

## Plan of Work
1. **ドメイン/アプリ層の固め**: Store/Survey/Inbound/HelpfulのエンティティとリポジトリIFを現仕様に合わせて確定し、Adminサービスに「店舗重複検知」「storeID不変」「統計再計算」「業種/タグ配列更新」を組み込む。
2. **Mongo実装の整備**: `internal/infrastructure/mongo` の各リポジトリで配列業種・タグ・画像URL・SNSリンク・価格などをマッピングし、必要な単一/複合インデックスとユニーク制約を `SetupIndexes` で実行できるようにする。
3. **Public HTTP層の切り出し**: `internal/interfaces/http/public` にルーターを移し、Store/Survey一覧・詳細・投稿・helpfulトグルをQuery/Commandサービス経由で提供（現在は`cmd/api/main.go`に直書き、ここを移植する）。
4. **Admin Storeハンドラ整備**: Store検索/詳細/作成/更新APIを `StoreService` 経由にし、同名＋同支店(null許容)検知や業種配列の追加/削除、画像URL/タグ/SNS更新をまとめる（HTTPレイヤー移植も含む）。
5. **Admin Surveyハンドラ整備**: Survey一覧/詳細/作成/更新を `SurveyService` に集約し、店舗詳細画面からのみ作成→storeID固定→登録後に店舗統計を再計算。未承認投稿閲覧APIもここで提供。
6. **通知ライン/失敗管理**: 匿名投稿受付でDiscord通知→即時リトライ→Slackフォールバック→`failed_notifications` への保管までをmessengerサービスで共通化し、障害時はERRORログとSlack個別DMを飛ばす。
7. **横断初期化の共通化**: JWT設定、CORS、Helpful Cookieシークレット、Discord/Slackクライアント、HTTPクライアントを `internal/server` 等にまとめ、`cmd/api/main.go` では依存注入のみ行う。
8. **テスト/検証/ドキュメント**: Store/Surveyサービス・Mongoリポジトリ・Helpfulトグル・通知失敗パスのユニットテストを追加し、curl検証手順とミーティングメモをdocsへ反映。

## Concrete Steps
1. `go test ./...` が通る状態を把握し、Mongoドキュメント構造とマッピング差分を洗い出す（`internal/infrastructure/mongo/*.go`）。
2. Admin/Publicサービスのインターフェースを更新し、`cmd/api/main.go` からの直接ロジックを削減するPR単位のブランチ作業を行う。
3. 新HTTP層へハンドラを移植しつつ、chiルーター初期化とミドルウェア適用をserverパッケージで共通化。
4. Discord/Slack通知を抽象化したAdapterを実装し、匿名投稿コマンドから利用。Mongoへ失敗レコードをInsertする関数もセットにする。
5. curlスクリプト or `make e2e` で主要APIを叩く手順を docs に追加し、最終確認。

## Validation and Acceptance
- `go test ./...` が成功し、Store/Suvey/Adminサービスの単体テストが主要フロー（重複検知・統計更新・helpfulトグル）をカバーしている。
- `curl http://localhost:PORT/api/v1/stores?sort=rating` などで新着/評価/稼ぎソートが期待通りに返却される。
- Admin APIで同名＋同支店を登録しようとすると409が返り、 storeID を変更するPATCHは400で拒否される。
- 匿名投稿→Discord成功、Discord失敗時にSlack通知＋`failed_notifications` へ記録されることをログとDBで確認。

## Idempotence and Recovery
- Mongoインデックス作成は `createIndexes` を利用し複数回実行しても安全にする。
- 通知失敗レコードは再送ジョブで再試行できる構造にし、アプリ再起動で自動復旧可能。
- CookieキーやJWT設定は環境変数で差し替え可能にし、再デプロイで即ローテーションできる。

## Artifacts and Notes
- ユニットテスト結果・主要curlレスポンス・通知失敗ログをここに随時追記する。
- Seed CLI: `cd backend/api && GOCACHE=$(pwd)/.gocache go run ./cmd/seed -env local -stores 10 -surveys 100 -inbound 5 -helpful 30`（.envを自動読込、dropフラグでコレクション初期化）
- Public投稿APIサンプル: `POST /reviews` で `industries[]`, `tags[]`, `photos[].{id,storedPath,publicUrl,contentType}` を含む最大10枚のJSONを送信し、そのままMongoへ保存・公開される。

## Interfaces and Dependencies
- `StoreRepository`: `Find`, `FindByID`, `Create`, `Update`, `ExistsByNameAndBranch`.
- `SurveyRepository`: `Find`, `FindByID`, `Create`, `Update`, `IncrementHelpful`, `RecalculateStoreStats`.
- `InboundSurveyRepository`: `Create`, `List`, `Get`.
- `HelpfulVoteRepository`: `Toggle(surveyID, voterID, desiredState)`。
- `NotificationClient`: `SendDiscord(payload)`, `SendSlack(payload)`, `RecordFailure(payload, err)`.

---
Updated: 2025-11-13 — Adminフロー/インデックス/通知ラインに関する決定＋Mongo Seed CLI/初期データ投入手順を反映。
Updated: 2025-11-14 — HTTP起動処理をinternal/serverへ移し、今後interfaces層へ展開する方針を記録。
Updated: 2025-11-14 — Public HTTPレイヤーを`internal/interfaces/http/public`へ移行し、サーバー/インターフェース間のDI方針を追記。
Updated: 2025-11-14 — Admin HTTPレイヤーを`internal/interfaces/http/admin`へ移行し、review/store CRUDの責務をHTTP層に集約。
