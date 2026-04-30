package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	pb "github.com/azzykesuma/warframeMarket/api/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type restServer struct {
	watcher *watcherServer
	mux     *http.ServeMux
}

func newRESTServer(watcher *watcherServer) http.Handler {
	s := &restServer{
		watcher: watcher,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s.withCORS(s.mux)
}

func (s *restServer) routes() {
	s.mux.HandleFunc("/api/login", s.login)
	s.mux.HandleFunc("/api/logout", s.protected(s.logout))
	s.mux.HandleFunc("/api/items", s.protected(s.listItems))
	s.mux.HandleFunc("/api/items/id/", s.protected(s.getItemByID))
	s.mux.HandleFunc("/api/items/slug/", s.protected(s.getItemBySlug))
	s.mux.HandleFunc("/api/items/", s.protected(s.itemSubroutes))
	s.mux.HandleFunc("/api/orders/my", s.protected(s.getMyOrders))
	s.mux.HandleFunc("/api/orders/", s.protected(s.orderByID))
	s.mux.HandleFunc("/api/orders", s.protected(s.createOrder))
	s.mux.HandleFunc("/api/transactions/recent", s.protected(s.getRecentTransactions))
	s.mux.HandleFunc("/api/inventory/bulk", s.protected(s.inventoryBulkUpload))
	s.mux.HandleFunc("/api/inventory/", s.protected(s.inventoryByID))
	s.mux.HandleFunc("/api/inventory", s.protected(s.inventoryCollection))
}

func (s *restServer) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *restServer) protected(next func(http.ResponseWriter, *http.Request, context.Context)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := metadata.NewIncomingContext(r.Context(), metadata.Pairs("authorization", r.Header.Get("Authorization")))
		if err := s.watcher.authorizeContext(ctx); err != nil {
			writeError(w, err)
			return
		}
		next(w, r, ctx)
	}
}

func (s *restServer) login(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req pb.LoginRequest
	if !decodeProtoBody(w, r, &req) {
		return
	}
	res, err := s.watcher.Login(r.Context(), &req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"success":       res.GetSuccess(),
		"message":       res.GetMessage(),
		"session_token": res.GetSessionToken(),
		"access_token":  res.GetSessionToken(),
		"accessToken":   res.GetSessionToken(),
		"token":         res.GetSessionToken(),
	})
}

func (s *restServer) logout(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	res, err := s.watcher.Logout(ctx, &pb.Empty{})
	writeProto(w, res, err)
}

func (s *restServer) listItems(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	res, err := s.watcher.ListItems(ctx, &pb.ListItemsRequest{Language: r.URL.Query().Get("language")})
	writeProto(w, res, err)
}

func (s *restServer) getItemByID(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	itemID := strings.TrimPrefix(r.URL.Path, "/api/items/id/")
	res, err := s.watcher.GetItemById(ctx, &pb.GetItemRequest{ItemId: itemID, Language: r.URL.Query().Get("language")})
	writeProto(w, res, err)
}

func (s *restServer) getItemBySlug(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	itemSlug := strings.TrimPrefix(r.URL.Path, "/api/items/slug/")
	res, err := s.watcher.GetItemBySlug(ctx, &pb.GetItemBySlugRequest{ItemSlug: itemSlug, Language: r.URL.Query().Get("language")})
	writeProto(w, res, err)
}

func (s *restServer) itemSubroutes(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/items/")
	switch {
	case strings.HasSuffix(path, "/orders/top"):
		itemSlug := strings.TrimSuffix(path, "/orders/top")
		rank, _ := strconv.Atoi(r.URL.Query().Get("rank"))
		res, err := s.watcher.GetTopItemOrders(ctx, &pb.TopItemOrdersRequest{ItemSlug: itemSlug, Rank: int32(rank)})
		writeProto(w, res, err)
	case strings.HasSuffix(path, "/orders"):
		itemSlug := strings.TrimSuffix(path, "/orders")
		res, err := s.watcher.GetItemOrders(ctx, &pb.ItemOrdersRequest{ItemSlug: itemSlug})
		writeProto(w, res, err)
	default:
		writeError(w, status.Error(codes.NotFound, "route not found"))
	}
}

func (s *restServer) getMyOrders(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	res, err := s.watcher.GetMyOrders(ctx, &pb.Empty{})
	writeProto(w, res, err)
}

func (s *restServer) createOrder(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req pb.CreateOrderRequest
	if !decodeProtoBody(w, r, &req) {
		return
	}
	res, err := s.watcher.CreateOrder(ctx, &req)
	writeProto(w, res, err)
}

func (s *restServer) orderByID(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	orderID := strings.TrimPrefix(r.URL.Path, "/api/orders/")
	switch r.Method {
	case http.MethodGet:
		res, err := s.watcher.GetOrderDetail(ctx, &pb.OrderId{Id: orderID})
		writeProto(w, res, err)
	case http.MethodPut:
		var req pb.UpdateOrderRequest
		if !decodeProtoBody(w, r, &req) {
			return
		}
		req.Id = orderID
		res, err := s.watcher.UpdateOrder(ctx, &req)
		writeProto(w, res, err)
	case http.MethodDelete:
		res, err := s.watcher.DeleteOrder(ctx, &pb.OrderId{Id: orderID})
		writeProto(w, res, err)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

func (s *restServer) getRecentTransactions(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	res, err := s.watcher.GetRecentTransactions(ctx, &pb.ItemRequest{ItemName: r.URL.Query().Get("item_name")})
	writeProto(w, res, err)
}

func decodeProtoBody(w http.ResponseWriter, r *http.Request, msg proto.Message) bool {
	defer r.Body.Close()
	body, err := ioReadAllLimit(r.Body, 1<<20)
	if err != nil {
		writeError(w, status.Errorf(codes.InvalidArgument, "failed to read request body: %v", err))
		return false
	}
	if err := protojson.Unmarshal(body, msg); err != nil {
		writeError(w, status.Errorf(codes.InvalidArgument, "invalid JSON body: %v", err))
		return false
	}
	return true
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	body, err := ioReadAllLimit(r.Body, 1<<20)
	if err != nil {
		writeError(w, status.Errorf(codes.InvalidArgument, "failed to read request body: %v", err))
		return false
	}
	if err := json.Unmarshal(body, target); err != nil {
		writeError(w, status.Errorf(codes.InvalidArgument, "invalid JSON body: %v", err))
		return false
	}
	return true
}

func ioReadAllLimit(r io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("request body exceeds %d bytes", limit)
	}
	return body, nil
}

func writeProto(w http.ResponseWriter, msg proto.Message, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	payload, err := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: false,
	}.Marshal(msg)
	if err != nil {
		writeError(w, status.Errorf(codes.Internal, "failed to encode response: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	code := status.Code(err)
	httpStatus := httpStatusFromCode(code)
	message := status.Convert(err).Message()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   message,
		"code":    code.String(),
		"success": false,
	})
}

func httpStatusFromCode(code codes.Code) int {
	switch code {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	writeMethodNotAllowed(w, method)
	return false
}

func writeMethodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	w.WriteHeader(http.StatusMethodNotAllowed)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   "method not allowed",
		"success": false,
	})
}
