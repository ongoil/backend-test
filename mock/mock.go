// Package mock provides hand-written test doubles, so tests can run
// without MongoDB and without slow bcrypt hashing.
package mock

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/laddawan/backend-test/model"
	"github.com/laddawan/backend-test/repository"
)

// UserRepository is an in-memory, concurrency-safe repository.UserRepository.
// Set Err to make every method fail with that error.
type UserRepository struct {
	mu     sync.Mutex
	users  map[string]model.User
	nextID int

	Err error
}

var _ repository.UserRepository = (*UserRepository)(nil)

// NewUserRepository returns an empty in-memory repository.
func NewUserRepository() *UserRepository {
	return &UserRepository{users: make(map[string]model.User)}
}

func (r *UserRepository) Create(_ context.Context, u *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	if r.emailTaken(u.Email, "") {
		return model.ErrEmailAlreadyExists
	}
	r.nextID++
	u.ID = fmt.Sprintf("%024x", r.nextID) // looks like a MongoDB ObjectID
	r.users[u.ID] = *u
	return nil
}

func (r *UserRepository) GetByID(_ context.Context, id string) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	u, ok := r.users[id]
	if !ok {
		return nil, model.ErrNotFound
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(_ context.Context, email string) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	for _, u := range r.users {
		if u.Email == email {
			return &u, nil
		}
	}
	return nil, model.ErrNotFound
}

func (r *UserRepository) List(_ context.Context) ([]*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	out := make([]*model.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, &u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *UserRepository) Update(_ context.Context, u *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	id := u.ID
	existing, ok := r.users[id]
	if !ok {
		return model.ErrNotFound
	}
	if r.emailTaken(u.Email, id) {
		return model.ErrEmailAlreadyExists
	}
	existing.Name = u.Name
	existing.Email = u.Email
	r.users[id] = existing
	return nil
}

func (r *UserRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	if _, ok := r.users[id]; !ok {
		return model.ErrNotFound
	}
	delete(r.users, id)
	return nil
}

func (r *UserRepository) Count(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return 0, r.Err
	}
	return int64(len(r.users)), nil
}

func (r *UserRepository) emailTaken(email, exceptID string) bool {
	for id, u := range r.users {
		if id != exceptID && u.Email == email {
			return true
		}
	}
	return false
}

// PasswordHasher is a fast, insecure password hasher for tests.
type PasswordHasher struct {
	HashErr error
}

const hashPrefix = "hashed:"

func (h *PasswordHasher) Hash(password string) (string, error) {
	if h.HashErr != nil {
		return "", h.HashErr
	}
	return hashPrefix + password, nil
}

func (h *PasswordHasher) Compare(hash, password string) error {
	if hash != hashPrefix+password {
		return model.ErrInvalidCredentials
	}
	return nil
}

// TokenManager is a fake token issuer and verifier.
// Issued tokens have the form "token-<userID>".
type TokenManager struct {
	IssueErr error
}

const tokenPrefix = "token-"

func (m *TokenManager) Issue(userID string) (string, time.Time, error) {
	if m.IssueErr != nil {
		return "", time.Time{}, m.IssueErr
	}
	return tokenPrefix + userID, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), nil
}

func (m *TokenManager) Verify(token string) (string, error) {
	userID, ok := strings.CutPrefix(token, tokenPrefix)
	if !ok || userID == "" {
		return "", model.ErrInvalidToken
	}
	return userID, nil
}
