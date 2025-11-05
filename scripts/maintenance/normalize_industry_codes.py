#!/usr/bin/env python3
"""
MongoDB 内の業種コードを日本語ラベルで統一するスクリプト。

対象:
  - stores コレクションの industryCodes (配列)
  - reviews コレクションの industryCode (文字列)

実行例:
  MONGO_URI="mongodb+srv://..." \
  MONGO_DB="makoto-club" \
  python scripts/maintenance/normalize_industry_codes.py --dry-run

  # 問題なければ --apply を付けて実際に更新
  python scripts/maintenance/normalize_industry_codes.py --apply
"""

from __future__ import annotations

import argparse
import os
import sys
from typing import Iterable, List

from pymongo import MongoClient
from pymongo.collection import Collection


JAPANESE_LABELS = {
    "deriheru": "デリヘル",
    "delivery_health": "デリヘル",
    "デリヘル": "デリヘル",
    "hoteheru": "ホテヘル",
    "hotel_health": "ホテヘル",
    "ホテヘル": "ホテヘル",
    "hakoheru": "箱ヘル",
    "hako_heru": "箱ヘル",
    "箱ヘル": "箱ヘル",
    "sopu": "ソープ",
    "soap": "ソープ",
    "ソープ": "ソープ",
    "dc": "DC",
    "ＤＣ": "DC",
    "Ｄｃ": "DC",
    "DC": "DC",
    "huesu": "風エス",
    "fuesu": "風エス",
    "風エス": "風エス",
    "menesu": "メンエス",
    "mens_es": "メンエス",
    "メンエス": "メンエス",
}


def normalize_code(value: str | None) -> str:
    if value is None:
        return ""
    trimmed = value.strip()
    if not trimmed:
        return ""
    lower = trimmed.lower()
    if lower in JAPANESE_LABELS:
        return JAPANESE_LABELS[lower]
    if trimmed in JAPANESE_LABELS:
        return JAPANESE_LABELS[trimmed]
    return trimmed


def normalize_codes(values: Iterable[str]) -> List[str]:
    normalized: List[str] = []
    seen = set()
    for value in values:
        label = normalize_code(value)
        if not label:
            continue
        if label in seen:
            continue
        seen.add(label)
        normalized.append(label)
    return normalized


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Normalize industry codes to Japanese labels.")
    parser.add_argument(
        "--apply",
        action="store_true",
        help="実際に更新を適用します。指定しない場合は dry-run になります。",
    )
    parser.add_argument(
        "--review-collection",
        default=os.getenv("REVIEW_COLLECTION", os.getenv("SURVEY_COLLECTION", "reviews")),
        help="レビューのコレクション名 (default: %(default)s)",
    )
    parser.add_argument(
        "--store-collection",
        default=os.getenv("STORE_COLLECTION", "stores"),
        help="店舗のコレクション名 (default: %(default)s)",
    )
    parser.add_argument(
        "--database",
        default=os.getenv("MONGO_DB", "makoto-club"),
        help="MongoDB データベース名 (default: %(default)s)",
    )
    parser.add_argument(
        "--uri",
        default=os.getenv("MONGO_URI", "mongodb://localhost:27017"),
        help="MongoDB 接続 URI (default: %(default)s)",
    )
    return parser.parse_args()


def normalize_stores(collection: Collection, apply_changes: bool) -> int:
    updated_count = 0
    for doc in collection.find({}, {"industryCodes": 1}):
        doc_id = doc.get("_id")
        original = doc.get("industryCodes") or []
        normalized = normalize_codes(original)
        if normalized == original:
            continue
        updated_count += 1
        if apply_changes:
            collection.update_one({"_id": doc_id}, {"$set": {"industryCodes": normalized}})
    return updated_count


def normalize_reviews(collection: Collection, apply_changes: bool) -> int:
    updated_count = 0
    for doc in collection.find({}, {"industryCode": 1}):
        doc_id = doc.get("_id")
        original = doc.get("industryCode")
        normalized = normalize_code(original)
        if normalized == original:
            continue
        updated_count += 1
        if apply_changes:
            update = {"industryCode": normalized}
            collection.update_one({"_id": doc_id}, {"$set": update})
    return updated_count


def main() -> int:
    args = parse_args()
    apply_changes = args.apply

    client = MongoClient(args.uri)
    database = client[args.database]
    stores = database[args.store_collection]
    reviews = database[args.review_collection]

    print(f"== 対象データベース: {args.database}")
    print(f"== 店舗コレクション: {args.store_collection}")
    print(f"== レビューコレクション: {args.review_collection}")
    print(f"== モード: {'apply (更新を適用)' if apply_changes else 'dry-run (確認のみ)'}")

    stores_updated = normalize_stores(stores, apply_changes)
    reviews_updated = normalize_reviews(reviews, apply_changes)

    print()
    print(f"店舗ドキュメントの更新対象数: {stores_updated}")
    print(f"レビューの更新対象数      : {reviews_updated}")

    if not apply_changes:
        print("\n--apply を付けて実行すると更新が反映されます。")

    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        print("\n中断しました。", file=sys.stderr)
        raise SystemExit(1)
