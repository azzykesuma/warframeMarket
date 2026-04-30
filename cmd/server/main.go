package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	pb "github.com/azzykesuma/warframeMarket/api/proto"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const defaultBaseURL = "https://api.warframe.market/v2/"

// watcherServer implements pb.WarframeMarketWatcherServer.
type watcherServer struct {
	pb.UnimplementedWarframeMarketWatcherServer
	baseURL       string
	jwtToken      string
	language      string
	httpClient    *http.Client
	rateMu        sync.Mutex
	lastRequest   time.Time
	minRequestGap time.Duration
}

func newWatcherServer() *watcherServer {
	_ = godotenv.Load()
	return newWatcherServerFromConfig(
		os.Getenv("BASE_URL"),
		os.Getenv("JWT_TOKEN"),
		os.Getenv("LANGUAGE"),
		&http.Client{Timeout: 10 * time.Second},
	)
}

func newWatcherServerFromConfig(baseURL, jwtToken, language string, client *http.Client) *watcherServer {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	if language == "" {
		language = "en"
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if jwtToken == "" {
		log.Println("WARNING: JWT_TOKEN is not set; authenticated endpoints will fail")
	}

	return &watcherServer{
		baseURL:       baseURL,
		jwtToken:      jwtToken,
		language:      language,
		httpClient:    client,
		minRequestGap: 350 * time.Millisecond,
	}
}

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

func (s *watcherServer) WatchItem(req *pb.ItemRequest, stream pb.WarframeMarketWatcher_WatchItemServer) error {
	return status.Error(codes.Unimplemented, "method WatchItem not implemented")
}

func (s *watcherServer) WatchAllItem(req *pb.AllItemRequest, stream pb.WarframeMarketWatcher_WatchAllItemServer) error {
	return status.Error(codes.Unimplemented, "method WatchAllItem not implemented")
}

func (s *watcherServer) ListItems(ctx context.Context, req *pb.ListItemsRequest) (*pb.ItemShortList, error) {
	var items []WfmItem
	apiVersion, err := s.decode(ctx, http.MethodGet, "items", nil, req.GetLanguage(), &items)
	if err != nil {
		return nil, err
	}

	result := make([]*pb.ItemShort, 0, len(items))
	for _, item := range items {
		result = append(result, mapItemShort(item))
	}
	return &pb.ItemShortList{ApiVersion: apiVersion, Items: result}, nil
}

func (s *watcherServer) GetItemById(ctx context.Context, req *pb.GetItemRequest) (*pb.GetItemResponse, error) {
	if strings.TrimSpace(req.GetItemId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "item_id is required")
	}
	return s.getItem(ctx, req.GetItemId(), req.GetLanguage())
}

func (s *watcherServer) GetItemBySlug(ctx context.Context, req *pb.GetItemBySlugRequest) (*pb.GetItemResponse, error) {
	if strings.TrimSpace(req.GetItemSlug()) == "" {
		return nil, status.Error(codes.InvalidArgument, "item_slug is required")
	}
	return s.getItem(ctx, req.GetItemSlug(), req.GetLanguage())
}

func (s *watcherServer) getItem(ctx context.Context, idOrSlug, language string) (*pb.GetItemResponse, error) {
	var item WfmItem
	apiVersion, err := s.decode(ctx, http.MethodGet, "item/"+url.PathEscape(idOrSlug), nil, language, &item)
	if err != nil {
		return nil, err
	}
	return &pb.GetItemResponse{ApiVersion: apiVersion, Data: mapItemDetail(item)}, nil
}

func (s *watcherServer) GetItemOrders(ctx context.Context, req *pb.ItemOrdersRequest) (*pb.OrderList, error) {
	if strings.TrimSpace(req.GetItemSlug()) == "" {
		return nil, status.Error(codes.InvalidArgument, "item_slug is required")
	}

	var orders []WfmOrderListing
	apiVersion, err := s.decode(ctx, http.MethodGet, "orders/item/"+url.PathEscape(req.GetItemSlug()), nil, "", &orders)
	if err != nil {
		return nil, err
	}
	return &pb.OrderList{ApiVersion: apiVersion, Orders: mapOrders(orders)}, nil
}

func (s *watcherServer) GetTopItemOrders(ctx context.Context, req *pb.TopItemOrdersRequest) (*pb.TopOrdersResponse, error) {
	if strings.TrimSpace(req.GetItemSlug()) == "" {
		return nil, status.Error(codes.InvalidArgument, "item_slug is required")
	}

	path := "orders/item/" + url.PathEscape(req.GetItemSlug()) + "/top"
	if req.GetRank() > 0 {
		params := url.Values{}
		params.Set("rank", fmt.Sprint(req.GetRank()))
		path += "?" + params.Encode()
	}

	var top WfmTopOrdersData
	apiVersion, err := s.decode(ctx, http.MethodGet, path, nil, "", &top)
	if err != nil {
		return nil, err
	}
	return &pb.TopOrdersResponse{
		ApiVersion: apiVersion,
		Sell:       mapOrders(top.Sell),
		Buy:        mapOrders(top.Buy),
	}, nil
}

func (s *watcherServer) GetMyOrders(ctx context.Context, req *pb.Empty) (*pb.OrderList, error) {
	if err := s.requireAuth(); err != nil {
		return nil, err
	}

	var orders []WfmOrderListing
	apiVersion, err := s.decode(ctx, http.MethodGet, "orders/my", nil, "", &orders)
	if err != nil {
		return nil, err
	}
	return &pb.OrderList{ApiVersion: apiVersion, Orders: mapOrders(orders)}, nil
}

func (s *watcherServer) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.ActionResponse, error) {
	if err := s.requireAuth(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetItemId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "item_id is required")
	}
	if req.GetPlatinum() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "platinum must be greater than zero")
	}
	if req.GetQuantity() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "quantity must be greater than zero")
	}

	body, err := json.Marshal(map[string]any{
		"item_id":    req.GetItemId(),
		"order_type": req.GetType(),
		"platinum":   req.GetPlatinum(),
		"quantity":   req.GetQuantity(),
		"visible":    req.GetVisible(),
		"subtype":    req.GetSubtype(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal request body: %v", err)
	}

	if _, err := s.decode(ctx, http.MethodPost, "order", body, "", nil); err != nil {
		return nil, err
	}
	return &pb.ActionResponse{Success: true, Message: "order successfully created"}, nil
}

func (s *watcherServer) UpdateOrder(ctx context.Context, req *pb.UpdateOrderRequest) (*pb.ActionResponse, error) {
	if err := s.requireAuth(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "order id is required")
	}
	if req.GetPlatinum() < 0 || req.GetQuantity() < 0 {
		return nil, status.Error(codes.InvalidArgument, "platinum and quantity cannot be negative")
	}
	if req.GetPlatinum() == 0 && req.GetQuantity() == 0 {
		return nil, status.Error(codes.InvalidArgument, "platinum or quantity must be provided")
	}

	bodyMap := map[string]any{"visible": req.GetVisible()}
	if req.GetPlatinum() > 0 {
		bodyMap["platinum"] = req.GetPlatinum()
	}
	if req.GetQuantity() > 0 {
		bodyMap["quantity"] = req.GetQuantity()
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal request body: %v", err)
	}

	if _, err := s.decode(ctx, http.MethodPut, "order/"+url.PathEscape(req.GetId()), body, "", nil); err != nil {
		return nil, err
	}
	return &pb.ActionResponse{Success: true, Message: "order successfully updated"}, nil
}

func (s *watcherServer) DeleteOrder(ctx context.Context, req *pb.OrderId) (*pb.ActionResponse, error) {
	if err := s.requireAuth(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "order id is required")
	}
	if _, err := s.decode(ctx, http.MethodDelete, "order/"+url.PathEscape(req.GetId()), nil, "", nil); err != nil {
		return nil, err
	}
	return &pb.ActionResponse{Success: true, Message: "order successfully deleted"}, nil
}

func (s *watcherServer) GetRecentTransactions(ctx context.Context, req *pb.ItemRequest) (*pb.TransactionList, error) {
	path := "orders/recent"
	if req.GetItemName() != "" {
		params := url.Values{}
		params.Set("item", req.GetItemName())
		path += "?" + params.Encode()
	}

	var txs []WfmTransaction
	if _, err := s.decode(ctx, http.MethodGet, path, nil, "", &txs); err != nil {
		return nil, err
	}

	result := make([]*pb.Transaction, 0, len(txs))
	for _, tx := range txs {
		result = append(result, &pb.Transaction{
			Id:               tx.Id,
			ItemName:         tx.ItemName,
			Price:            tx.Platinum,
			Quantity:         tx.Quantity,
			OrderType:        tx.OrderType,
			CounterpartyName: tx.UserName,
			CreatedAt:        tx.CreatedAt,
		})
	}
	return &pb.TransactionList{Transactions: result}, nil
}

func (s *watcherServer) GetOrderDetail(ctx context.Context, req *pb.OrderId) (*pb.GetOrderDetailResponse, error) {
	if strings.TrimSpace(req.GetId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "order id is required")
	}

	var listing WfmOrderListing
	apiVersion, err := s.decode(ctx, http.MethodGet, "order/"+url.PathEscape(req.GetId()), nil, "", &listing)
	if err != nil {
		return nil, err
	}
	order := normalizeOrder(listing)
	return &pb.GetOrderDetailResponse{
		ApiVersion: apiVersion,
		Data: &pb.OrderDetail{
			Id:        order.Id,
			Type:      orderType(order),
			Platinum:  order.Platinum,
			Quantity:  order.Quantity,
			Subtype:   order.Subtype,
			Visible:   order.Visible,
			CreatedAt: order.CreatedAt,
			UpdatedAt: order.UpdatedAt,
			ItemId:    order.ItemId,
			ItemSlug:  order.ItemSlug,
			User:      mapUser(listing.User),
		},
	}, nil
}

func mapItemShort(item WfmItem) *pb.ItemShort {
	return &pb.ItemShort{
		Id:       item.Id,
		Slug:     item.Slug,
		GameRef:  item.GameRef,
		Tags:     item.Tags,
		Subtypes: item.Subtypes,
		Vaulted:  item.Vaulted,
		MaxRank:  item.MaxRank,
		Ducats:   item.Ducats,
		I18N:     mapI18n(item.I18n),
	}
}

func mapItemDetail(item WfmItem) *pb.ItemDetail {
	return &pb.ItemDetail{
		Id:         item.Id,
		Slug:       item.Slug,
		GameRef:    item.GameRef,
		Tags:       item.Tags,
		Subtypes:   item.Subtypes,
		Vaulted:    item.Vaulted,
		TradingTax: item.TradingTax,
		Tradable:   item.Tradable,
		MaxRank:    item.MaxRank,
		Ducats:     item.Ducats,
		I18N:       mapI18n(item.I18n),
	}
}

func mapI18n(i18n WfmI18n) *pb.ItemI18N {
	return &pb.ItemI18N{
		En: &pb.ItemLanguageDetail{
			Name:        i18n.En.Name,
			Description: i18n.En.Description,
			Icon:        i18n.En.Icon,
			Thumb:       i18n.En.Thumb,
			SubIcon:     i18n.En.SubIcon,
		},
	}
}

func mapOrders(items []WfmOrderListing) []*pb.Order {
	orders := make([]*pb.Order, 0, len(items))
	for _, item := range items {
		order := normalizeOrder(item)
		orders = append(orders, &pb.Order{
			Id:        order.Id,
			ItemId:    order.ItemId,
			ItemSlug:  order.ItemSlug,
			Price:     order.Platinum,
			Quantity:  order.Quantity,
			OrderType: orderType(order),
			Subtype:   order.Subtype,
			Visible:   order.Visible,
			CreatedAt: order.CreatedAt,
			UpdatedAt: order.UpdatedAt,
			User:      mapUser(item.User),
		})
	}
	return orders
}

func normalizeOrder(listing WfmOrderListing) WfmOrder {
	if listing.Order.Id != "" || listing.Order.Platinum != 0 || listing.Order.Quantity != 0 {
		return listing.Order
	}
	return WfmOrder{
		Id:        listing.Id,
		ItemId:    listing.ItemId,
		ItemSlug:  listing.ItemSlug,
		Type:      listing.Type,
		OrderType: listing.OrderType,
		Platinum:  listing.Platinum,
		Quantity:  listing.Quantity,
		Subtype:   listing.Subtype,
		Visible:   listing.Visible,
		CreatedAt: listing.CreatedAt,
		UpdatedAt: listing.UpdatedAt,
	}
}

func orderType(order WfmOrder) string {
	if order.Type != "" {
		return order.Type
	}
	return order.OrderType
}

func mapUser(user WfmUser) *pb.User {
	return &pb.User{
		Id:         user.Id,
		IngameName: user.IngameName,
		Status:     user.Status,
		Platform:   user.Platform,
		Reputation: user.Reputation,
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterWarframeMarketWatcherServer(s, newWatcherServer())
	reflection.Register(s)

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
