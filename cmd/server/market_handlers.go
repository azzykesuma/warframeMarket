package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	pb "github.com/azzykesuma/warframeMarket/api/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
