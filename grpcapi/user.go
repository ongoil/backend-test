// Package grpcapi is the gRPC server: it exposes CreateUser and GetUser
// on top of the usecases.
package grpcapi

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/laddawan/backend-test/model"
	userv1 "github.com/laddawan/backend-test/proto/user/v1"
	"github.com/laddawan/backend-test/usecase"
)

// Server implements userv1.UserServiceServer.
type Server struct {
	userv1.UnimplementedUserServiceServer
	users  *usecase.UserUsecase
	logger *slog.Logger
}

// NewServer creates a gRPC server with logging, panic recovery and JWT
// authentication interceptors, and registers the user service and
// server reflection (for tools such as grpcurl).
func NewServer(users *usecase.UserUsecase, tokens TokenVerifier, logger *slog.Logger) *grpc.Server {
	s := grpc.NewServer(grpc.ChainUnaryInterceptor(
		LoggingInterceptor(logger),
		RecoveryInterceptor(logger),
		AuthInterceptor(tokens),
	))
	userv1.RegisterUserServiceServer(s, &Server{users: users, logger: logger})
	reflection.Register(s)
	return s
}

// CreateUser creates a user.
func (s *Server) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	user, err := s.users.Create(ctx, usecase.CreateUserInput{
		Name:     req.GetName(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &userv1.CreateUserResponse{User: toProto(user)}, nil
}

// GetUser fetches a user by ID.
func (s *Server) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	user, err := s.users.Get(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(ctx, err)
	}
	return &userv1.GetUserResponse{User: toProto(user)}, nil
}

func toProto(u *model.User) *userv1.User {
	return &userv1.User{
		Id:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: timestamppb.New(u.CreatedAt),
	}
}

// toStatus maps domain errors to gRPC status codes. Validation errors carry
// a google.rpc.BadRequest detail listing each invalid field.
func (s *Server) toStatus(ctx context.Context, err error) error {
	var verr *model.ValidationError
	switch {
	case errors.As(err, &verr):
		st := status.New(codes.InvalidArgument, verr.Error())
		fields := make([]string, 0, len(verr.Fields))
		for f := range verr.Fields {
			fields = append(fields, f)
		}
		sort.Strings(fields)
		br := &errdetails.BadRequest{}
		for _, f := range fields {
			br.FieldViolations = append(br.FieldViolations,
				&errdetails.BadRequest_FieldViolation{Field: f, Description: verr.Fields[f]})
		}
		if withDetails, derr := st.WithDetails(br); derr == nil {
			st = withDetails
		}
		return st.Err()
	case errors.Is(err, model.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, model.ErrEmailAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		s.logger.ErrorContext(ctx, "grpc internal error", "error", err)
		return status.Error(codes.Internal, "internal server error")
	}
}

// TokenVerifier validates an access token and returns its user ID
// (implemented by security.JWTManager).
type TokenVerifier interface {
	Verify(token string) (userID string, err error)
}

type contextKey struct{}

// UserIDFromContext returns the authenticated user's ID set by AuthInterceptor.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKey{}).(string)
	return id, ok
}

// LoggingInterceptor logs the method, status code and execution time.
func LoggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		logger.InfoContext(ctx, "grpc request",
			"method", info.FullMethod,
			"code", status.Code(err).String(),
			"duration", time.Since(start).String(),
		)
		return resp, err
	}
}

// RecoveryInterceptor turns panics into codes.Internal.
func RecoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if v := recover(); v != nil {
				logger.ErrorContext(ctx, "grpc panic recovered",
					"method", info.FullMethod, "panic", v, "stack", string(debug.Stack()))
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// AuthInterceptor requires "authorization: Bearer <token>" metadata on every
// call and stores the token's user ID in the context.
func AuthInterceptor(tokens TokenVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		values := md.Get("authorization")
		if len(values) != 1 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization metadata")
		}
		scheme, token, ok := strings.Cut(strings.TrimSpace(values[0]), " ")
		token = strings.TrimSpace(token)
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			return nil, status.Error(codes.Unauthenticated, "authorization must be 'Bearer <token>'")
		}
		userID, err := tokens.Verify(token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}
		return handler(context.WithValue(ctx, contextKey{}, userID), req)
	}
}
