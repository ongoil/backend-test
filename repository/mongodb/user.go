// Package mongodb implements the repository interfaces with the official
// MongoDB Go driver. Domain models are mapped to MongoDB documents in model.go.
package mongodb

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/laddawan/backend-test/model"
	"github.com/laddawan/backend-test/repository"
)

// UserRepository stores users in MongoDB. Run database.Migrate first so
// the collection, validator and unique email index exist.
type UserRepository struct {
	coll *mongo.Collection
}

var _ repository.UserRepository = (*UserRepository)(nil)

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{
		coll: db.Collection(UserCollection),
	}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	doc := toDocument(user)
	doc.ID = bson.ObjectID{} // always let MongoDB assign the ID

	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return model.ErrEmailAlreadyExists
		}
		return fmt.Errorf("insert user: %w", err)
	}
	id, ok := res.InsertedID.(bson.ObjectID)
	if !ok {
		return fmt.Errorf("unexpected inserted id type %T", res.InsertedID)
	}
	user.ID = id.Hex()
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, model.ErrNotFound // a malformed ID can never match
	}
	return r.findOne(ctx, bson.D{{Key: FieldID, Value: oid}})
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return r.findOne(ctx, bson.D{{Key: FieldEmail, Value: email}})
}

func (r *UserRepository) findOne(ctx context.Context, filter bson.D) (*model.User, error) {
	var doc userDocument
	err := r.coll.FindOne(ctx, filter).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return doc.toModel(), nil
}

func (r *UserRepository) List(ctx context.Context) ([]*model.User, error) {
	cur, err := r.coll.Find(ctx, bson.D{},
		options.Find().SetSort(bson.D{{Key: FieldCreatedAt, Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("find users: %w", err)
	}
	var docs []userDocument
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}
	users := make([]*model.User, 0, len(docs))
	for _, d := range docs {
		users = append(users, d.toModel())
	}
	return users, nil
}

func (r *UserRepository) Update(ctx context.Context, user *model.User) error {
	oid, err := bson.ObjectIDFromHex(user.ID)
	if err != nil {
		return model.ErrNotFound
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.D{{Key: FieldID, Value: oid}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: FieldName, Value: user.Name},
			{Key: FieldEmail, Value: user.Email},
		}}},
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return model.ErrEmailAlreadyExists
		}
		return fmt.Errorf("update user: %w", err)
	}
	if res.MatchedCount == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return model.ErrNotFound
	}
	res, err := r.coll.DeleteOne(ctx, bson.D{{Key: FieldID, Value: oid}})
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if res.DeletedCount == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	n, err := r.coll.CountDocuments(ctx, bson.D{})
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}
