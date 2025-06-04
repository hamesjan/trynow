// Lab 7: Implement a local filesystem video content service

package web

import (
	"os"
	"fmt"
	"path/filepath"
)

// FSVideoContentService implements VideoContentService using the local filesystem.
type FSVideoContentService struct{
	StorageDirectory string
}

// Uncomment the following line to ensure FSVideoContentService implements VideoContentService
var _ VideoContentService = (*FSVideoContentService)(nil)

func (s *FSVideoContentService) Read(videoId string, filename string) ([]byte, error) {
	filePath := filepath.Join(s.StorageDirectory, videoId, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("faiol to read file: %v", err)
	}
	return data, nil
}

func (s *FSVideoContentService) Write(videoId string, filename string, data []byte) error {
	videoDir := filepath.Join(s.StorageDirectory, videoId)
	os.MkdirAll(videoDir, os.ModePerm)
	filePath := filepath.Join(videoDir, filename)
	err := os.WriteFile(filePath, data, os.ModePerm)
	if err != nil {
		return fmt.Errorf("fail to write file: %v", err)
	}
	return nil
}