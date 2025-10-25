## Makoto Club Services

Makoto Club ドメイン向けの API 群をまとめた Docker Compose プロジェクトです。Go 製の API コンテナ (`api/`) をビルドして MongoDB Atlas に接続します。

```
makoto-club-services/
├── api/               # Go (chi) ベースの HTTP API
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
├── docker-compose.yml # API を起動する Compose 定義
└── Makefile           # 開発用ユーティリティ
```

### ローカル開発

1. ルートにある `.env` はローカル向けの設定です（VPS 用とは分離）。MongoDB Atlas の URI などはそのまま利用できます。
2. `make up` を実行すると、自動的に `EDGE_NETWORK`（既定値 `makoto-club-local`）の Docker ネットワークを作成したうえで、API コンテナをビルドして起動します。
3. ログ確認は `make logs`、停止は `make down`。
4. API の疎通は `http://localhost:8080/api/healthz` や `http://localhost:8080/api/ping` で確認できます。
5. LINE ログインで発行された JWT を検証したい場合は、リクエストヘッダーに `Authorization: Bearer <token>` を付けて `http://localhost:8080/api/auth/verify` を叩いてください。

> 初回起動時に MongoDB Atlas の IP アクセスリストへ開発マシンのグローバル IP を追加しておく必要があります。

### 本番デプロイ（VPS）

- VPS 側では `~/makoto-club-services/.env` に本番用の値（例: `EDGE_NETWORK=project01-edge` や本番サブドメイン）を配置してください。リポジトリの `.env` はローカル専用なので、そのままコピーするとホスト名が `localhost` になります。
- GitHub Actions (`.github/workflows/deploy.yml`) で `main` ブランチの更新ごとに GHCR へビルド&プッシュし、VPS へ SSH 経由でデプロイします。
- `base-services/reverse-proxy` の `nginxproxy/nginx-proxy` と同じ Docker ネットワーク（例: `project01-edge`）に参加させることで、`https://makoto.iqx9l9hxmw0dj3kt.space/api/…` へルーティングできます。

### Makefile ターゲット

| ターゲット | 説明 |
| --- | --- |
| `make up` | `.env` を読み込んでコンテナをビルド & 起動。必要なら `EDGE_NETWORK` の Docker ネットワークを自動生成 |
| `make down` | 停止 (`docker compose down`) |
| `make restart` | 再起動 |
| `make logs` | ログをフォロー |
| `make ps` | 稼働状況の確認 |
| `make network` | `EDGE_NETWORK` の Docker ネットワークだけ先に作成 |
| `make tidy` | Go 依存関係の整理 (`go mod tidy`) |

### 環境変数

`.env` / VPS の `.env` で指定する主な変数です。ローカルの既定値は括弧内に記載しています。

| 変数名 | 既定値 (ローカル) | 説明 |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | API サーバーのリッスンアドレス |
| `MONGO_URI` | `mongodb+srv://...` | MongoDB Atlas への接続 URI |
| `MONGO_DB` | `makoto-club` | 利用するデータベース名 |
| `SURVEY_COLLECTION` | `tokumei-tenpo-ankeet` | アンケートコレクション名 |
| `PING_COLLECTION` | `pings` | Ping コレクション名 |
| `MONGO_CONNECT_TIMEOUT` | `10s` | MongoDB 接続タイムアウト |
| `TIMEZONE` | `Asia/Tokyo` | 日時表示に利用するタイムゾーン |
| `GHCR_USER` | `sngm3741` | GHCR の名前空間（CI/CD 用） |
| `VIRTUAL_HOST` | `localhost` | `nginx-proxy` が公開するホスト名（VPS では `makoto.iqx9l9hxmw0dj3kt.space` などに変更） |
| `VIRTUAL_PATH` | `/` | サブパスでマウントする場合に指定（VPS では `/api/` を推奨） |
| `LETSENCRYPT_HOST` | `""` | ACME Companion 用のホスト名。ローカルでは空欄のままでOK |
| `LETSENCRYPT_EMAIL` | `devnull@example.com` | 証明書更新通知先（VPS では実アドレスに変更） |
| `EDGE_NETWORK` | `makoto-club-local` | 共有 Docker ネットワーク名（VPS では `project01-edge` などに変更） |
| `AUTH_LINE_JWT_SECRET` | _(必須)_ | `auth-line` が署名した JWT を検証するための共有秘密鍵 |
| `AUTH_LINE_JWT_ISSUER` | `makoto-club-auth` | 検証時に期待する `iss` 値 |
| `AUTH_LINE_JWT_AUDIENCE` | _(必要に応じて)_ | 検証時に期待する `aud` 値（空ならチェックしない） |

### 補足

- VPS で `docker compose` を単独実行する場合は、事前に `docker login ghcr.io` で GHCR にログインしておくか、GitHub Actions のデプロイを利用してください。
- `.env` ファイルは `.gitignore` 対象なので、必要に応じて `.env.example` を作成するか、共有は別の安全な手段で行ってください。
