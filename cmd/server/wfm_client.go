package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *watcherServer) languageFor(requestLanguage string) string {
	if requestLanguage != "" {
		return requestLanguage
	}
	return s.language
}

func (s *watcherServer) waitForRateLimit() {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()

	if !s.lastRequest.IsZero() {
		if wait := s.minRequestGap - time.Since(s.lastRequest); wait > 0 {
			time.Sleep(wait)
		}
	}
	s.lastRequest = time.Now()
}

func (s *watcherServer) doRequest(ctx context.Context, method, path string, body []byte, language string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+strings.TrimPrefix(path, "/"), bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Language", s.languageFor(language))
	if s.jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.jwtToken)
	}

	s.waitForRateLimit()
	return s.httpClient.Do(req)
}

func (s *watcherServer) decode(ctx context.Context, method, path string, body []byte, language string, target any) (string, error) {
	resp, err := s.doRequest(ctx, method, path, body, language)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to call WFM API: %v", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to read WFM response: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", mapHTTPError(resp.StatusCode, payload)
	}

	var envelope WfmEnvelope[json.RawMessage]
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return "", status.Errorf(codes.Internal, "failed to decode WFM response envelope: %v", err)
	}
	if envelope.Error != nil && *envelope.Error != "" {
		return "", status.Errorf(codes.Internal, "WFM API error: %s", *envelope.Error)
	}
	if target != nil && envelope.Data != nil {
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			return "", status.Errorf(codes.Internal, "failed to decode WFM data: %v", err)
		}
	}
	return envelope.ApiVersion, nil
}

func mapHTTPError(statusCode int, payload []byte) error {
	message := strings.TrimSpace(string(payload))
	if message == "" {
		message = http.StatusText(statusCode)
	}
	switch statusCode {
	case http.StatusUnauthorized:
		return status.Error(codes.Unauthenticated, message)
	case http.StatusForbidden:
		return status.Error(codes.PermissionDenied, message)
	case http.StatusNotFound:
		return status.Error(codes.NotFound, message)
	case http.StatusTooManyRequests:
		return status.Error(codes.ResourceExhausted, message)
	case http.StatusBadRequest:
		return status.Error(codes.InvalidArgument, message)
	default:
		return status.Errorf(codes.Internal, "WFM API returned status %d: %s", statusCode, message)
	}
}

func (s *watcherServer) requireAuth() error {
	if s.jwtToken == "" {
		return status.Error(codes.Unauthenticated, "JWT_TOKEN is required for this endpoint")
	}
	return nil
}
