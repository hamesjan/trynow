// Lab 7: Implement a web server

package web

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"html/template"
	"os"
	"io"
	"path/filepath"
	"time"
	"fmt"
	"os/exec"
)

type server struct {
	Addr string
	Port int

	metadataService VideoMetadataService
	contentService  VideoContentService

	mux *http.ServeMux
}

func NewServer(
	metadataService VideoMetadataService,
	contentService VideoContentService,
) *server {
	return &server{
		metadataService: metadataService,
		contentService:  contentService,
	}
}

type VideoInfo struct {
	EscapedId   string
	Id      	string
	UploadTime 	string	
}

type VideoInfoVideoPage struct {
	Id      	string
	UploadedAt 	string	
}

// var vidList []VideoInfo


func (s *server) Start(lis net.Listener) error {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/upload", s.handleUpload)
	s.mux.HandleFunc("/videos/", s.handleVideo)
	s.mux.HandleFunc("/content/", s.handleVideoContent)
	s.mux.HandleFunc("/", s.handleIndex)

	return http.Serve(lis, s.mux)
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if s.contentService == nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if s.metadataService == nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	tmplIndex := template.Must(template.New("index").Parse(indexHTML))
	vids, err := s.metadataService.List()
	if err != nil {
		log.Printf("%s", err)
		http.Error(w, "Error list", http.StatusInternalServerError)
		return
	}
	vidList := make([]VideoInfo, 0, len(vids))
	// vidList = []VideoInfo{}
	for _, vid := range vids {
		tempVid := VideoInfo{
			Id: vid.Id,
			EscapedId: url.PathEscape(vid.Id),
			UploadTime: vid.UploadedAt.Format("2006-01-02 15:04:05"),
		}
		vidList = append(vidList, tempVid)
	}
	err = tmplIndex.Execute(w, vidList)
	if err != nil {
		http.Error(w, "ERor Executing ", http.StatusInternalServerError)
		return
	}
}

func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if s.contentService == nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if s.metadataService == nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Parse Form failed", http.StatusBadRequest)
		return
	}
	f, h, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Get file failed", http.StatusBadRequest)
		return
	}
	defer f.Close()
	videoID := strings.TrimSuffix(h.Filename, filepath.Ext(h.Filename))
	videoData, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "Read file fail", http.StatusInternalServerError)
		return
	}
	tempDir, err := os.MkdirTemp("", "*")
	if err != nil {
		http.Error(w, "Failed create tmp", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)
	tempVid := filepath.Join(tempDir, videoID + ".mp4")
	err = os.WriteFile(tempVid, videoData, 0644)
	if err != nil {
		http.Error(w, "Failed to save tmp vid", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempVid)
	manifestPath := filepath.Join(tempDir, "manifest.mpd")
	cmd := exec.Command("ffmpeg",
		"-i", tempVid,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-bf", "1",
		"-keyint_min", "120",
		"-g", "120",
		"-sc_threshold", "0",
		"-b:v", "3000k",
		"-b:a", "128k",
		"-f", "dash",
		"-use_timeline", "1",
		"-use_template", "1",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		"-seg_duration", "4",
		manifestPath,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		http.Error(w, "Video conversion error", http.StatusInternalServerError)
		return
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		http.Error(w, "Manifest read fail", http.StatusInternalServerError)
		return
	}
	err = s.contentService.Write(videoID, "manifest.mpd", manifestData)
	if err != nil {
		http.Error(w, "Manifest write fail", http.StatusInternalServerError)
		return
	}
	segmentFiles, err := filepath.Glob(filepath.Join(tempDir, "*.m4s"))
	if err != nil {
		http.Error(w, "Segment read fail", http.StatusInternalServerError)
		return
	}
	for _, segmentPath := range segmentFiles {
		segmentName := filepath.Base(segmentPath)
		segmentData, err := os.ReadFile(segmentPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Segment %s fail", segmentName), http.StatusInternalServerError)
			return
		}
		err = s.contentService.Write(videoID, segmentName, segmentData)
		log.Printf("Write to file: %s", segmentName)
		if err != nil {
			log.Printf("Write fail: %v", err)
			http.Error(w, "Segment write fail", http.StatusInternalServerError)
			return
		}
		os.Remove(segmentPath)
	}
	uploadTime := time.Now()
	log.Printf("TIME: %s", uploadTime)
	err = s.metadataService.Create(videoID, uploadTime)
	if err != nil {
		http.Error(w, "Metadata fail", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) handleVideo(w http.ResponseWriter, r *http.Request) {
	videoId := r.URL.Path[len("/videos/"):]
	tmplIndex := template.Must(template.New("video").Parse(videoHTML))
	readVideo, err := s.metadataService.Read(videoId)
	if err != nil {
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}
	var readVideoDict = VideoInfoVideoPage{}
	readVideoDict.Id = readVideo.Id
	readVideoDict.UploadedAt = readVideo.UploadedAt.Format("2006-01-02 15:04:05")
	err = tmplIndex.Execute(w, readVideoDict)
	if err != nil {
		log.Printf("%s", err)
		http.Error(w, "Error loading page", http.StatusInternalServerError)
		return
	}
}

func (s *server) handleVideoContent(w http.ResponseWriter, r *http.Request) {
	videoId := r.URL.Path[len("/content/"):]
	parts := strings.Split(videoId, "/")
	if len(parts) != 2 {
		http.Error(w, "Path error", http.StatusBadRequest)
		return
	}
	videoId = parts[0]
	filename := parts[1]
	// log.Println("Video ID:", videoId, "Filename:", filename)
	data, err := s.contentService.Read(videoId, filename)
	if err != nil {
		log.Printf("%s", err)
		http.Error(w, "No file found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.Write(data)
}
