package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Configuration constants and variables
const defaultPort = ":8080"

var (
	webhookSecret string
	bookmarkAPI   string
	apiToken      string
)

// BookmarkService handles communication with the bookmark API
type BookmarkService struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// BookmarkType represents the type of bookmark
type BookmarkType string

const (
	BookmarkTypeLink BookmarkType = "link"
)

// BookmarkRequest represents the JSON structure for creating a bookmark
type BookmarkRequest struct {
	Type BookmarkType `json:"type"`
	URL  string       `json:"url"`
}

// NewBookmarkService creates a new instance of BookmarkService
func NewBookmarkService(baseURL, token string) *BookmarkService {
	return &BookmarkService{
		baseURL:  baseURL,
		apiToken: token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// AddBookmark sends a request to add a new bookmark
func (s *BookmarkService) AddBookmark(entry Entry) error {
	// Create bookmark request from entry
	bookmark := BookmarkRequest{
		Type: BookmarkTypeLink,
		URL:  entry.URL,
	}

	jsonData, err := json.Marshal(bookmark)
	if err != nil {
		return fmt.Errorf("failed to marshal bookmark request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, s.baseURL+"/api/v1/bookmarks", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully created bookmark. Response: %s", string(body))
	return nil
}

type Feed struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	FeedURL   string    `json:"feed_url"`
	SiteURL   string    `json:"site_url"`
	Title     string    `json:"title"`
	CheckedAt time.Time `json:"checked_at"`
}

type Enclosure struct {
	ID              int    `json:"id"`
	UserID          int    `json:"user_id"`
	EntryID         int    `json:"entry_id"`
	URL             string `json:"url"`
	MimeType        string `json:"mime_type"`
	Size            int64  `json:"size"`
	MediaProgession int    `json:"media_progression"`
}

type Entry struct {
	ID          int         `json:"id"`
	UserID      int         `json:"user_id"`
	FeedID      int         `json:"feed_id"`
	Status      string      `json:"status"`
	Hash        string      `json:"hash"`
	Title       string      `json:"title"`
	URL         string      `json:"url"`
	CommentsURL string      `json:"comments_url"`
	PublishedAt time.Time   `json:"published_at"`
	CreatedAt   time.Time   `json:"created_at"`
	ChangedAt   time.Time   `json:"changed_at"`
	Content     string      `json:"content"`
	ShareCode   string      `json:"share_code"`
	Starred     bool        `json:"starred"`
	ReadingTime int         `json:"reading_time"`
	Enclosures  []Enclosure `json:"enclosures"`
	Tags        []string    `json:"tags"`
	Feed        *Feed       `json:"feed,omitempty"`
}

type NewEntriesPayload struct {
	EventType string  `json:"event_type"`
	Feed      Feed    `json:"feed"`
	Entries   []Entry `json:"entries"`
}

type SaveEntryPayload struct {
	EventType string `json:"event_type"`
	Entry     Entry  `json:"entry"`
}

func loadConfig() error {
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("error loading .env file: %w", err)
	}

	webhookSecret = os.Getenv("WEBHOOK_SECRET")
	if webhookSecret == "" {
		return errors.New("WEBHOOK_SECRET must be set in .env file")
	}

	bookmarkAPI = os.Getenv("BOOKMARK_API_URL")
	if bookmarkAPI == "" {
		return errors.New("BOOKMARK_API_URL must be set in .env file")
	}

	apiToken = os.Getenv("BOOKMARK_API_TOKEN")
	if apiToken == "" {
		return errors.New("BOOKMARK_API_TOKEN must be set in .env file")
	}

	return nil
}

func verifySignature(payload []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expectedMAC), []byte(signature))
}

var bookmarkService *BookmarkService

func handleNewEntries(feed Feed, entries []Entry) error {
	log.Printf("Processing %d new entries from feed: %s", len(entries), feed.Title)
	for _, entry := range entries {
		log.Printf("New entry: %s - %s", entry.Title, entry.URL)
	}
	return nil
}

func handleSaveEntry(entry Entry) error {
	log.Printf("Processing saved entry: %s - %s", entry.Title, entry.URL)

	if err := bookmarkService.AddBookmark(entry); err != nil {
		log.Printf("Failed to save bookmark for %s: %v", entry.URL, err)
		return fmt.Errorf("failed to save bookmark: %w", err)
	}

	log.Printf("Successfully saved bookmark for: %s", entry.URL)
	return nil
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	signature := r.Header.Get("X-Miniflux-Signature")
	if signature == "" {
		http.Error(w, "Missing signature", http.StatusBadRequest)
		return
	}

	eventType := r.Header.Get("X-Miniflux-Event-Type")
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	if !verifySignature(payload, signature) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	switch eventType {
	case "new_entries":
		var newEntries NewEntriesPayload
		if err := json.Unmarshal(payload, &newEntries); err != nil {
			http.Error(w, "Error parsing payload", http.StatusBadRequest)
			return
		}
		if err := handleNewEntries(newEntries.Feed, newEntries.Entries); err != nil {
			http.Error(w, "Error processing entries", http.StatusInternalServerError)
			return
		}

	case "save_entry":
		var saveEntry SaveEntryPayload
		if err := json.Unmarshal(payload, &saveEntry); err != nil {
			http.Error(w, "Error parsing payload", http.StatusBadRequest)
			return
		}
		if err := handleSaveEntry(saveEntry.Entry); err != nil {
			http.Error(w, "Error processing saved entry", http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, fmt.Sprintf("Unknown event type: %s", eventType), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func main() {
	if err := loadConfig(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	bookmarkService = NewBookmarkService(bookmarkAPI, apiToken)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	http.HandleFunc("/webhook", webhookHandler)

	log.Printf("Starting webhook server on port %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
