// Lab 7: Implement a SQLite video metadata service

package web

import (
	"time"
	_ "github.com/mattn/go-sqlite3"
	"database/sql"
	"fmt"
	"log"
)

type SQLiteVideoMetadataService struct{
	DB *sql.DB
}

// Uncomment the following line to ensure SQLiteVideoMetadataService implements VideoMetadataService
var _ VideoMetadataService = (*SQLiteVideoMetadataService)(nil)

func (s *SQLiteVideoMetadataService) Create(videoId string, uploadedAt time.Time) error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS videos (
						videoId TEXT PRIMARY KEY,
						uploadedTime TIMESTAMP);`)
	if err != nil {
		log.Println("Error creating table")
		return fmt.Errorf("Failed create table%v", err)
	}
	_, err = s.DB.Exec(`INSERT INTO videos (videoId, uploadedTime) VALUES (?, ?)`, videoId, uploadedAt)
	if err != nil {
		return fmt.Errorf("failed to insert video metadata: %v", err)
	}
	return nil
}

// List retrieves all video metadata records
func (s *SQLiteVideoMetadataService) List() ([]VideoMetadata, error) {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS videos (
						videoId TEXT PRIMARY KEY,
						uploadedTime TIMESTAMP);`)
	if err != nil {
		log.Println("Error creating table")
		return nil, fmt.Errorf("Failed create table%v", err)
	}
	rows, err := s.DB.Query(`SELECT videoId, uploadedTime FROM videos ORDER BY uploadedTime DESC`)
	if err != nil {
		return nil, fmt.Errorf("Failed select %v", err)
	}
	defer rows.Close()
	var videos []VideoMetadata
	for rows.Next() {
		var video VideoMetadata
		err = rows.Scan(&video.Id, &video.UploadedAt)
		log.Printf("%s %s", video.Id, video.UploadedAt)
		if err != nil {
			return nil, fmt.Errorf("%v", err)
		}
		videos = append(videos, video)
	}
	if len(videos) == 0 {
		return []VideoMetadata{}, nil
	}
	return videos, nil
}

func (s *SQLiteVideoMetadataService) Read(id string) (*VideoMetadata, error) {
	_, err := s.DB.Exec(` CREATE TABLE IF NOT EXISTS videos (videoId TEXT PRIMARY KEY, uploadedTime TIMESTAMP);`)
	if err != nil {
		return nil, fmt.Errorf("failed to create videos table: %v", err)
	}
	row := s.DB.QueryRow(`SELECT videoId, uploadedTime FROM videos WHERE videoId = ?`, id)
	var metadata VideoMetadata
	err = row.Scan(&metadata.Id, &metadata.UploadedAt)
	if err != nil {
		if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no vids")}
		return nil, fmt.Errorf("failed to read video metadata: %v", err)
	}
	return &metadata, nil
}