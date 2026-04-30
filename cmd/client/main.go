package main

import (
	"context"
	"log"
	"time"

	pb "github.com/azzykesuma/warframeMarket/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewWarframeMarketWatcherClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Testing ListItems...")
	items, err := client.ListItems(ctx, &pb.ListItemsRequest{Language: "en"})
	if err != nil {
		log.Printf("ListItems failed: %v", err)
		return
	}
	log.Printf("Loaded %d items", len(items.GetItems()))

	itemSlug := "nikana_prime_set"
	if len(items.GetItems()) > 0 && items.GetItems()[0].GetSlug() != "" {
		itemSlug = items.GetItems()[0].GetSlug()
	}

	log.Printf("Testing GetTopItemOrders for %q...", itemSlug)
	topOrders, err := client.GetTopItemOrders(ctx, &pb.TopItemOrdersRequest{ItemSlug: itemSlug})
	if err != nil {
		log.Printf("GetTopItemOrders failed: %v", err)
		return
	}
	log.Printf("Top orders for %s: %d sell, %d buy", itemSlug, len(topOrders.GetSell()), len(topOrders.GetBuy()))
}
