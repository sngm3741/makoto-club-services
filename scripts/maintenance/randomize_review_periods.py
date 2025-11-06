#!/usr/bin/env python3
"""
レビューの訪問時期(period)を直近3年以内のランダムな年月に置き換えるスクリプト。

特徴:
  - 各レビューの ObjectID をシードにしているため、dry-run と apply で結果が一致します。
  - 長さはデフォルトで最新36か月 (約3年)。
  - --only-empty を指定すると period が空 (None または "") のドキュメントのみ更新対象。

実行例:
  MONGO_URI="mongodb+srv://..." \
  MONGO_DB="makoto-club" \
  python scripts/maintenance/randomize_review_periods.py --dry-run

  # 問題なければ --apply を付けて反映
  python scripts/maintenance/randomize_review_periods.py --apply
"""

from __future__ import annotations

import argparse
import os
import random
import sys
from datetime import datetime, timezone
from typing import Dict

from bson import ObjectId
from pymongo import MongoClient
from pymongo.collection import Collection


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Randomize review period within recent N months.")
    parser.add_argument(
        "--apply",
        action="store_true",
        help="実際に更新を適用します。指定しない場合は dry-run になります。",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="確認のみ実行します（デフォルト動作）。--apply と同時指定はできません。",
    )
    parser.add_argument(
        "--months",
        type=int,
        default=int(os.getenv("RANDOMIZE_MONTHS", 36)),
        help="ランダム範囲の月数 (default: %(default)s)",
    )
    parser.add_argument(
        "--only-empty",
        action="store_true",
        help="period が空のレビューのみ対象にします。",
    )
    parser.add_argument(
        "--review-collection",
        default=os.getenv("REVIEW_COLLECTION", "reviews"),
        help="レビューのコレクション名 (default: %(default)s)",
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


def month_offset(now: datetime, offset: int) -> Dict[str, int]:
    total_months = now.year * 12 + (now.month - 1) - offset
    year = total_months // 12
    month = total_months % 12 + 1
    return {"year": year, "month": month}


def generate_period(doc_id: ObjectId, now: datetime, months_range: int) -> str:
    if months_range <= 0:
        months_range = 1
    rng = random.Random(str(doc_id))
    offset = rng.randint(0, months_range - 1)
    components = month_offset(now, offset)
    return f"{components['year']}年{components['month']:02d}月"


def randomize_periods(
    reviews: Collection,
    months_range: int,
    only_empty: bool,
    apply_changes: bool,
) -> int:
    now = datetime.now(timezone.utc)
    filter_query: Dict[str, object] = {}
    if only_empty:
        filter_query["$or"] = [{"period": {"$exists": False}}, {"period": None}, {"period": ""}]

    cursor = reviews.find(filter_query, {"_id": 1}, batch_size=200)
    updated = 0
    for doc in cursor:
        review_id = doc.get("_id")
        if not isinstance(review_id, ObjectId):
            continue

        new_period = generate_period(review_id, now, months_range)
        updated += 1
        if apply_changes:
            reviews.update_one(
                {"_id": review_id},
                {
                    "$set": {
                        "period": new_period,
                        "updatedAt": now,
                    }
                },
            )
    return updated


def main() -> int:
    args = parse_args()
    if args.apply and args.dry_run:
        print("--apply と --dry-run は同時に指定できません。", file=sys.stderr)
        return 1

    apply_changes = args.apply

    client = MongoClient(args.uri)
    database = client[args.database]
    reviews = database[args.review_collection]

    print(f"== 対象データベース: {args.database}")
    print(f"== レビューコレクション: {args.review_collection}")
    print(f"== モード: {'apply (更新を適用)' if apply_changes else 'dry-run (確認のみ)'}")
    print(f"== 対象範囲: 直近 {args.months} ヶ月")
    print(f"== 対象条件: {'period が空のみ' if args.only_empty else '全レビュー'}")

    updated = randomize_periods(
        reviews=reviews,
        months_range=args.months,
        only_empty=args.only_empty,
        apply_changes=apply_changes,
    )

    print(f"\n更新対象レビュー数: {updated}")
    if not apply_changes:
        print("\n--apply を付けて実行すると更新が反映されます。")

    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        print("\n中断しました。", file=sys.stderr)
        raise SystemExit(1)
