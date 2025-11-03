import asyncio
import json
import os
from datetime import datetime
from pathlib import Path

import discord
from discord.ext import commands
from dotenv import load_dotenv

load_dotenv()

TOKEN = os.getenv("DISCORD_TOKEN")
if not TOKEN:
    raise RuntimeError("DISCORD_TOKEN is not set. Please define it in the .env file.")

output_path = Path(os.getenv("OUTPUT_FILE", "data/surveys.json"))
output_path.parent.mkdir(parents=True, exist_ok=True)

KEYWORD = "匿名店舗アンケート"

intents = discord.Intents.default()
intents.guilds = True
intents.messages = True
intents.message_content = True

bot = commands.Bot(command_prefix="!", intents=intents)
collected_messages: list[dict] = []


def _isoformat(dt: datetime) -> str:
    if dt.tzinfo is None:
        return dt.isoformat() + "Z"
    return dt.isoformat()


@bot.event
async def on_ready() -> None:
    print(f"✅ Logged in as {bot.user} ({bot.user.id})")

    for guild in bot.guilds:
        print(f"サーバー: {guild.name}")
        for channel in guild.text_channels:
            category_name = channel.category.name if channel.category else "（カテゴリなし）"
            print(f"  - [{category_name}] #{channel.name}")

            try:
                async for message in channel.history(limit=None, oldest_first=True):
                    content = message.content or ""
                    if KEYWORD in content:
                        entry = {
                            "guild": guild.name,
                            "category": category_name,
                            "channel": channel.name,
                            "author": str(message.author),
                            "content": content,
                            "created_at": _isoformat(message.created_at),
                        }
                        collected_messages.append(entry)
                        preview = content.splitlines()[0]
                        if len(preview) > 60:
                            preview = preview[:57] + "..."
                        print(f"    → {message.author}: {preview}")
                await asyncio.sleep(0.25)
            except discord.Forbidden:
                print("    ⚠️ アクセス権限がないためスキップします")
            except discord.HTTPException as exc:
                print(f"    ⚠️ 取得失敗: {exc}")
                await asyncio.sleep(1)
            except Exception as exc:  # pylint: disable=broad-except
                print(f"    ⚠️ 想定外のエラー: {exc}")
                await asyncio.sleep(1)

    with output_path.open("w", encoding="utf-8") as fp:
        json.dump(collected_messages, fp, ensure_ascii=False, indent=2)

    print(f"✅ 収集完了 (計 {len(collected_messages)} 件) -> {output_path}")
    await bot.close()


if __name__ == "__main__":
    bot.run(TOKEN)
