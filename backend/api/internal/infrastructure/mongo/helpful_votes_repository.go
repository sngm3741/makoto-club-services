package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// HelpfulVoteRepository はアンケートの Helpful 投票状態を MongoDB で管理するリポジトリ。
type HelpfulVoteRepository struct {
	collection *mongo.Collection
}

// NewHelpfulVoteRepository は HelpfulVoteRepository を生成する Factory。
func NewHelpfulVoteRepository(db *mongo.Database, collectionName string) *HelpfulVoteRepository {
	return &HelpfulVoteRepository{collection: db.Collection(collectionName)}
}

// Upsert は投票者単位で「役に立った」状態をトグルし、状態が変化したかどうかを返す。
func (r *HelpfulVoteRepository) Upsert(ctx context.Context, surveyID, voterID primitive.ObjectID, desiredState bool) (bool, error) {
	filter := bson.M{"surveyId": surveyID, "voterId": voterID}

	if desiredState {
		update := bson.M{
			"$setOnInsert": bson.M{
				"createdAt": time.Now().UTC(),
			},
		}
		opts := options.Update().SetUpsert(true)
		result, err := r.collection.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			return false, err
		}
		return result.UpsertedCount > 0, nil
	}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return false, err
	}
	return result.DeletedCount > 0, nil
}
