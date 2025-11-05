import asyncio
import hashlib
import json
import os
import re
import unicodedata
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Dict, Iterable, Tuple
from uuid import uuid4

import discord
from discord.ext import commands
from dotenv import load_dotenv

load_dotenv()

TOKEN = os.getenv("DISCORD_TOKEN")
if not TOKEN:
    raise RuntimeError("DISCORD_TOKEN is not set. Please define it in the .env file.")

MEDIA_ROOT = Path(os.getenv("MEDIA_ROOT", "data/media"))
MEDIA_ROOT.mkdir(parents=True, exist_ok=True)

STORE_ROOT = Path(os.getenv("STORE_ROOT", "data/stores"))
STORE_ROOT.mkdir(parents=True, exist_ok=True)

KEYWORD = "匿名店舗アンケート"

_KANA_ROMAJI_OVERRIDES: Dict[str, str] = {
    "SI": "shi",
    "ZI": "ji",
    "JI": "ji",
    "TI": "chi",
    "DI": "ji",
    "TU": "tsu",
    "DU": "zu",
    "SHI": "shi",
    "CHI": "chi",
    "TSU": "tsu",
    "HU": "fu",
    "FU": "fu",
    "WO": "o",
    "WI": "wi",
    "WE": "we",
    "VA": "va",
    "VI": "vi",
    "VE": "ve",
    "VO": "vo",
    "V": "v",
}

intents = discord.Intents.default()
intents.guilds = True
intents.messages = True
intents.message_content = True

bot = commands.Bot(command_prefix="!", intents=intents)


@dataclass
class StoreState:
    data: dict
    path: Path
    surveys: Dict[str, dict] = field(default_factory=dict)
    survey_legacy: Dict[str, dict] = field(default_factory=dict)
    assets: Dict[str, dict] = field(default_factory=dict)
    asset_legacy: Dict[str, dict] = field(default_factory=dict)


store_states: Dict[str, StoreState] = {}
store_states_by_name: Dict[Tuple[str, str], StoreState] = {}


def _isoformat(dt: datetime) -> str:
    if dt.tzinfo is None:
        return dt.isoformat() + "Z"
    return dt.isoformat()


def _romanize_kana_token(token: str) -> str:
    upper = token.upper()
    return _KANA_ROMAJI_OVERRIDES.get(upper, upper.lower())


def _safe_path_component(value: str) -> str:
    normalized = unicodedata.normalize("NFKC", (value or "").strip())
    if not normalized:
        return "unknown"

    pieces: list[str] = []
    for ch in normalized:
        if ch.isascii():
            if ch.isalnum():
                pieces.append(ch)
            elif ch in {" ", "-", "_"}:
                pieces.append("-")
            continue

        ascii_equiv = unicodedata.normalize("NFKD", ch).encode("ascii", "ignore").decode("ascii")
        if ascii_equiv:
            pieces.append(ascii_equiv)
            continue

        name = unicodedata.name(ch, "")
        candidate = ""
        if name == "KATAKANA-HIRAGANA PROLONGED SOUND MARK":
            pieces.append("-")
            continue
        if name.startswith("CJK UNIFIED IDEOGRAPH-"):
            suffix = name.rsplit("-", 1)[-1].lower()
            candidate = f"u{suffix}"
        elif "LETTER" in name:
            letter_part = name.split("LETTER", 1)[1]
            letter_part = letter_part.replace(" SMALL ", " ")
            letter_part = letter_part.strip()
            tokens = [tok for tok in re.split(r"[-\\s]+", letter_part) if tok]
            if tokens:
                candidate = "".join(_romanize_kana_token(tok) for tok in tokens)
        elif name:
            candidate = name

        if candidate:
            pieces.append(candidate)
        else:
            pieces.append(f"u{ord(ch):04x}")

    raw_slug = "".join(pieces)
    slug = re.sub(r"[^0-9A-Za-z]+", "-", raw_slug).strip("-").lower()
    if not slug:
        digest = hashlib.sha1(normalized.encode("utf-8")).hexdigest()[:10]
        slug = f"slug-{digest}"
    return slug[:80]


def _compute_store_path(guild_name: str, channel_name: str) -> Path:
    guild_part = _safe_path_component(guild_name)
    channel_part = _safe_path_component(channel_name)
    return STORE_ROOT / guild_part / f"{channel_part}.json"


def _legacy_key_from_values(
    *,
    guild_name: str,
    category_name: str,
    channel_name: str,
    thread_name: str | None,
    author: str,
    created_at: datetime | str,
    content: str,
) -> str:
    thread_part = thread_name or ""
    created_str = created_at if isinstance(created_at, str) else _isoformat(created_at)
    return "|".join(
        [
            guild_name,
            category_name,
            channel_name,
            thread_part,
            author,
            created_str,
            content,
        ]
    )


def _iter_states() -> Iterable[StoreState]:
    seen: set[int] = set()
    for state in store_states.values():
        ident = id(state)
        if ident in seen:
            continue
        seen.add(ident)
        yield state
    for state in store_states_by_name.values():
        ident = id(state)
        if ident in seen:
            continue
        seen.add(ident)
        yield state


def _register_state(state: StoreState, *, channel_id: str | None, guild_name: str, channel_name: str) -> None:
    if channel_id:
        store_states[channel_id] = state
        state.data["channel_id"] = channel_id
    to_remove = [key for key, value in store_states_by_name.items() if value is state and key != (guild_name, channel_name)]
    for key in to_remove:
        del store_states_by_name[key]
    store_states_by_name[(guild_name, channel_name)] = state


def _load_existing_store_files() -> None:
    if not STORE_ROOT.exists():
        return

    for store_file in STORE_ROOT.glob("**/*.json"):
        try:
            data = json.loads(store_file.read_text(encoding="utf-8"))
        except json.JSONDecodeError:
            print(f"⚠️ ストアファイルを読み込めませんでした: {store_file}")
            continue

        data.setdefault("surveys", [])
        data.setdefault("assets", [])

        state = StoreState(data=data, path=store_file)

        for entry in data["surveys"]:
            message_id = str(entry.get("message_id") or "")
            if message_id:
                state.surveys[message_id] = entry
            else:
                key = _legacy_key_from_values(
                    guild_name=data.get("guild", ""),
                    category_name=data.get("category", ""),
                    channel_name=data.get("channel", ""),
                    thread_name=entry.get("thread"),
                    author=entry.get("author", ""),
                    created_at=entry.get("created_at", ""),
                    content=entry.get("content", ""),
                )
                state.survey_legacy[key] = entry

        for entry in data["assets"]:
            message_id = str(entry.get("message_id") or "")
            if message_id:
                state.assets[message_id] = entry
            else:
                key = _legacy_key_from_values(
                    guild_name=data.get("guild", ""),
                    category_name=data.get("category", ""),
                    channel_name=data.get("channel", ""),
                    thread_name=entry.get("thread"),
                    author=entry.get("author", ""),
                    created_at=entry.get("created_at", ""),
                    content=entry.get("content", ""),
                )
                state.asset_legacy[key] = entry

        channel_id = str(data.get("channel_id") or "")
        guild_name = data.get("guild", "")
        channel_name = data.get("channel", "")
        if channel_id:
            _register_state(state, channel_id=channel_id, guild_name=guild_name, channel_name=channel_name)
        else:
            store_states_by_name[(guild_name, channel_name)] = state


def _write_store_state(state: StoreState) -> None:
    state.path.parent.mkdir(parents=True, exist_ok=True)
    state.path.write_text(json.dumps(state.data, ensure_ascii=False, indent=2), encoding="utf-8")


async def _handle_attachments(
    message: discord.Message,
    entry: dict,
    *,
    guild_name: str,
    channel_name: str,
    thread_name: str | None,
) -> None:
    if not message.attachments:
        return

    base_dir = MEDIA_ROOT
    base_dir.mkdir(parents=True, exist_ok=True)

    attachments = entry.get("attachments")
    if attachments is None:
        attachments = []
    existing = {att.get("id"): att for att in attachments if att.get("id")}

    for idx, attachment in enumerate(message.attachments):
        attachment_id = str(attachment.id)
        suffix = Path(attachment.filename or f"file_{idx}").suffix or ""

        existing_entry = existing.get(attachment_id)
        if existing_entry:
            stored_filename = existing_entry.get("stored_filename")
        else:
            stored_filename = None

        if not stored_filename:
            stored_filename = f"{uuid4().hex}{suffix}"

        local_path = base_dir / stored_filename

        if existing_entry:
            existing_entry["stored_filename"] = stored_filename
            existing_entry["local_path"] = local_path.as_posix()
            if not local_path.exists():
                try:
                    await attachment.save(local_path)
                except Exception as exc:  # pylint: disable=broad-except
                    print(f"        ⚠️ 添付ファイルの保存に失敗しました: {exc}")
                    continue

        if attachment_id not in existing:
            if not local_path.exists():
                try:
                    await attachment.save(local_path)
                except Exception as exc:  # pylint: disable=broad-except
                    print(f"        ⚠️ 添付ファイルの保存に失敗しました: {exc}")
                    continue

            attachment_entry = {
                "id": attachment_id,
                "file_name": attachment.filename,
                "stored_filename": stored_filename,
                "content_type": attachment.content_type,
                "size": attachment.size,
                "original_url": attachment.url,
                "local_path": local_path.as_posix(),
            }
            attachments.append(attachment_entry)
            existing[attachment_id] = attachment_entry

    entry["attachments"] = attachments


async def _collect_history(
    history_owner: discord.abc.Messageable,
    *,
    guild_name: str,
    category_name: str,
    channel_name: str,
    thread_name: str | None,
    store_state: StoreState,
) -> None:
    store_data = store_state.data
    surveys = store_state.surveys
    survey_legacy = store_state.survey_legacy
    assets = store_state.assets
    asset_legacy = store_state.asset_legacy

    try:
        async for message in history_owner.history(limit=None, oldest_first=True):
            content = message.content or ""
            message_id = str(message.id)
            author_name = str(message.author)

            if KEYWORD in content:
                entry = surveys.get(message_id)
                if entry is None:
                    legacy_key = _legacy_key_from_values(
                        guild_name=guild_name,
                        category_name=category_name,
                        channel_name=channel_name,
                        thread_name=thread_name,
                        author=author_name,
                        created_at=message.created_at,
                        content=content,
                    )
                    entry = survey_legacy.pop(legacy_key, None)
                    if entry is None:
                        entry = {
                            "guild": guild_name,
                            "category": category_name,
                            "channel": channel_name,
                            "author": author_name,
                            "content": content,
                            "created_at": _isoformat(message.created_at),
                            "message_id": message_id,
                        }
                        if thread_name:
                            entry["thread"] = thread_name
                        store_data.setdefault("surveys", []).append(entry)
                    else:
                        entry["message_id"] = message_id
                        entry["author"] = author_name
                        entry["content"] = content
                        entry["created_at"] = _isoformat(message.created_at)
                        if thread_name:
                            entry["thread"] = thread_name
                        else:
                            entry.pop("thread", None)
                    surveys[message_id] = entry

                    await _handle_attachments(
                        message,
                        entry,
                        guild_name=guild_name,
                        channel_name=channel_name,
                        thread_name=thread_name,
                    )

                    preview = content.splitlines()[0] if content else ""
                    if len(preview) > 60:
                        preview = preview[:57] + "..."
                    prefix = "      " if thread_name else "    "
                    print(f"{prefix}→ {message.author}: {preview}")
                else:
                    skip_prefix = "      " if thread_name else "    "
                    print(f"{skip_prefix}↺ 既存アンケートを更新: {author_name} ({message_id})")
                    await _handle_attachments(
                        message,
                        entry,
                        guild_name=guild_name,
                        channel_name=channel_name,
                        thread_name=thread_name,
                    )

            else:
                if not message.attachments:
                    continue

                entry = assets.get(message_id)
                if entry is None:
                    legacy_key = _legacy_key_from_values(
                        guild_name=guild_name,
                        category_name=category_name,
                        channel_name=channel_name,
                        thread_name=thread_name,
                        author=author_name,
                        created_at=message.created_at,
                        content=content,
                    )
                    entry = asset_legacy.pop(legacy_key, None)
                    if entry is None:
                        entry = {
                            "message_id": message_id,
                            "author": author_name,
                            "content": content,
                            "created_at": _isoformat(message.created_at),
                        }
                        if thread_name:
                            entry["thread"] = thread_name
                        store_data.setdefault("assets", []).append(entry)
                    else:
                        entry["message_id"] = message_id
                        entry["author"] = author_name
                        entry["content"] = content
                        entry["created_at"] = _isoformat(message.created_at)
                        if thread_name:
                            entry["thread"] = thread_name
                        else:
                            entry.pop("thread", None)
                    assets[message_id] = entry

                    await _handle_attachments(
                        message,
                        entry,
                        guild_name=guild_name,
                        channel_name=channel_name,
                        thread_name=thread_name,
                    )
                    prefix = "      " if thread_name else "    "
                    print(f"{prefix}◎ 画像投稿: {author_name} ({message_id})")
                else:
                    skip_prefix = "      " if thread_name else "    "
                    print(f"{skip_prefix}↺ 既存画像投稿を更新: {author_name} ({message_id})")
                    await _handle_attachments(
                        message,
                        entry,
                        guild_name=guild_name,
                        channel_name=channel_name,
                        thread_name=thread_name,
                    )

        await asyncio.sleep(0.25)
    except discord.Forbidden:
        print("    ⚠️ アクセス権限がないためスキップします")
    except discord.HTTPException as exc:
        print(f"    ⚠️ 取得失敗: {exc}")
        await asyncio.sleep(1)
    except Exception as exc:  # pylint: disable=broad-except
        print(f"    ⚠️ 想定外のエラー: {exc}")
        await asyncio.sleep(1)


def _get_state_for_channel(guild: discord.Guild, channel: discord.TextChannel, category_name: str) -> StoreState:
    channel_id = str(channel.id)
    state = store_states.get(channel_id)
    if state is None:
        state = store_states_by_name.get((guild.name, channel.name))
        if state is None:
            data = {
                "guild": guild.name,
                "guild_id": str(guild.id),
                "category": category_name,
                "channel": channel.name,
                "channel_id": channel_id,
                "surveys": [],
                "assets": [],
            }
            path = _compute_store_path(guild.name, channel.name)
            state = StoreState(data=data, path=path)
        _register_state(state, channel_id=channel_id, guild_name=guild.name, channel_name=channel.name)
    else:
        _register_state(state, channel_id=channel_id, guild_name=guild.name, channel_name=channel.name)

    state.data["guild"] = guild.name
    state.data["guild_id"] = str(guild.id)
    state.data["category"] = category_name
    state.data["channel"] = channel.name
    state.data["channel_id"] = channel_id

    state.path = _compute_store_path(guild.name, channel.name)
    state.data.setdefault("surveys", [])
    state.data.setdefault("assets", [])
    return state


@bot.event
async def on_ready() -> None:
    print(f"✅ Logged in as {bot.user} ({bot.user.id})")

    _load_existing_store_files()

    for guild in bot.guilds:
        print(f"サーバー: {guild.name}")
        for channel in guild.text_channels:
            category_name = channel.category.name if channel.category else "（カテゴリなし）"
            print(f"  - [{category_name}] #{channel.name}")

            state = _get_state_for_channel(guild, channel, category_name)

            await _collect_history(
                channel,
                guild_name=guild.name,
                category_name=category_name,
                channel_name=channel.name,
                thread_name=None,
                store_state=state,
            )

            for thread in channel.threads:
                print(f"    ↪︎ スレッド: {thread.name}")
                await _collect_history(
                    thread,
                    guild_name=guild.name,
                    category_name=category_name,
                    channel_name=channel.name,
                    thread_name=thread.name,
                    store_state=state,
                )

            async for thread in channel.archived_threads(limit=None, private=False):
                print(f"    ↪︎ スレッド(アーカイブ): {thread.name}")
                await _collect_history(
                    thread,
                    guild_name=guild.name,
                    category_name=category_name,
                    channel_name=channel.name,
                    thread_name=thread.name,
                    store_state=state,
                )

            try:
                async for thread in channel.archived_threads(limit=None, private=True):
                    print(f"    ↪︎ スレッド(プライベート/アーカイブ): {thread.name}")
                    await _collect_history(
                        thread,
                        guild_name=guild.name,
                        category_name=category_name,
                        channel_name=channel.name,
                        thread_name=thread.name,
                        store_state=state,
                    )
            except discord.Forbidden:
                print("    ⚠️ プライベートスレッドにはアクセスできません")

            _write_store_state(state)

    for state in _iter_states():
        _write_store_state(state)

    total_surveys = sum(len(state.data.get("surveys", [])) for state in _iter_states())
    total_assets = sum(len(state.data.get("assets", [])) for state in _iter_states())
    print(f"✅ 収集完了: アンケート {total_surveys} 件, 画像投稿 {total_assets} 件 -> {STORE_ROOT}")
    await bot.close()


if __name__ == "__main__":
    bot.run(TOKEN)
