package main

import (
	"flag"
	"fmt"
	"net"
	"database/sql"
	"log"
	"sort"
	
	"crypto/sha256"
	"encoding/binary"
	
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"


	"os"
	"tritontube/internal/web"
	pb "tritontube/internal/proto"
	"strings"
)

func hashStringToUint64(s string) uint64 {
	sum := sha256.Sum256([]byte(s))
	return binary.BigEndian.Uint64(sum[:8])
}

type StorageNode struct {
	Address string
	Hash    uint64
}

// printUsage prints the usage information for the application
func printUsage() {
	fmt.Println("Usage: ./program [OPTIONS] METADATA_TYPE METADATA_OPTIONS CONTENT_TYPE CONTENT_OPTIONS")
	fmt.Println()
	fmt.Println("Arguments:")
	fmt.Println("  METADATA_TYPE         Metadata service type (sqlite, etcd)")
	fmt.Println("  METADATA_OPTIONS      Options for metadata service (e.g., db path)")
	fmt.Println("  CONTENT_TYPE          Content service type (fs, nw)")
	fmt.Println("  CONTENT_OPTIONS       Options for content service (e.g., base dir, network addresses)")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Example: ./program sqlite db.db fs /path/to/videos")
}


func main() {
	// Define flags
	port := flag.Int("port", 8080, "Port number for the web server")
	host := flag.String("host", "localhost", "Host address for the web server")

	// Set custom usage message
	flag.Usage = printUsage

	// Parse flags
	flag.Parse()

	// Check if the correct number of positional arguments is provided
	if len(flag.Args()) != 4 {
		fmt.Println("Error: Incorrect number of arguments")
		printUsage()
		return
	}

	// Parse positional arguments
	metadataServiceType := flag.Arg(0)
	metadataServiceOptions := flag.Arg(1)
	contentServiceType := flag.Arg(2)
	contentServiceOptions := flag.Arg(3)

	// Validate port number (already an int from flag, check if positive)
	if *port <= 0 {
		fmt.Println("Error: Invalid port number:", *port)
		printUsage()
		return
	}

	// Construct metadata service
	var metadataService web.VideoMetadataService
	fmt.Println("Creating metadata service of type", metadataServiceType, "with options", metadataServiceOptions)
	// TODO: Implement metadata service creation logic
	// metadataService = &web.SQLiteVideoMetadataService{}
	db, err := sql.Open("sqlite3", metadataServiceOptions)
	if err != nil {
		log.Fatalf("Failed db: %v", err)
	}
	_, err = db.Exec(` CREATE TABLE IF NOT EXISTS videos (videoId TEXT PRIMARY KEY, uploadedTime TIMESTAMP);`)
	if err != nil {
		log.Fatalf("Err: %v", err)
		return
	}
	metadataService = &web.SQLiteVideoMetadataService{DB: db}

	// Construct content service
	var contentService web.VideoContentService
	if contentServiceType == "fs"{
		fmt.Println("Creating content service of type", contentServiceType, "with options", contentServiceOptions)
		// TODO: Implement content service creation logic
		err = os.MkdirAll(contentServiceOptions, os.ModePerm)
		if err != nil {
			log.Fatalf("Directory Fail %v", err)
		}
		contentService = &web.FSVideoContentService{StorageDirectory: contentServiceOptions}
	} else {
		serverNames := strings.Split(contentServiceOptions, ",")
		adminAddr := serverNames[0]
		storageAddrs := serverNames[1:]
		var nodes []web.StorageNode
		clients := make(map[string]pb.StorageServiceClient)
		for _, addr := range storageAddrs {
			h := hashStringToUint64(addr)
			nodes = append(nodes, web.StorageNode{Address: addr, Hash: h})
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				panic(fmt.Sprintf("Failed to connect to %s: %v", addr, err))
			}
			clients[addr] = pb.NewStorageServiceClient(conn)
		}
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Hash < nodes[j].Hash })
		contentService = &web.NetworkVideoContentService{Nodes: nodes, Clients: clients,}
		go func() {
			adminLis, err := net.Listen("tcp", adminAddr)
			if err != nil {
				log.Fatalf("Failed to listen for admin gRPC on %s: %v", adminAddr, err)
			}
			adminServer := grpc.NewServer()
			pb.RegisterVideoContentAdminServiceServer(adminServer, contentService.(*web.NetworkVideoContentService))
			log.Printf("Admin gRPC server running at %s", adminAddr)
			if err := adminServer.Serve(adminLis); err != nil {
				log.Fatalf("Admin gRPC server error: %v", err)
			}
		}()
	}


	// Start the server
	server := web.NewServer(metadataService, contentService)
	listenAddr := fmt.Sprintf("%s:%d", *host, *port)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fmt.Println("Error starting listener:", err)
		return
	}
	defer lis.Close()

	fmt.Println("Starting web server on", listenAddr)
	err = server.Start(lis)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
}
