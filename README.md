## Makoto Club Services

`base-services` と同様に、サービスごとにディレクトリを分けた構成で Makoto Club ドメインの API / DB をまとめています。

```
makoto-club-services/
├── api/               # Go (chi) ベースの HTTP API
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
├── docker-compose.yml # API コンテナ
└── Makefile           # よく使う操作
```

### 使い方

```bash
cd makoto-club-services
make up        # 初回は build も自動で実行
make logs      # API コンテナのログ確認
make down      # 停止
```

- `http://localhost:8080/healthz` で MongoDB Atlas との疎通チェック。
- `http://localhost:8080/api/ping` で `pings` コレクションの最新ドキュメントを返却します（初回起動時に `{"message":"pong"}` を自動投入）。
- `http://localhost:8080/api/stores` で Atlas の `tokumei-tenpo-ankeet` を集計した店舗サマリーを取得できます。

### Make ターゲット

| ターゲット | 説明 |
| --- | --- |
| `make up` | コンテナをビルドして起動 |
| `make down` | コンテナ停止 (`docker compose down`) |
| `make restart` | 再起動 |
| `make logs` | ログをフォロー |
| `make ps` | 稼働状況の確認 |
| `make tidy` | Go 依存関係の更新 (`go mod tidy`) |

### 環境変数

| 変数名 | 既定値 | 説明 |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | API サーバーのリッスンアドレス |
| `MONGO_URI` | `mongodb+srv://...` | MongoDB Atlas への接続 URI |
| `MONGO_DB` | `makoto-club` | 利用するデータベース名 |
| `SURVEY_COLLECTION` | `tokumei-tenpo-ankeet` | アンケートを格納するコレクション名 |
| `PING_COLLECTION` | `pings` | Ping ドキュメント用コレクション |
| `MONGO_CONNECT_TIMEOUT` | `10s` | MongoDB 接続タイムアウト |
| `TIMEZONE` | `Asia/Tokyo` | 日時表示に利用するタイムゾーン |

### Atlas 接続について

- `docker-compose.yml` の `MONGO_URI` は MongoDB Atlas の接続文字列に置き換えています。プロジェクトごとのクレデンシャルを利用してください。
- 開発者のマシンから直接クエリを実行する場合は、Atlas 側の IP アクセスリストにローカル IP を追加し、`mongodb+srv://...` を使って接続できます。
