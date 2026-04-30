package main

import (
	"context"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultInventoryCollection = "inventory_items"

type firestoreInventoryRepository struct {
	client     *firestore.Client
	collection string
}

func newInventoryRepository(ctx context.Context) (InventoryRepository, error) {
	collection := os.Getenv("FIRESTORE_INVENTORY_COLLECTION")
	if collection == "" {
		collection = defaultInventoryCollection
	}

	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	credentialsFile := os.Getenv("FIREBASE_CREDENTIALS_FILE")

	var opts []option.ClientOption
	if credentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsFile))
	}

	var config *firebase.Config
	if projectID != "" {
		config = &firebase.Config{ProjectID: projectID}
	}

	app, err := firebase.NewApp(ctx, config, opts...)
	if err != nil {
		return nil, err
	}
	client, err := app.Firestore(ctx)
	if err != nil {
		return nil, err
	}

	return &firestoreInventoryRepository{
		client:     client,
		collection: collection,
	}, nil
}

func (r *firestoreInventoryRepository) List(ctx context.Context) ([]InventoryItem, error) {
	iter := r.client.Collection(r.collection).Documents(ctx)
	defer iter.Stop()

	items := []InventoryItem{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		item, err := inventoryFromDoc(doc)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *firestoreInventoryRepository) Get(ctx context.Context, id string) (InventoryItem, error) {
	if strings.TrimSpace(id) == "" {
		return InventoryItem{}, status.Error(codes.InvalidArgument, "inventory id is required")
	}

	doc, err := r.client.Collection(r.collection).Doc(id).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return InventoryItem{}, status.Error(codes.NotFound, "inventory item not found")
	}
	if err != nil {
		return InventoryItem{}, err
	}
	return inventoryFromDoc(doc)
}

func (r *firestoreInventoryRepository) Create(ctx context.Context, item InventoryItem) (InventoryItem, error) {
	if err := validateInventoryItem(item); err != nil {
		return InventoryItem{}, err
	}

	now := time.Now().UTC()
	item.ID = ""
	item.CreatedAt = now
	item.UpdatedAt = now

	doc := r.client.Collection(r.collection).NewDoc()
	if _, err := doc.Set(ctx, item); err != nil {
		return InventoryItem{}, err
	}
	item.ID = doc.ID
	return item, nil
}

func (r *firestoreInventoryRepository) Update(ctx context.Context, id string, item InventoryItem) (InventoryItem, error) {
	if strings.TrimSpace(id) == "" {
		return InventoryItem{}, status.Error(codes.InvalidArgument, "inventory id is required")
	}
	if err := validateInventoryItem(item); err != nil {
		return InventoryItem{}, err
	}

	existing, err := r.Get(ctx, id)
	if err != nil {
		return InventoryItem{}, err
	}

	item.ID = id
	item.CreatedAt = existing.CreatedAt
	item.UpdatedAt = time.Now().UTC()

	if _, err := r.client.Collection(r.collection).Doc(id).Set(ctx, item); err != nil {
		return InventoryItem{}, err
	}
	return item, nil
}

func (r *firestoreInventoryRepository) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return status.Error(codes.InvalidArgument, "inventory id is required")
	}
	if _, err := r.Get(ctx, id); err != nil {
		return err
	}
	_, err := r.client.Collection(r.collection).Doc(id).Delete(ctx)
	return err
}

func (r *firestoreInventoryRepository) BulkUpload(ctx context.Context, items []InventoryItem, replace bool) ([]InventoryItem, error) {
	for _, item := range items {
		if err := validateInventoryItem(item); err != nil {
			return nil, err
		}
	}

	if replace {
		existing, err := r.List(ctx)
		if err != nil {
			return nil, err
		}
		batch := r.client.Batch()
		for _, item := range existing {
			batch.Delete(r.client.Collection(r.collection).Doc(item.ID))
		}
		if _, err := batch.Commit(ctx); err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	batch := r.client.Batch()
	created := make([]InventoryItem, 0, len(items))
	for _, item := range items {
		item.ID = ""
		item.CreatedAt = now
		item.UpdatedAt = now
		doc := r.client.Collection(r.collection).NewDoc()
		batch.Set(doc, item)
		item.ID = doc.ID
		created = append(created, item)
	}
	if _, err := batch.Commit(ctx); err != nil {
		return nil, err
	}
	return created, nil
}

func (r *firestoreInventoryRepository) Close() error {
	return r.client.Close()
}

func inventoryFromDoc(doc *firestore.DocumentSnapshot) (InventoryItem, error) {
	var item InventoryItem
	if err := doc.DataTo(&item); err != nil {
		return InventoryItem{}, err
	}
	item.ID = doc.Ref.ID
	return item, nil
}
