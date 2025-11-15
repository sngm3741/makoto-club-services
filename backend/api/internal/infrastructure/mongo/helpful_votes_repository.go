package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// HelpfulVoteRepository persists survey helpful votes per voter.
type HelpfulVoteRepository struct {
	collection *mongo.Collection
}

func NewHelpfulVoteRepository(db *mongo.Database, collectionName string) *HelpfulVoteRepository {
	return &HelpfulVoteRepository{collection: db.Collection(collectionName)}
}

// Upsert applies the desired vote state. Returns true if state changed.
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
