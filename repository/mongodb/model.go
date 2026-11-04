package mongodb

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/laddawan/backend-test/model"
)

// UserCollection is the MongoDB collection that stores users.
const UserCollection = "users"

// Field names of userDocument, used in filters, updates, indexes and the
// schema validator in database/table.go.
const (
	FieldID           = "_id"
	FieldName         = "name"
	FieldEmail        = "email"
	FieldPasswordHash = "password_hash"
	FieldCreatedAt    = "created_at"
)

// userDocument is how a model.User is stored in MongoDB. Keeping the bson
// tags here, instead of on model.User, keeps the domain free of database
// concerns.
type userDocument struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	Name         string        `bson:"name"`
	Email        string        `bson:"email"`
	PasswordHash string        `bson:"password_hash"`
	CreatedAt    time.Time     `bson:"created_at"`
}

// toDocument converts a domain user into a document. An empty or invalid ID
// becomes the zero ObjectID, so MongoDB assigns one on insert.
func toDocument(u *model.User) userDocument {
	d := userDocument{
		Name:         u.Name,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
	}
	if id, err := bson.ObjectIDFromHex(u.ID); err == nil {
		d.ID = id
	}
	return d
}

// toModel converts a document into a domain user.
func (d userDocument) toModel() *model.User {
	return &model.User{
		ID:           d.ID.Hex(),
		Name:         d.Name,
		Email:        d.Email,
		PasswordHash: d.PasswordHash,
		CreatedAt:    d.CreatedAt.UTC(),
	}
}
