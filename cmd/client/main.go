package main

import (
	"context"
	"log"
	"os"
	"time"

	pb "github.com/azzykesuma/warframeMarket/api/proto"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	_ = godotenv.Load()

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewWarframeMarketWatcherClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	username := os.Getenv("APP_USERNAME")
	password := os.Getenv("APP_PASSWORD")
	if username == "" || password == "" {
		log.Println("APP_USERNAME and APP_PASSWORD must be set before running the sample client")
		return
	}

	log.Println("Testing Login...")
	login, err := client.Login(ctx, &pb.LoginRequest{Username: username, Password: password})
	if err != nil {
		log.Printf("Login failed: %v", err)
		return
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+login.GetSessionToken())

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

	log.Println("Testing Logout...")
	if _, err := client.Logout(ctx, &pb.Empty{}); err != nil {
		log.Printf("Logout failed: %v", err)
	}
}
