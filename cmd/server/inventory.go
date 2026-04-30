package main

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InventoryItem struct {
	ID         string    `json:"id" firestore:"-"`
	ItemID     string    `json:"item_id" firestore:"item_id"`
	ItemSlug   string    `json:"item_slug" firestore:"item_slug"`
	ItemName   string    `json:"item_name" firestore:"item_name"`
	Quantity   int       `json:"quantity" firestore:"quantity"`
	Rank       int       `json:"rank" firestore:"rank"`
	Subtype    string    `json:"subtype" firestore:"subtype"`
	Notes      string    `json:"notes" firestore:"notes"`
	AcquiredAt string    `json:"acquired_at" firestore:"acquired_at"`
	CreatedAt  time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" firestore:"updated_at"`
}

type InventoryBulkRequest struct {
	Replace bool            `json:"replace"`
	Items   []InventoryItem `json:"items"`
}

type InventoryBulkResponse struct {
	Success bool            `json:"success"`
	Count   int             `json:"count"`
	Items   []InventoryItem `json:"items"`
}

type InventoryRepository interface {
	List(ctx context.Context) ([]InventoryItem, error)
	Get(ctx context.Context, id string) (InventoryItem, error)
	Create(ctx context.Context, item InventoryItem) (InventoryItem, error)
	Update(ctx context.Context, id string, item InventoryItem) (InventoryItem, error)
	Delete(ctx context.Context, id string) error
	BulkUpload(ctx context.Context, items []InventoryItem, replace bool) ([]InventoryItem, error)
	Close() error
}

type unavailableInventoryRepository struct {
	err error
}

func (r unavailableInventoryRepository) List(ctx context.Context) ([]InventoryItem, error) {
	return nil, status.Errorf(codes.Unavailable, "inventory database is not available: %v", r.err)
}

func (r unavailableInventoryRepository) Get(ctx context.Context, id string) (InventoryItem, error) {
	return InventoryItem{}, status.Errorf(codes.Unavailable, "inventory database is not available: %v", r.err)
}

func (r unavailableInventoryRepository) Create(ctx context.Context, item InventoryItem) (InventoryItem, error) {
	return InventoryItem{}, status.Errorf(codes.Unavailable, "inventory database is not available: %v", r.err)
}

func (r unavailableInventoryRepository) Update(ctx context.Context, id string, item InventoryItem) (InventoryItem, error) {
	return InventoryItem{}, status.Errorf(codes.Unavailable, "inventory database is not available: %v", r.err)
}

func (r unavailableInventoryRepository) Delete(ctx context.Context, id string) error {
	return status.Errorf(codes.Unavailable, "inventory database is not available: %v", r.err)
}

func (r unavailableInventoryRepository) BulkUpload(ctx context.Context, items []InventoryItem, replace bool) ([]InventoryItem, error) {
	return nil, status.Errorf(codes.Unavailable, "inventory database is not available: %v", r.err)
}

func (r unavailableInventoryRepository) Close() error {
	return nil
}

func validateInventoryItem(item InventoryItem) error {
	if strings.TrimSpace(item.ItemName) == "" && strings.TrimSpace(item.ItemSlug) == "" && strings.TrimSpace(item.ItemID) == "" {
		return status.Error(codes.InvalidArgument, "item_name, item_slug, or item_id is required")
	}
	if item.Quantity < 0 {
		return status.Error(codes.InvalidArgument, "quantity cannot be negative")
	}
	if item.Rank < 0 {
		return status.Error(codes.InvalidArgument, "rank cannot be negative")
	}
	return nil
}
