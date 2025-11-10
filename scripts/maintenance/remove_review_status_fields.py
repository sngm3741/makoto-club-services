#!/usr/bin/env python3
"""
レビューコレクションから廃止済みのステータス・報酬・レビュワー関連フィールドを一括削除するスクリプト。

対象フィールド:
  - status
  - statusNote
  - reviewedBy
  - reviewedAt
  - reward (サブドキュメントごと削除)
  - reviewerId
  - reviewerName
  - reviewerUsername

実行例:
  # 影響範囲を確認 (dry-run)
  MONGO_URI="mongodb+srv://..." python scripts/maintenance/remove_review_status_fields.py

  # 実際に反映 (--apply)
  MONGO_URI="..." python scripts/maintenance/remove_review_status_fields.py --apply
"""

from __future__ import annotations

import argparse
import os
import sys
from typing import Dict, List

from pymongo import MongoClient
from pymongo.collection import Collection


LEGACY_FIELDS: List[str] = [
    "status",
    "statusNote",
    "reviewedBy",
    "reviewedAt",
    "reward",
    "reviewerId",
    "reviewerName",
    "reviewerUsername",
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Remove deprecated moderation fields from review documents.",
    )
    parser.add_argument(
        "--apply",
        action="store_true",
        help="実際に更新を適用します。指定しない場合は dry-run になります。",
    )
    parser.add_argument(
        "--uri",
        default=os.getenv("MONGO_URI", "mongodb://localhost:27017"),
        help="MongoDB 接続 URI (default: %(default)s)",
    )
    parser.add_argument(
        "--database",
        default=os.getenv("MONGO_DB", "makoto-club"),
        help="データベース名 (default: %(default)s)",
    )
    parser.add_argument(
        "--collection",
        default=os.getenv("REVIEW_COLLECTION", "reviews"),
        help="レビューコレクション名 (default: %(default)s)",
    )
    return parser.parse_args()


def build_filter() -> Dict[str, Dict[str, bool]]:
    return {
        "$or": [{field: {"$exists": True}} for field in LEGACY_FIELDS],
    }


def cleanup_reviews(collection: Collection, apply_changes: bool) -> int:
    query = build_filter()
    candidates = list(collection.find(query, {"_id": 1, **{field: 1 for field in LEGACY_FIELDS}}))

    if not candidates:
        print("クリーンアップ対象のフィールドは見つかりませんでした。")
        return 0

    print(f"対象レビュー数: {len(candidates)}")
    if not apply_changes:
        for doc in candidates[:10]:
            doc_id = doc.get("_id")
            present_fields = [field for field in LEGACY_FIELDS if field in doc]
            print(f"  - {doc_id}: {', '.join(present_fields)}")
        print("dry-run のため更新は行っていません。--apply を付けて実行してください。")
        return 0

    unset_spec = {field: "" for field in LEGACY_FIELDS}
    result = collection.update_many(query, {"$unset": unset_spec})
    print(f"更新済みレビュー数: {result.modified_count}")
    return result.modified_count


def main() -> int:
    args = parse_args()
    client = MongoClient(args.uri)
    collection = client[args.database][args.collection]

    print(f"== MongoDB: {args.uri}")
    print(f"== Database: {args.database}")
    print(f"== Collection: {args.collection}")
    print(f"== Mode: {'apply' if args.apply else 'dry-run'}")

    try:
        cleanup_reviews(collection, args.apply)
    finally:
        client.close()
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        print("\n中断しました。", file=sys.stderr)
        raise SystemExit(1)
