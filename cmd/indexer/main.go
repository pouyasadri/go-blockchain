package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pouyasadri/go-blockchain/api/proto"
	"github.com/pouyasadri/go-blockchain/internal/dashboard"
	"github.com/pouyasadri/go-blockchain/internal/indexer"
	"github.com/pouyasadri/go-blockchain/internal/marketplace"
	"github.com/pouyasadri/go-blockchain/internal/network"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	nodeAddr := flag.String("node-address", "localhost:50051", "Core node gRPC endpoint")
	httpPort := flag.String("http-port", ":8080", "HTTP Dashboard web server port")
	flag.Parse()

	log.Printf("Starting Indexer, Marketplace & Web Dashboard... Connecting to %s", *nodeAddr)

	conn, err := grpc.NewClient(*nodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to node at %s: %v", *nodeAddr, err)
	}
	defer func() { _ = conn.Close() }()

	client := proto.NewNodeServiceClient(conn)

	store := indexer.NewIndexStore()
	idx := indexer.NewIndexer(store)
	_ = marketplace.NewServiceCatalog(store)
	_ = marketplace.NewEscrowManager(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Launch embedded dashboard server
	dashSrv, err := dashboard.NewServer(store, nil, *httpPort)
	if err != nil {
		log.Fatalf("Failed to initialize dashboard server: %v", err)
	}
	go func() {
		if err := dashSrv.Start(ctx); err != nil {
			log.Printf("Dashboard server exited: %v", err)
		}
	}()

	// Shutdown handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down indexer & dashboard...")
		cancel()
	}()

	// Start block streaming in background
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				stream, err := client.StreamNewBlocks(ctx, &proto.StreamBlocksRequest{StartHeight: 1})
				if err != nil {
					log.Printf("Failed to subscribe to block stream: %v. Retrying in 3s...", err)
					time.Sleep(3 * time.Second)
					continue
				}

				log.Println("Successfully subscribed to real-time node block stream")
				for {
					event, err := stream.Recv()
					if err != nil {
						log.Printf("Block stream disconnected: %v. Reconnecting...", err)
						break
					}

					coreBlock, err := network.ProtoBlockToCore(event.Block)
					if err == nil {
						_ = idx.ProcessBlock(coreBlock)
						dashSrv.NotifyBlockProcessed()
					}
					log.Printf("Indexed block at height %d (hash: %x)", event.Block.Height, event.Block.Hash)
				}
			}
		}
	}()

	fmt.Printf("Indexer & Dashboard active at http://localhost%s (Press Ctrl+C to stop)\n", *httpPort)
	<-ctx.Done()
	log.Println("Indexer service exited cleanly")
}
