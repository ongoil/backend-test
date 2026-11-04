package mongodb

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/laddawan/backend-test/model"
)

func TestDocumentConversion(t *testing.T) {
	u := &model.User{
		ID:           "6ae992073b6a15836779cb0e",
		Name:         "Carol",
		Email:        "carol@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Date(2026, 11, 3, 7, 49, 27, 0, time.UTC),
	}

	d := toDocument(u)
	if d.ID.Hex() != u.ID || d.Name != u.Name || d.Email != u.Email || d.PasswordHash != u.PasswordHash {
		t.Fatalf("toDocument() = %+v", d)
	}
	if back := d.toModel(); *back != *u {
		t.Errorf("toModel() = %+v, want %+v", back, u)
	}
}

func TestToDocument_NewUserHasZeroID(t *testing.T) {
	for _, id := range []string{"", "not-an-object-id"} {
		if d := toDocument(&model.User{ID: id}); !d.ID.IsZero() {
			t.Errorf("toDocument(ID=%q).ID = %v, want zero so MongoDB assigns one", id, d.ID)
		}
	}
}

func TestDocument_BSONFieldNames(t *testing.T) {
	raw, err := bson.Marshal(userDocument{Name: "Carol", Email: "carol@example.com", PasswordHash: "h"})
	if err != nil {
		t.Fatal(err)
	}
	var doc bson.M
	if err := bson.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{FieldName, FieldEmail, FieldPasswordHash, FieldCreatedAt} {
		if _, ok := doc[f]; !ok {
			t.Errorf("bson document %v missing field %q", doc, f)
		}
	}
	if _, ok := doc[FieldID]; ok {
		t.Error("zero ID must be omitted so MongoDB generates one")
	}
}
