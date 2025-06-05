// Lab 8: Implement a network video content service (server)

package storage

// Implement a network video content service (server)
import (
	"fmt"
	"os"
	"context"
	"io"
	"path/filepath"
	pb "tritontube/internal/proto"
)

type StorageService struct {
	pb.UnimplementedStorageServiceServer
	StorageDirectory string
}

func NewStorageService(directoryPath string) *StorageService {
	return &StorageService{StorageDirectory: directoryPath}
}

func (s *StorageService) WriteVideo(ctx context.Context, req *pb.WriteRequest) (*pb.WriteResponse, error) {
	videoDir := filepath.Join(s.StorageDirectory, req.VideoId)
	if err := os.MkdirAll(videoDir, os.ModePerm); err != nil {
		fmt.Printf( "error: %v" , err)
		return &pb.WriteResponse{Status: fmt.Sprintf("mkdir fail: %v", err)}, err
	}
	fullPath := filepath.Join(videoDir, req.Filename)
	file, err := os.Create(fullPath)
	if err != nil {
		fmt.Printf( "Create error %v" , err)
		return &pb.WriteResponse{Status: fmt.Sprintf("create file error: %v", err)}, err
	}
	defer file.Close()
	_, err = file.Write(req.Content)
	if err != nil {
		fmt.Printf( "error: %v" , err)
		return &pb.WriteResponse{Status: fmt.Sprintf("write error: %v", err)}, err
	}
	return &pb.WriteResponse{Status: "ok"}, nil
}

func (s *StorageService) ReadVideo(ctx context.Context, req *pb.ReadRequest) (*pb.ReadResponse, error) {
	fullPath := filepath.Join(s.StorageDirectory, req.VideoId, req.Filename)
	file, err := os.Open(fullPath)
	if err != nil {
		fmt.Printf( "Writeerror: %v" , err)
		return &pb.ReadResponse{Status: fmt.Sprintf("open error: %v", err)}, err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		fmt.Printf( "Writeerror: %v" , err)
		return &pb.ReadResponse{Status: fmt.Sprintf("read error: %v", err)}, err
	}
	return &pb.ReadResponse{
		Status:  "ok",
		Content: content,
	}, nil
}


func (s *StorageService) ListFiles(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	var files []*pb.File
	storageFolder, err := os.ReadDir(s.StorageDirectory)
	if err != nil {
		return nil, fmt.Errorf("Read error: %v", err)
	}
	for _, d := range storageFolder {
		if !d.IsDir() {
			continue
		}
		videoId := d.Name()
		videoPath := filepath.Join(s.StorageDirectory, videoId)
		fileEntries, err := os.ReadDir(videoPath)
		if err != nil {
			continue
		}
		for _, f := range fileEntries {
			files = append(files, &pb.File{
				VideoId:  videoId,
				Filename: f.Name(),
			})
		}
	}
	return &pb.ListResponse{FilesList: files}, nil
}

func (s *StorageService) RemoveAllFiles(ctx context.Context, req *pb.RemoveRequest) (*pb.RemoveResponse, error) {
	err := os.RemoveAll(s.StorageDirectory)
	if err != nil {
		return &pb.RemoveResponse{Status: fmt.Sprintf("delete %v", err)}, err
	}
	err = os.MkdirAll(s.StorageDirectory, os.ModePerm)
	if err != nil {
		return &pb.RemoveResponse{Status: fmt.Sprintf("leave dir %v", err)}, err
	}
	return &pb.RemoveResponse{Status: "ok"}, nil
}

func (s *StorageService) DeleteVideo(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	fullPath := filepath.Join(s.StorageDirectory, req.VideoId, req.Filename)
	err := os.Remove(fullPath)
	if err != nil {
		return &pb.DeleteResponse{Status: fmt.Sprintf("delete error: %v", err)}, err
	}
	return &pb.DeleteResponse{Status: "ok"}, nil
}