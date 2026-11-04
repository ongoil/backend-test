package database

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/laddawan/backend-test/repository/mongodb"
)

// collection describes a collection's schema validator and indexes.
type collection struct {
	name      string
	validator bson.M
	indexes   []mongo.IndexModel
}

// collections lists every collection the application needs.
var collections = []collection{
	{
		name: mongodb.UserCollection,
		// Mirrors the user document and the usecase validation rules, so the
		// database rejects malformed documents even if written by another client.
		validator: bson.M{"$jsonSchema": bson.M{
			"bsonType": "object",
			"required": bson.A{mongodb.FieldName, mongodb.FieldEmail, mongodb.FieldPasswordHash, mongodb.FieldCreatedAt},
			"properties": bson.M{
				mongodb.FieldName:         bson.M{"bsonType": "string", "minLength": 1, "maxLength": 100},
				mongodb.FieldEmail:        bson.M{"bsonType": "string", "minLength": 3, "maxLength": 254},
				mongodb.FieldPasswordHash: bson.M{"bsonType": "string", "minLength": 1},
				mongodb.FieldCreatedAt:    bson.M{"bsonType": "date"},
			},
		}},
		indexes: []mongo.IndexModel{
			{
				Keys:    bson.D{{Key: mongodb.FieldEmail, Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_email"),
			},
		},
	},
}

// Migrate creates or updates every collection, its schema validator and its
// indexes. It is idempotent and runs on every startup.
func Migrate(ctx context.Context, db *mongo.Database) error {
	for _, c := range collections {
		if err := ensureCollection(ctx, db, c); err != nil {
			return fmt.Errorf("migrate %s: %w", c.name, err)
		}
		if len(c.indexes) > 0 {
			if _, err := db.Collection(c.name).Indexes().CreateMany(ctx, c.indexes); err != nil {
				return fmt.Errorf("migrate %s: create indexes: %w", c.name, err)
			}
		}
	}
	return nil
}

// ensureCollection creates the collection with its validator, or updates
// the validator if the collection already exists.
func ensureCollection(ctx context.Context, db *mongo.Database, c collection) error {
	names, err := db.ListCollectionNames(ctx, bson.D{{Key: "name", Value: c.name}})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}

	if len(names) == 0 {
		if err := db.CreateCollection(ctx, c.name, options.CreateCollection().SetValidator(c.validator)); err != nil {
			return fmt.Errorf("create collection: %w", err)
		}
		return nil
	}

	cmd := bson.D{{Key: "collMod", Value: c.name}, {Key: "validator", Value: c.validator}}
	if err := db.RunCommand(ctx, cmd).Err(); err != nil {
		return fmt.Errorf("update validator: %w", err)
	}
	return nil
}
