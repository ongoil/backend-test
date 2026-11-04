//go:build integration

// Integration tests against a real MongoDB. Run with:
//
//	MONGO_URI=mongodb://localhost:27017 go test -tags=integration ./repository/mongodb/...
package mongodb_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/laddawan/backend-test/database"
	"github.com/laddawan/backend-test/model"
	"github.com/laddawan/backend-test/repository/mongodb"
)

// newTestDB connects to MONGO_URI, creates a throwaway database and runs
// database.Migrate on it.
func newTestDB(t *testing.T) *mongo.Database {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := database.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	db := client.Database(fmt.Sprintf("backend_test_%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = client.Disconnect(context.Background())
	})

	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return db
}

func TestUserRepository_CRUD(t *testing.T) {
	repo := mongodb.NewUserRepository(newTestDB(t))
	ctx := context.Background()

	u := &model.User{
		Name: "Alice", Email: "alice@example.com", PasswordHash: "hash",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if u.ID == "" {
		t.Fatal("expected ID to be set")
	}
	id := u.ID

	dup := *u
	if err := repo.Create(ctx, &dup); !errors.Is(err, model.ErrEmailAlreadyExists) {
		t.Fatalf("duplicate Create() error = %v, want ErrEmailAlreadyExists", err)
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil || got.Email != u.Email || !got.CreatedAt.Equal(u.CreatedAt) {
		t.Fatalf("GetByID() = %+v, %v", got, err)
	}
	if _, err := repo.GetByEmail(ctx, "alice@example.com"); err != nil {
		t.Fatalf("GetByEmail() error = %v", err)
	}
	if _, err := repo.GetByID(ctx, "not-an-object-id"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("GetByID(bad) error = %v, want ErrNotFound", err)
	}

	got.Name = "Alicia"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	n, err := repo.Count(ctx)
	if err != nil || n != 1 {
		t.Fatalf("Count() = %d, %v; want 1", n, err)
	}
	users, err := repo.List(ctx)
	if err != nil || len(users) != 1 || users[0].Name != "Alicia" {
		t.Fatalf("List() = %+v, %v", users, err)
	}

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := repo.Delete(ctx, id); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("second Delete() error = %v, want ErrNotFound", err)
	}
}

func TestMigrate_IsIdempotent(t *testing.T) {
	db := newTestDB(t) // first run
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
}

func TestMigrate_SchemaRejectsInvalidDocuments(t *testing.T) {
	db := newTestDB(t)
	coll := db.Collection(mongodb.UserCollection)
	ctx := context.Background()

	invalid := map[string]bson.M{
		"missing email": {"name": "A", "password_hash": "h", "created_at": time.Now()},
		"empty name":    {"name": "", "email": "a@b.co", "password_hash": "h", "created_at": time.Now()},
		"wrong type":    {"name": 123, "email": "a@b.co", "password_hash": "h", "created_at": time.Now()},
	}
	for name, doc := range invalid {
		t.Run(name, func(t *testing.T) {
			if _, err := coll.InsertOne(ctx, doc); err == nil {
				t.Error("expected the schema validator to reject the document")
			}
		})
	}

	valid := bson.M{"name": "A", "email": "a@b.co", "password_hash": "h", "created_at": time.Now()}
	if _, err := coll.InsertOne(ctx, valid); err != nil {
		t.Errorf("valid document rejected: %v", err)
	}
}
