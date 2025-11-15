# Internal Package Layout

このディレクトリはDDD/オニオンアーキテクチャに沿って、各層を以下のように分割する計画である。

- `config`: 環境変数や設定ファイルの読み込みロジック。
- `server`: ルーター初期化やHTTPサーバー起動コード。
- `public/domain`, `public/application`: ユーザー向けコンテキストのドメインモデルとアプリケーションサービス。
- `admin/domain`, `admin/application`: 管理向けコンテキスト。
- `interfaces/http/public`, `interfaces/http/admin`: chi ハンドラやルーティング層。
- `infrastructure/mongo`: MongoDB リポジトリ実装。
- `infrastructure/notification`: Discord/Slack などのクライアント。
- `infrastructure/auth`: JWTやCookie署名ユーティリティ。

空ディレクトリは `.gitkeep` を置かず、必要になり次第コードを追加していく。
