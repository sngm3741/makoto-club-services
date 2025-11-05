# Discord Survey Collector Bot

Discord の全テキストチャンネル（およびスレッド）を巡回し、本文に `匿名店舗アンケート` を含むメッセージを収集・保存する bot です。投稿に添付された画像も自動的にダウンロードし、店舗（チャンネル）単位の JSON に紐づけて管理できます。

## 収集されるデータ

- `data/stores/{ギルド}/{チャンネル}.json`
  - `surveys`: 「匿名店舗アンケート」を含むメッセージの一覧（添付ファイルのメタ情報を同梱）
  - `assets`: 同チャンネル内で投稿された画像（アンケートに紐づかないものも含む）
- `data/media/{ギルド}/{チャンネル}/...` に実ファイルを保存
- `data/surveys.json` には収集したアンケートの総一覧（互換用）が出力されます

繰り返し実行しても `message_id` をキーに重複が自動で除外され、新規投稿と新規添付のみ追記されます。

## Requirements

- Python 3.10+
- Discord Bot Token（**MESSAGE CONTENT INTENT** を有効化）
- `discord.py`, `python-dotenv`
- 付属の Dockerfile / docker-compose でも実行可能

## Setup

1. `.env.example` を `.env` にコピーし、`DISCORD_TOKEN` を設定してください。

   ```bash
   cp .env.example .env
   # .env を編集してボットトークンをセット
   ```

2. 実行方法は 2 通りあります。

   **Docker Compose**
   ```bash
   docker compose up --build
   ```

   **ローカル実行（任意）**
   ```bash
   python -m venv .venv
   source .venv/bin/activate
   pip install -r requirements.txt
   python bot.py
   ```

   いずれも進行状況が STDOUT に流れ、完了すると JSON と添付ファイルが `data/` 配下に保存されます。

3. 実行後、次のようなログが表示されます。

   ```
   ✅ 収集完了 (計 XX 件) -> data/surveys.json
   ```

## Notes

- Bot には対象チャンネルで `メッセージ履歴を読む` 権限が必要です。プライベートスレッドは参加していないとアクセスできません。
- ダウンロード先のディレクトリは `MEDIA_ROOT` 環境変数、店舗 JSON の出力先は `STORE_ROOT` で変更できます。
- 既存の `data/surveys.json` や `data/stores/**.json` があれば自動的に読み込まれ、続きから収集されます。
