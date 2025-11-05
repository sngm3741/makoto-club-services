import argparse
import json
import math
import os
import pathlib
import re
import unicodedata
from collections import defaultdict
from datetime import datetime, timezone
from typing import Any, Dict, Iterable, List, Optional, Tuple


EMOJI_INDUSTRY_MAP = {
    "🚗": "deriheru",
    "🚙": "dc",
    "🛀": "sopu",
    "🏩": "hoteheru",
    "📦": "hakoheru",
    "🍜": "menesu",
    "🎈": "huesu",
}


FALLBACK_CATEGORY_KEYWORDS = [
    ("ソープ", "sopu"),
    ("箱", "hakoheru"),
    ("箱ヘル", "hakoheru"),
    ("デリヘル", "deriheru"),
    ("ヘルス", "deriheru"),
    ("ホテヘル", "hoteheru"),
    ("メンエス", "menesu"),
    ("メンズエステ", "menesu"),
    ("DC", "dc"),
    ("デリキャバ", "dc"),
    ("風エス", "huesu"),
]


PREFECTURES = [
    "北海道",
    "青森県",
    "岩手県",
    "宮城県",
    "秋田県",
    "山形県",
    "福島県",
    "茨城県",
    "栃木県",
    "群馬県",
    "埼玉県",
    "千葉県",
    "東京都",
    "神奈川県",
    "新潟県",
    "富山県",
    "石川県",
    "福井県",
    "山梨県",
    "長野県",
    "岐阜県",
    "静岡県",
    "愛知県",
    "三重県",
    "滋賀県",
    "京都府",
    "大阪府",
    "兵庫県",
    "奈良県",
    "和歌山県",
    "鳥取県",
    "島根県",
    "岡山県",
    "広島県",
    "山口県",
    "徳島県",
    "香川県",
    "愛媛県",
    "高知県",
    "福岡県",
    "佐賀県",
    "長崎県",
    "熊本県",
    "大分県",
    "宮崎県",
    "鹿児島県",
    "沖縄県",
]

PREFECTURE_ALIASES = {}
for pref in PREFECTURES:
    PREFECTURE_ALIASES[pref] = pref
    base = pref
    for suffix in ("都", "道", "府", "県"):
        if base.endswith(suffix):
            PREFECTURE_ALIASES[base[:-1]] = pref
            PREFECTURE_ALIASES[base[:-1] + suffix] = pref
            base = base[:-1]
    PREFECTURE_ALIASES[base] = pref


POSITIVE_KEYWORDS = [
    "稼げ",
    "稼げた",
    "優しい",
    "良かった",
    "良い",
    "オススメ",
    "おすすめ",
    "最高",
    "楽しい",
    "快適",
    "神",
    "安心",
    "丁寧",
    "感謝",
    "綺麗",
    "キレイ",
    "素敵",
    "親切",
    "助か",
]

NEGATIVE_KEYWORDS = [
    "最悪",
    "稼げない",
    "稼げず",
    "飛ん",
    "干され",
    "保証割れ",
    "クレーム",
    "悪い",
    "怖い",
    "嫌",
    "酷い",
    "地雷",
    "二度と",
    "不満",
    "辞め",
    "帰りました",
    "帰った",
    "帰らされ",
    "厳しい",
    "辛い",
    "渋ら",
    "渋っ",
    "渋る",
    "大変",
    "怒ら",
]


FIELD_PREFIXES = {
    "store": ("店舗：", "店舗:"),
    "period": ("時期：", "時期:", "訪問時期：", "訪問時期:"),
    "age": ("年齢：", "年齢:", "年齢 :"),
    "spec": ("スペック：", "スペック:", "スぺック：", "スペ："),
    "wait": ("待機時間：", "待機時間:", "待機：", "待機時間 :"),
    "earning": ("平均稼ぎ：", "平均稼ぎ:", "平均収入：", "平均収入:", "平均："),
}


def slugify(value: str) -> str:
    normalized = unicodedata.normalize("NFKC", value)
    normalized = remove_emojis(normalized)
    normalized = re.sub(r"[^\w\s-]", " ", normalized)
    normalized = re.sub(r"[\s_-]+", "-", normalized)
    normalized = normalized.strip("-").lower()
    return normalized or "unknown"


def remove_emojis(text: str) -> str:
    return re.sub(r"[\U00010000-\U0010FFFF]", "", text)


def parse_iso_date(value: str) -> Optional[str]:
    try:
        dt = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
    return dt.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")


def wrap_oid(hex_value: str) -> Dict[str, str]:
    return {"$oid": hex_value}


def wrap_date(iso_value: Optional[str]) -> Optional[Dict[str, str]]:
    if not iso_value:
        return None
    return {"$date": iso_value}


def generate_object_id() -> str:
    return os.urandom(12).hex()


def extract_first_line_icon(content: str) -> Tuple[str, str]:
    stripped = content.lstrip()
    if not stripped:
        return "", content

    icon = ""
    if stripped[0] in ("😈", "👼"):
        icon = stripped[0]
        stripped = stripped[1:].lstrip()

    lines = split_lines(stripped)
    if lines and lines[0].lstrip("# ").startswith("匿名店舗アンケート"):
        lines = lines[1:]
    cleaned = "\n".join(lines)
    return icon, cleaned


def split_lines(content: str) -> List[str]:
    return [line.rstrip() for line in content.replace("\r\n", "\n").split("\n")]


def extract_fields(lines: Iterable[str]) -> Tuple[Dict[str, str], List[str]]:
    extracted: Dict[str, str] = {}
    comment_lines: List[str] = []

    for line in lines:
        stripped = line.strip()
        if not stripped:
            comment_lines.append("")
            continue

        matched = False
        for key, prefixes in FIELD_PREFIXES.items():
            for prefix in prefixes:
                if stripped.startswith(prefix):
                    value = stripped[len(prefix):].strip()
                    extracted.setdefault(key, value)
                    matched = True
                    break
            if matched:
                break

        if not matched:
            comment_lines.append(line)

    return extracted, normalize_comment(comment_lines)


def normalize_comment(lines: List[str]) -> List[str]:
    trimmed = [line.rstrip() for line in lines]
    while trimmed and not trimmed[0].strip():
        trimmed.pop(0)
    while trimmed and not trimmed[-1].strip():
        trimmed.pop()
    return trimmed


def detect_prefecture(line: str) -> Tuple[Optional[str], str]:
    for key, prefecture in PREFECTURE_ALIASES.items():
        if key and key in line:
            remainder = line.replace(key, "", 1).strip()
            return prefecture, remainder
    return None, line.strip()


BRANCH_SUFFIXES = [
    "本店",
    "駅前店",
    "駅前",
    "店",
    "支店",
    "別館",
    "本館",
    "別邸",
    "亭",
    "ルーム",
    "ショップ",
]


def split_store_and_branch(name: str) -> Tuple[str, Optional[str]]:
    cleaned = remove_emojis(name).strip()
    if not cleaned:
        return "", None

    match = re.match(r"^(.*?)（([^）]+)）$", cleaned)
    if match:
        return match.group(1).strip(), match.group(2).strip()

    for suffix in BRANCH_SUFFIXES:
        if cleaned.endswith(suffix) and len(cleaned) > len(suffix) + 1:
            base = cleaned[: -len(suffix)].rstrip(" ・-")
            if base:
                return base.strip(), suffix

    return cleaned, None


def extract_numbers(value: str) -> List[float]:
    matches = re.findall(r"\d+(?:\.\d+)?", value.replace("１", "1").replace("２", "2"))
    return [float(m) for m in matches]


def average_from_value(value: str) -> Optional[float]:
    numbers = extract_numbers(value)
    if not numbers:
        return None
    if len(numbers) == 1:
        return numbers[0]
    if len(numbers) >= 2:
        return sum(numbers[:2]) / 2
    return None


def parse_int_field(value: str) -> Optional[int]:
    avg = average_from_value(value)
    if avg is None:
        return None
    return int(round(avg))


def detect_industry_code(channel: str, category: str, content_store_line: str) -> Optional[str]:
    channel = channel.strip()
    category = category.strip()

    if channel:
        first = channel[0]
        if first in EMOJI_INDUSTRY_MAP:
            return EMOJI_INDUSTRY_MAP[first]

    if category:
        for keyword, code in FALLBACK_CATEGORY_KEYWORDS:
            if keyword in category:
                return code

    if content_store_line:
        first = content_store_line.strip()[0]
        if first in EMOJI_INDUSTRY_MAP:
            return EMOJI_INDUSTRY_MAP[first]

    return None


def sentiment_score(text: str) -> int:
    lowered = text.lower()
    score = 0
    for word in POSITIVE_KEYWORDS:
        if word in text:
            score += 1
    for word in NEGATIVE_KEYWORDS:
        if word in text:
            score -= 1
    score += lowered.count("good")
    score -= lowered.count("bad")
    return score


def clamp_rating(value: float, minimum: float, maximum: float) -> float:
    clamped = max(minimum, min(maximum, value))
    return round(clamped * 2) / 2


def estimate_rating(icon: str, comment: str) -> float:
    if icon == "😈":
        rating_min, rating_max = 0.0, 2.0
    elif icon == "👼":
        rating_min, rating_max = 3.5, 5.0
    else:
        rating_min, rating_max = 2.0, 3.5

    base = (rating_min + rating_max) / 2
    score = sentiment_score(comment)
    adjusted = base + 0.5 * score
    return clamp_rating(adjusted, rating_min, rating_max)


COMMENT_SECTION_HEADERS = {
    "［客層・スタッフ・環境］",
    "[客層・スタッフ・環境]",
}


def clean_comment_text(lines: List[str]) -> str:
    filtered = []
    for line in lines:
        stripped = remove_emojis(line).strip()
        if stripped in COMMENT_SECTION_HEADERS:
            continue
        filtered.append(line)
    text = "\n".join(filtered)
    text = remove_emojis(text)
    return text.strip()


def normalize_earning_label(value: str) -> Optional[int]:
    parsed = average_from_value(value)
    if parsed is None:
        return None
    return int(round(parsed))


def normalize_wait_time(value: str) -> Optional[int]:
    parsed = average_from_value(value)
    if parsed is None:
        return None
    return int(round(parsed))


def ensure_list(value: Optional[List[Any]]) -> List[Any]:
    return value if isinstance(value, list) else []


def ensure_iso(date_value: Optional[str]) -> Optional[str]:
    if not date_value:
        return None
    return parse_iso_date(date_value)


def update_store_stats(store: Dict[str, Any], review_date: Optional[str]) -> None:
    if review_date is None:
        return
    current_created = store.get("createdAt")
    current_updated = store.get("updatedAt")

    if current_created is None or review_date < current_created:
        store["createdAt"] = review_date
    if current_updated is None or review_date > current_updated:
        store["updatedAt"] = review_date
    store["stats"]["lastReviewedAt"] = review_date


def add_media_entries(store: Dict[str, Any], assets: List[Dict[str, Any]]) -> None:
    existing = {item["storedFilename"] for item in store["media"]}
    for asset in assets:
        stored = asset.get("stored_filename") or asset.get("storedFilename")
        local_path = asset.get("local_path") or asset.get("localPath")
        filename = stored or (pathlib.Path(local_path).name if local_path else None)
        if not filename or filename in existing:
            continue
        entry = {
            "_id": wrap_oid(generate_object_id()),
            "storedFilename": filename,
            "contentType": asset.get("content_type") or asset.get("contentType"),
            "caption": remove_emojis(asset.get("content", "")).strip() or None,
            "sourceMessageId": asset.get("message_id") or None,
            "uploadedAt": wrap_date(ensure_iso(asset.get("created_at") or asset.get("createdAt"))),
        }
        store["media"].append({k: v for k, v in entry.items() if v is not None})
        existing.add(filename)


def build_store_identifier(prefecture: Optional[str], store_name: str) -> str:
    base = prefecture or "unknown"
    return f"{slugify(base)}-{slugify(store_name)}"


def convert_survey_document(
    store_registry: Dict[str, Dict[str, Any]],
    reviews: List[Dict[str, Any]],
    document: Dict[str, Any],
) -> None:
    channel = document.get("channel", "")
    category = document.get("category", "")
    guild_name = document.get("guild", "")

    store_keys_in_document = set()

    for survey in ensure_list(document.get("surveys")):
        content = survey.get("content", "")
        if not content.strip():
            continue

        icon, stripped = extract_first_line_icon(content)
        lines = split_lines(stripped)
        fields, comment_lines = extract_fields(lines)

        store_line = fields.get("store", "")
        prefecture, remainder = detect_prefecture(store_line)
        industry_code = (
            detect_industry_code(channel, category, store_line)
            or detect_industry_code("", guild_name, store_line)
        )

        store_name, branch_name = split_store_and_branch(remainder or channel)
        if not store_name:
            store_name = remove_emojis(channel).strip() or "名称不明"

        store_key = build_store_identifier(prefecture, store_name)

        if store_key not in store_registry:
            store_registry[store_key] = {
                "_id": wrap_oid(generate_object_id()),
                "name": store_name,
                "branchName": branch_name,
                "prefecture": prefecture,
                "industryCodes": set(),
                "keywords": [],
                "contact": {},
                "media": [],
                "stats": {
                    "reviewCount": 0,
                    "avgRating": None,
                    "avgEarning": None,
                    "avgWaitTime": None,
                    "lastReviewedAt": None,
                },
                "createdAt": None,
                "updatedAt": None,
            }
        store_entry = store_registry[store_key]

        if branch_name and not store_entry.get("branchName"):
            store_entry["branchName"] = branch_name
        if prefecture and not store_entry.get("prefecture"):
            store_entry["prefecture"] = prefecture
        if industry_code:
            store_entry["industryCodes"].add(industry_code)

        attachments = ensure_list(survey.get("attachments"))
        review_attachments = []
        for att in attachments:
            local_path = att.get("local_path") or att.get("localPath")
            stored_filename = att.get("stored_filename") or att.get("storedFilename")
            filename = stored_filename or (pathlib.Path(local_path).name if local_path else None)
            if not filename:
                continue
            review_attachments.append(
                {
                    "storedFilename": filename,
                    "contentType": att.get("content_type") or att.get("contentType"),
                }
            )

        rating = estimate_rating(icon, "\n".join(comment_lines))
        period_raw = fields.get("period") or ""
        age = parse_int_field(fields.get("age", ""))
        spec = parse_int_field(fields.get("spec", ""))
        wait_hours = normalize_wait_time(fields.get("wait", ""))
        earning = normalize_earning_label(fields.get("earning", ""))
        period_parsed = None
        if period_raw:
            period_parsed = normalize_period(period_raw)

        created_at_iso = ensure_iso(survey.get("created_at") or survey.get("createdAt"))

        assigned_industry = industry_code
        if assigned_industry is None and store_entry["industryCodes"]:
            assigned_industry = next(iter(store_entry["industryCodes"]))

        review_document = {
            "_id": wrap_oid(generate_object_id()),
            "storeId": store_entry["_id"],
            "industryCode": assigned_industry or "",
            "status": "pending",
            "period": period_parsed,
            "age": age,
            "specScore": spec,
            "waitTimeHours": wait_hours,
            "averageEarning": earning,
            "rating": rating,
            "comment": clean_comment_text(comment_lines),
            "attachments": review_attachments,
            "reward": {"status": "pending", "sentAt": None, "note": ""},
            "createdAt": wrap_date(created_at_iso),
            "updatedAt": wrap_date(created_at_iso),
        }

        reviews.append(review_document)
        update_store_stats(store_entry, created_at_iso)
        store_entry["stats"]["reviewCount"] += 1
        store_keys_in_document.add(store_key)

    asset_entries = ensure_list(document.get("assets"))
    if asset_entries and store_keys_in_document:
        for key in store_keys_in_document:
            add_media_entries(store_registry[key], asset_entries)


def normalize_period(value: str) -> Optional[str]:
    value = value.strip()
    if not value:
        return None
    value = value.replace("年", "-").replace("月", "")
    value = value.replace("〜", "~").replace("～", "~").replace("~", "-")
    numbers = extract_numbers(value)
    if len(numbers) >= 2:
        year = int(numbers[0])
        month = int(numbers[1])
        month = 1 if month < 1 else 12 if month > 12 else month
        return f"{year:04d}-{month:02d}"
    if len(numbers) == 1:
        now = datetime.now()
        month = int(numbers[0])
        month = 1 if month < 1 else 12 if month > 12 else month
        return f"{now.year:04d}-{month:02d}"
    if re.match(r"^\d{4}-\d{2}$", value):
        return value
    return None


def finalize_store_entry(store: Dict[str, Any]) -> Dict[str, Any]:
    industry_codes = sorted(store.pop("industryCodes"))
    store["industryCodes"] = industry_codes
    store["createdAt"] = wrap_date(store["createdAt"])
    store["updatedAt"] = wrap_date(store["updatedAt"])
    last_review = store["stats"].get("lastReviewedAt")
    store["stats"]["lastReviewedAt"] = wrap_date(last_review) if last_review else None
    if store.get("branchName") is None:
        store.pop("branchName", None)
    if store.get("prefecture") is None:
        store.pop("prefecture", None)
    return store


def output_json(path: pathlib.Path, data: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def collect_documents(data_root: pathlib.Path) -> List[pathlib.Path]:
    return sorted(data_root.rglob("*.json"))


def convert(data_root: pathlib.Path, output_dir: pathlib.Path) -> None:
    store_registry: Dict[str, Dict[str, Any]] = {}
    reviews: List[Dict[str, Any]] = []

    for json_file in collect_documents(data_root):
        with json_file.open(encoding="utf-8") as fp:
            try:
                document = json.load(fp)
            except json.JSONDecodeError:
                continue
        convert_survey_document(store_registry, reviews, document)

    stores_output = [finalize_store_entry(store) for store in store_registry.values()]
    reviews_output = reviews

    output_json(output_dir / "stores.json", stores_output)
    output_json(output_dir / "reviews.json", reviews_output)


def main() -> None:
    parser = argparse.ArgumentParser(description="Convert collected surveys into MongoDB-ready documents.")
    parser.add_argument(
        "--data-root",
        default="scripts/discord-survey-collector/data/stores",
        help="Path to the directory that contains store JSON files.",
    )
    parser.add_argument(
        "--output",
        default="scripts/discord-survey-collector/_data/mongo",
        help="Directory to write converted JSON files.",
    )
    args = parser.parse_args()

    data_root = pathlib.Path(args.data_root).resolve()
    output_dir = pathlib.Path(args.output).resolve()

    if not data_root.exists():
        raise FileNotFoundError(f"Data root not found: {data_root}")

    convert(data_root, output_dir)


if __name__ == "__main__":
    main()
