package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "github.com/azzykesuma/warframeMarket/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestListItemsSendsHeadersAndMapsItems(t *testing.T) {
	var gotAuth string
	var gotLanguage string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		gotLanguage = r.Header.Get("Language")

		_ = json.NewEncoder(w).Encode(WfmEnvelope[[]WfmItem]{
			ApiVersion: "v0.23.0",
			Data: []WfmItem{{
				Id:      "item-1",
				Slug:    "nikana_prime_set",
				GameRef: "/Lotus/Types/Items/NikanaPrime",
				I18n: WfmI18n{En: WfmLangDetail{
					Name: "Nikana Prime Set",
					Icon: "items/images/en/nikana_prime.png",
				}},
			}},
		})
	}))
	defer upstream.Close()

	server := newWatcherServerFromConfig(upstream.URL, "token", "en", upstream.Client())
	server.minRequestGap = 0

	res, err := server.ListItems(context.Background(), &pb.ListItemsRequest{Language: "ko"})
	if err != nil {
		t.Fatalf("ListItems returned error: %v", err)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("Authorization header = %q", gotAuth)
	}
	if gotLanguage != "ko" {
		t.Fatalf("Language header = %q", gotLanguage)
	}
	if res.GetApiVersion() != "v0.23.0" {
		t.Fatalf("ApiVersion = %q", res.GetApiVersion())
	}
	if len(res.GetItems()) != 1 || res.GetItems()[0].GetSlug() != "nikana_prime_set" {
		t.Fatalf("unexpected items: %+v", res.GetItems())
	}
}

func TestAuthenticatedEndpointRequiresToken(t *testing.T) {
	server := newWatcherServerFromConfig("http://example.test", "", "en", nil)
	server.minRequestGap = 0

	_, err := server.GetMyOrders(context.Background(), &pb.Empty{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("status code = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
}

func TestHTTPStatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       codes.Code
	}{
		{name: "unauthorized", statusCode: http.StatusUnauthorized, want: codes.Unauthenticated},
		{name: "forbidden", statusCode: http.StatusForbidden, want: codes.PermissionDenied},
		{name: "not found", statusCode: http.StatusNotFound, want: codes.NotFound},
		{name: "rate limited", statusCode: http.StatusTooManyRequests, want: codes.ResourceExhausted},
		{name: "bad request", statusCode: http.StatusBadRequest, want: codes.InvalidArgument},
		{name: "server error", statusCode: http.StatusInternalServerError, want: codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mapHTTPError(tt.statusCode, []byte("upstream error"))
			if status.Code(err) != tt.want {
				t.Fatalf("status code = %v, want %v", status.Code(err), tt.want)
			}
		})
	}
}

func TestCreateOrderValidation(t *testing.T) {
	server := newWatcherServerFromConfig("http://example.test", "token", "en", nil)
	server.minRequestGap = 0

	_, err := server.CreateOrder(context.Background(), &pb.CreateOrderRequest{ItemId: "item-1", Platinum: 0, Quantity: 1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
}

func TestLoginCreatesSessionToken(t *testing.T) {
	server := newWatcherServerFromConfigWithAuth("http://example.test", "", "en", "aoi umi", "blue_sea_30", nil)

	res, err := server.Login(context.Background(), &pb.LoginRequest{
		Username: "aoi umi",
		Password: "blue_sea_30",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if !res.GetSuccess() || res.GetSessionToken() == "" {
		t.Fatalf("unexpected login response: %+v", res)
	}
	if !server.hasSession(res.GetSessionToken()) {
		t.Fatalf("session token was not stored")
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	server := newWatcherServerFromConfigWithAuth("http://example.test", "", "en", "aoi umi", "blue_sea_30", nil)

	_, err := server.Login(context.Background(), &pb.LoginRequest{
		Username: "aoi umi",
		Password: "wrong",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("status code = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
}

func TestUnaryAuthInterceptorRequiresValidToken(t *testing.T) {
	server := newWatcherServerFromConfigWithAuth("http://example.test", "", "en", "aoi umi", "blue_sea_30", nil)
	info := &grpc.UnaryServerInfo{FullMethod: "/watcher.WarframeMarketWatcher/ListItems"}
	handler := func(ctx context.Context, req any) (any, error) {
		return &pb.ItemShortList{}, nil
	}

	_, err := server.unaryAuthInterceptor(context.Background(), &pb.ListItemsRequest{}, info, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing token code = %v, want %v", status.Code(err), codes.Unauthenticated)
	}

	login, err := server.Login(context.Background(), &pb.LoginRequest{Username: "aoi umi", Password: "blue_sea_30"})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+login.GetSessionToken()))
	if _, err := server.unaryAuthInterceptor(ctx, &pb.ListItemsRequest{}, info, handler); err != nil {
		t.Fatalf("valid token returned error: %v", err)
	}

	if _, err := server.Logout(ctx, &pb.Empty{}); err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	_, err = server.unaryAuthInterceptor(ctx, &pb.ListItemsRequest{}, info, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("logged out token code = %v, want %v", status.Code(err), codes.Unauthenticated)
	}
}

func TestLoginMethodBypassesAuthInterceptor(t *testing.T) {
	server := newWatcherServerFromConfigWithAuth("http://example.test", "", "en", "aoi umi", "blue_sea_30", nil)
	info := &grpc.UnaryServerInfo{FullMethod: "/watcher.WarframeMarketWatcher/Login"}
	handler := func(ctx context.Context, req any) (any, error) {
		return &pb.LoginResponse{Success: true}, nil
	}

	if _, err := server.unaryAuthInterceptor(context.Background(), &pb.LoginRequest{}, info, handler); err != nil {
		t.Fatalf("Login should bypass auth interceptor: %v", err)
	}
}
