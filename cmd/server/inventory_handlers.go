package main

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *restServer) inventoryCollection(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.inventoryRepo().List(ctx)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, map[string]any{"items": items})
	case http.MethodPost:
		var item InventoryItem
		if !decodeJSONBody(w, r, &item) {
			return
		}
		created, err := s.inventoryRepo().Create(ctx, item)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, created)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *restServer) inventoryBulkUpload(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var req InventoryBulkRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	items, err := s.inventoryRepo().BulkUpload(ctx, req.Items, req.Replace)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, InventoryBulkResponse{
		Success: true,
		Count:   len(items),
		Items:   items,
	})
}

func (s *restServer) inventoryByID(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	id := strings.TrimPrefix(r.URL.Path, "/api/inventory/")
	if strings.TrimSpace(id) == "" {
		writeError(w, status.Error(codes.InvalidArgument, "inventory id is required"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		item, err := s.inventoryRepo().Get(ctx, id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, item)
	case http.MethodPut:
		var item InventoryItem
		if !decodeJSONBody(w, r, &item) {
			return
		}
		updated, err := s.inventoryRepo().Update(ctx, id, item)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, updated)
	case http.MethodDelete:
		if err := s.inventoryRepo().Delete(ctx, id); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, map[string]any{"success": true, "message": "inventory item deleted"})
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

func (s *restServer) inventoryRepo() InventoryRepository {
	return s.watcher.inventory
}
