package grpcapi_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/laddawan/backend-test/grpcapi"
	"github.com/laddawan/backend-test/mock"
	userv1 "github.com/laddawan/backend-test/proto/user/v1"
	"github.com/laddawan/backend-test/usecase"
)

// newClient starts the real gRPC server over an in-memory connection,
// backed by the in-memory repository mock.
func newClient(t *testing.T) (userv1.UserServiceClient, *mock.UserRepository) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := mock.NewUserRepository()
	users := usecase.NewUserUsecase(repo, &mock.PasswordHasher{}, usecase.NewValidator())

	srv := grpcapi.NewServer(users, &mock.TokenManager{}, logger)
	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn), repo
}

func authed(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

func TestCreateAndGetUser(t *testing.T) {
	client, _ := newClient(t)
	ctx := authed("token-u1")

	created, err := client.CreateUser(ctx, &userv1.CreateUserRequest{
		Name: "Alice", Email: "Alice@Example.com", Password: "s3cretpass",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	u := created.GetUser()
	if u.GetId() == "" || u.GetEmail() != "alice@example.com" || u.GetCreatedAt() == nil {
		t.Fatalf("CreateUser() = %v", u)
	}

	got, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: u.GetId()})
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if got.GetUser().GetName() != "Alice" {
		t.Errorf("GetUser() = %v", got.GetUser())
	}
}

func TestErrorCodes(t *testing.T) {
	client, repo := newClient(t)
	ctx := authed("token-u1")
	_, _ = client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "A", Email: "a@example.com", Password: "password1"})

	_, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Errorf("GetUser(missing) code = %v, want NotFound", status.Code(err))
	}

	_, err = client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "A", Email: "a@example.com", Password: "password1"})
	if status.Code(err) != codes.AlreadyExists {
		t.Errorf("duplicate CreateUser code = %v, want AlreadyExists", status.Code(err))
	}

	_, err = client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "", Email: "bad", Password: "x"})
	st := status.Convert(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("invalid CreateUser code = %v, want InvalidArgument", st.Code())
	}
	var violations int
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			violations = len(br.GetFieldViolations())
		}
	}
	if violations != 3 {
		t.Errorf("field violations = %d, want 3 (name, email, password)", violations)
	}

	repo.Err = errors.New("secret database detail")
	_, err = client.GetUser(ctx, &userv1.GetUserRequest{Id: "x"})
	if st := status.Convert(err); st.Code() != codes.Internal || st.Message() != "internal server error" {
		t.Errorf("repo failure = %v %q, want Internal with generic message", st.Code(), st.Message())
	}
}

func TestAuthInterceptor(t *testing.T) {
	client, _ := newClient(t)
	req := &userv1.GetUserRequest{Id: "x"}

	tests := map[string]context.Context{
		"no metadata":   context.Background(),
		"wrong scheme":  metadata.AppendToOutgoingContext(context.Background(), "authorization", "Basic abc"),
		"empty token":   metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "),
		"invalid token": authed("garbage"),
	}
	for name, ctx := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := client.GetUser(ctx, req); status.Code(err) != codes.Unauthenticated {
				t.Errorf("code = %v, want Unauthenticated", status.Code(err))
			}
		})
	}
}
