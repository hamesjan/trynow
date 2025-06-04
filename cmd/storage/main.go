package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	pb "tritontube/internal/proto"
	"tritontube/internal/storage"  
)


func main() {
	host := flag.String("host", "localhost", "Host address for the server")
	port := flag.Int("port", 8090, "Port number for the server")
	flag.Parse()

	// Validate arguments
	if *port <= 0 {
		panic("Error: Port number must be positive")
	}

	if flag.NArg() < 1 {
		fmt.Println("Usage: storage [OPTIONS] <baseDir>")
		fmt.Println("Error: Base directory argument is required")
		return
	}
	baseDir := flag.Arg(0)

	fmt.Println("Starting storage server...")
	fmt.Printf("Host: %s\n", *host)
	fmt.Printf("Port: %d\n", *port)
	fmt.Printf("Base Directory: %s\n", baseDir)

	err := os.MkdirAll(baseDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Directory Make Fail %v", err)
	}
	addr := fmt.Sprintf("%s:%d", *host, *port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed listen %s, %v", addr, err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterStorageServiceServer(grpcServer, storage.NewStorageService(baseDir))
	log.Printf("Storage server running at %s (dir: %s)", addr, baseDir)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
