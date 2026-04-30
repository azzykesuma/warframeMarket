package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"strings"

	pb "github.com/azzykesuma/warframeMarket/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func (s *watcherServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if !constantTimeEqual(req.GetUsername(), s.appUsername) || !constantTimeEqual(req.GetPassword(), s.appPassword) {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}

	token, err := generateSessionToken()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate session token: %v", err)
	}

	s.sessionMu.Lock()
	s.sessions[token] = struct{}{}
	s.sessionMu.Unlock()

	return &pb.LoginResponse{
		Success:      true,
		SessionToken: token,
		Message:      "login successful",
	}, nil
}

func (s *watcherServer) Logout(ctx context.Context, req *pb.Empty) (*pb.ActionResponse, error) {
	token, err := sessionTokenFromContext(ctx)
	if err != nil {
		return nil, err
	}

	s.sessionMu.Lock()
	delete(s.sessions, token)
	s.sessionMu.Unlock()

	return &pb.ActionResponse{Success: true, Message: "logout successful"}, nil
}

func (s *watcherServer) unaryAuthInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if isPublicMethod(info.FullMethod) {
		return handler(ctx, req)
	}
	if err := s.authorizeContext(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func (s *watcherServer) streamAuthInterceptor(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if isPublicMethod(info.FullMethod) {
		return handler(srv, stream)
	}
	if err := s.authorizeContext(stream.Context()); err != nil {
		return err
	}
	return handler(srv, stream)
}

func isPublicMethod(fullMethod string) bool {
	return strings.HasSuffix(fullMethod, "/Login")
}

func (s *watcherServer) authorizeContext(ctx context.Context) error {
	token, err := sessionTokenFromContext(ctx)
	if err != nil {
		return err
	}

	if !s.hasSession(token) {
		return status.Error(codes.Unauthenticated, "invalid or expired session token")
	}
	return nil
}

func (s *watcherServer) hasSession(token string) bool {
	s.sessionMu.RLock()
	_, ok := s.sessions[token]
	s.sessionMu.RUnlock()
	return ok
}

func sessionTokenFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "authorization metadata is required")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "authorization metadata is required")
	}
	authValue := strings.TrimSpace(values[0])
	token, ok := strings.CutPrefix(authValue, "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		return "", status.Error(codes.Unauthenticated, "authorization metadata must use Bearer token")
	}
	return strings.TrimSpace(token), nil
}

func generateSessionToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func constantTimeEqual(a, b string) bool {
	if a == "" || b == "" || len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
