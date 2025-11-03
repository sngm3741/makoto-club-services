# Discord Survey Collector Bot

Python + discord.py bot that scans every text channel across all guilds the bot belongs to and collects messages containing the keyword `匿名店舗アンケート`. Matching posts are saved to `data/surveys.json`.

## Requirements

- Docker / Docker Compose
- A Discord bot token with the **MESSAGE CONTENT INTENT** enabled

## Setup

1. Copy `.env.example` to `.env` and set `DISCORD_TOKEN`.

   ```bash
   cp .env.example .env
   # edit .env and fill in the token
   ```

2. Build and run via Docker Compose:

   ```bash
   docker compose up --build
   ```

   The bot logs progress to STDOUT, iterates all guilds/channels, and writes the collected messages to `data/surveys.json`.

3. After completion you should see a message similar to:

   ```
   ✅ 収集完了 (計 XX 件) -> data/surveys.json
   ```

## Notes

- The bot requests `message_content` intent. Ensure it is enabled in the Discord developer portal for your bot.
- Channels where the bot lacks `Read Message History` permission are skipped with a warning.
- Output location can be changed via `OUTPUT_FILE` environment variable.
