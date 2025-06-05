// Lab 8: Implement a network video content service (client using consistent hashing)

package web

import (
	"fmt"
	"context"
	"encoding/binary"
	"sync"
	"crypto/sha256"
	"sort"
	"log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "tritontube/internal/proto"
)

type StorageNode struct {
	Address string
	Hash    uint64
}

// NetworkVideoContentService implements VideoContentService using a network of nodes.
type NetworkVideoContentService struct{
	pb.UnimplementedVideoContentAdminServiceServer
	Nodes   []StorageNode
	Clients map[string]pb.StorageServiceClient
	mu      sync.Mutex
}

// Uncomment the following line to ensure NetworkVideoContentService implements VideoContentService
var _ VideoContentService = (*NetworkVideoContentService)(nil)
var _ pb.VideoContentAdminServiceServer = (*NetworkVideoContentService)(nil)

func (s *NetworkVideoContentService) getHashRingNode(key string) StorageNode {
	if len(s.Nodes) == 0 {
		log.Fatalf("No nodes")
	}
	sortedNodes := make([]StorageNode, len(s.Nodes))
	copy(sortedNodes, s.Nodes)
	sort.Slice(sortedNodes, func(i, j int) bool {
		return sortedNodes[i].Hash < sortedNodes[j].Hash
	})
	sum := sha256.Sum256([]byte(key))
	hashVal := binary.BigEndian.Uint64(sum[:8])
	for _, n := range sortedNodes {
		if hashVal <= n.Hash {
			return n
		}
	}
	return sortedNodes[0]
}

func (s *NetworkVideoContentService) Read(videoId string, filename string) ([]byte, error) {
	key := fmt.Sprintf("%s/%s", videoId, filename)
	n := s.getHashRingNode(key)
	client := s.Clients[n.Address]
	resp, err := client.ReadVideo(context.Background(), &pb.ReadRequest{
		VideoId:  videoId,
		Filename: filename,
	})
	if err != nil {
		return nil, fmt.Errorf("nw read err %v", err)
	}
	return resp.Content, nil
}

func (s *NetworkVideoContentService) Write(videoId string, filename string, data []byte) error {
	key := fmt.Sprintf("%s/%s", videoId, filename)
	n := s.getHashRingNode(key)
	client := s.Clients[n.Address]
	_, err := client.WriteVideo(context.Background(), &pb.WriteRequest{
		VideoId:  videoId,
		Filename: filename,
		Content:     data,
	})
	if err != nil {
		return fmt.Errorf("nw write error: %v", err)
	}
	return nil
}

func (s *NetworkVideoContentService) ListNodes(ctx context.Context, req *pb.ListNodesRequest) (*pb.ListNodesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	addresses := make([]string, len(s.Nodes))
	for i, node := range s.Nodes {
		addresses[i] = node.Address
	}
	return &pb.ListNodesResponse{
		Nodes: addresses,
	}, nil
}

func (s *NetworkVideoContentService) AddNode(ctx context.Context, req *pb.AddNodeRequest) (*pb.AddNodeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	addr := req.NodeAddress
	sum := sha256.Sum256([]byte(addr))
	hash := binary.BigEndian.Uint64(sum[:8])
	s.Nodes = append(s.Nodes, StorageNode{Address: addr, Hash: hash})
	sort.Slice(s.Nodes, func(i, j int) bool {
		return s.Nodes[i].Hash < s.Nodes[j].Hash
	})
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("Failed to add: %v", err)
	}
	s.Clients[addr] = pb.NewStorageServiceClient(conn)
	filesMoved := s.addFilesToThisNode()
	return &pb.AddNodeResponse{MigratedFileCount: int32(filesMoved)}, nil
}

func (s *NetworkVideoContentService) addFilesToThisNode() int {
	totalFilesMoved := 0
	for addr, client := range s.Clients {
		isValid := false
		for _, n := range s.Nodes {
			if n.Address == addr {
				isValid = true
				break
			}
		}
		if !isValid {
			continue
		}
		resp, err := client.ListFiles(context.Background(), &pb.ListRequest{})
		if err != nil {
			continue
		}
		for _, file := range resp.FilesList {
			key := fmt.Sprintf("%s/%s", file.VideoId, file.Filename)
			destinationNode := s.getHashRingNode(key)
			// if target node is diff
			if destinationNode.Address != addr {
				readResponse, err := client.ReadVideo(context.Background(), &pb.ReadRequest{
					VideoId:  file.VideoId,
					Filename: file.Filename,
				})
				if err != nil {
					continue
				}
				newOwner := s.Clients[destinationNode.Address]
				_, err = newOwner.WriteVideo(context.Background(), &pb.WriteRequest{
					VideoId:  file.VideoId,
					Filename: file.Filename,
					Content:  readResponse.Content,
				})
				if err != nil {
					continue
				}
				_, err = client.DeleteVideo(context.Background(), &pb.DeleteRequest{
					VideoId:  file.VideoId,
					Filename: file.Filename,
				})
				if err != nil {
					log.Printf("Failed delete %v", err)
				}
				totalFilesMoved += 1
			}
		}
	}
	return totalFilesMoved
}

func (s *NetworkVideoContentService) RemoveNode(ctx context.Context, req *pb.RemoveNodeRequest) (*pb.RemoveNodeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	addr := req.NodeAddress
	var newNodes []StorageNode
	for _, node := range s.Nodes {
		if node.Address != addr {
			newNodes = append(newNodes, node)
		}
	}
	s.Nodes = newNodes
	sort.Slice(s.Nodes, func(i, j int) bool {
		return s.Nodes[i].Hash < s.Nodes[j].Hash
	})
	migratedCount := s.sendFilesOut(addr)

	return &pb.RemoveNodeResponse{MigratedFileCount: int32(migratedCount)}, nil
}

func (s *NetworkVideoContentService) sendFilesOut(addr string) int {
	client := s.Clients[addr]
	resp, err := client.ListFiles(context.Background(), &pb.ListRequest{})
	if err != nil {
		return 0
	}
	totalFilesMoved := 0
	for _, file := range resp.FilesList {
		key := fmt.Sprintf("%s/%s", file.VideoId, file.Filename)
		newNode := s.getHashRingNode(key)
		if newNode.Address == addr {
			continue
		}
		readResponse, err := client.ReadVideo(context.Background(), &pb.ReadRequest{
			VideoId:  file.VideoId,
			Filename: file.Filename,
		})
		if err != nil {
			continue
		}
		newStorageNode := s.Clients[newNode.Address]
		_, err = newStorageNode.WriteVideo(context.Background(), &pb.WriteRequest{
			VideoId:  file.VideoId,
			Filename: file.Filename,
			Content:  readResponse.Content,
		})
		if err != nil {
			continue
		}
		totalFilesMoved += 1
	}
	_, err = client.RemoveAllFiles(context.Background(), &pb.RemoveRequest{})
	if err != nil {
		log.Printf("Delet all fail %v", err)
	}
		
	return totalFilesMoved
}