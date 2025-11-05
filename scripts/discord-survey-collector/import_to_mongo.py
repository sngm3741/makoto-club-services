import os
from pathlib import Path

from bson import json_util
from pymongo import MongoClient, ReplaceOne


def load_json_array(path: Path):
    if not path.exists():
        raise FileNotFoundError(f"JSON file not found: {path}")
    data = json_util.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, list):
        raise ValueError(f"Expected JSON array in {path}")
    return data


def bool_env(name: str, default: bool = True) -> bool:
    value = os.getenv(name)
    if value is None:
        return default
    return value.strip().lower() in {"1", "true", "yes", "on"}


def main():
    mongo_uri = os.getenv("MONGO_URI")
    if not mongo_uri:
        raise RuntimeError("MONGO_URI is not set")

    mongo_db = os.getenv("MONGO_DB", "makoto-club")
    stores_path = Path(os.getenv("STORES_JSON", "_data/mongo/stores.json"))
    reviews_path = Path(os.getenv("REVIEWS_JSON", "_data/mongo/reviews.json"))
    drop_collections = bool_env("MONGO_IMPORT_DROP", True)

    stores = load_json_array(stores_path)
    reviews = load_json_array(reviews_path)

    client = MongoClient(mongo_uri)
    db = client[mongo_db]

    def apply_collection(coll_name: str, documents):
        collection = db[coll_name]
        if drop_collections:
            collection.delete_many({})
            if documents:
                collection.insert_many(documents)
        else:
            operations = [ReplaceOne({"_id": doc["_id"]}, doc, upsert=True) for doc in documents]
            if operations:
                collection.bulk_write(operations, ordered=False)
        return collection.count_documents({})

    stores_count = apply_collection("stores", stores)
    reviews_count = apply_collection("reviews", reviews)

    print(f"Imported stores: {stores_count}")
    print(f"Imported reviews: {reviews_count}")


if __name__ == "__main__":
    main()
