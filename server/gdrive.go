package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const (
	tokenFileName     = "gdrive_token.json"
	credentialsFile   = "gdrive_credentials.json"
	defaultBackupFolder = "Cheapskate Backups"
)

type GDriveConfig struct {
	Enabled          bool   `json:"enabled"`
	Connected        bool   `json:"connected"`
	FolderID         string `json:"folder_id"`
	FolderName       string `json:"folder_name"`
	Email            string `json:"email"`
	LastSync         string `json:"last_sync"`
	ClientID         string `json:"client_id"`
	ClientSecret     string `json:"client_secret"`
	RedirectURI      string `json:"redirect_uri"`
}

type GDriveService struct {
	config      *GDriveConfig
	token       *oauth2.Token
	drive       *drive.Service
	tokenSource oauth2.TokenSource
}

func NewGDriveService(cfg *GDriveConfig) (*GDriveService, error) {
	svc := &GDriveService{
		config: cfg,
	}

	if cfg.Enabled && cfg.ClientID != "" && cfg.ClientSecret != "" {
		if err := svc.loadToken(); err != nil {
			log.Printf("GDrive: could not load token: %v", err)
		}
	}

	return svc, nil
}

func (s *GDriveService) isConfigured() bool {
	return s.config != nil && s.config.Enabled && s.config.ClientID != "" && s.config.ClientSecret != ""
}

func (s *GDriveService) getOAuth2Config() *oauth2.Config {
	redirectURI := "http://localhost:8080/oauth2/callback"
	if s.config != nil && s.config.RedirectURI != "" {
		redirectURI = s.config.RedirectURI
	}

	return &oauth2.Config{
		ClientID:     s.config.ClientID,
		ClientSecret: s.config.ClientSecret,
		Scopes:       []string{drive.DriveScope},
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURI,
	}
}

func (s *GDriveService) loadToken() error {
	if s.config == nil {
		return fmt.Errorf("no config")
	}

	tokenPath := s.getTokenPath()
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return err
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return err
	}

	s.token = &token
	s.tokenSource = s.getOAuth2Config().TokenSource(context.Background(), s.token)
	return nil
}

func (s *GDriveService) getTokenPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".cheapskate", tokenFileName)
}

func (s *GDriveService) GetAuthURL() string {
	if s.config == nil {
		return ""
	}
	return s.getOAuth2Config().AuthCodeURL("state", oauth2.AccessTypeOffline)
}

func (s *GDriveService) HandleCallback(code string) error {
	token, err := s.getOAuth2Config().Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	if err := s.saveToken(token); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	s.token = token
	s.tokenSource = s.getOAuth2Config().TokenSource(context.Background(), token)

	if err := s.ensureBackupFolder(); err != nil {
		return fmt.Errorf("failed to ensure backup folder: %w", err)
	}

	return nil
}

func (s *GDriveService) saveToken(token *oauth2.Token) error {
	homeDir, _ := os.UserHomeDir()
	tokenDir := filepath.Join(homeDir, ".cheapskate")
	if err := os.MkdirAll(tokenDir, 0755); err != nil {
		return err
	}

	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(tokenDir, tokenFileName), data, 0600)
}

func (s *GDriveService) ensureBackupFolder() error {
	ctx := context.Background()

	client := s.getClient()
	svc, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return err
	}

	s.drive = svc

	query := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder' and 'me' in owners", defaultBackupFolder)
	fileList, err := svc.Files.List().Q(query).Spaces("drive").Fields("files(id, name)").Context(ctx).Do()
	if err != nil {
		return err
	}

	if len(fileList.Files) > 0 {
		s.config.FolderID = fileList.Files[0].Id
		s.config.FolderName = defaultBackupFolder
		return nil
	}

	folder := &drive.File{
		Name:     defaultBackupFolder,
		MimeType: "application/vnd.google-apps.folder",
	}
	created, err := svc.Files.Create(folder).Fields("id", "name").Context(ctx).Do()
	if err != nil {
		return err
	}

	s.config.FolderID = created.Id
	s.config.FolderName = defaultBackupFolder

	user, err := svc.About().Get().Fields("user").Context(ctx).Do()
	if err == nil && user != nil && user.User != nil {
		s.config.Email = user.User.EmailAddress
	}

	return nil
}

func (s *GDriveService) getClient() *http.Client {
	return oauth2.NewClient(context.Background(), s.tokenSource)
}

func (s *GDriveService) IsConnected() bool {
	return s.isConfigured() && s.token != nil
}

func (s *GDriveService) UploadBackup(dbPath, jsonPath string) error {
	if !s.IsConnected() {
		return fmt.Errorf("not connected to Google Drive")
	}

	ctx := context.Background()

	svc, err := drive.NewService(ctx, option.WithHTTPClient(s.getClient()))
	if err != nil {
		return err
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")

	dbFile, err := os.Open(dbPath)
	if err != nil {
		return err
	}
	defer dbFile.Close()

	dbFileName := fmt.Sprintf("cheapskate_%s.db", timestamp)
	dbFileMeta := &drive.File{
		Name:     dbFileName,
		Parents:  []string{s.config.FolderID},
		MimeType: "application/x-sqlite3",
	}
	_, err = svc.Files.Create(dbFileMeta).Media(dbFile).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to upload DB: %w", err)
	}

	jsonFile, err := os.Open(jsonPath)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	jsonFileName := fmt.Sprintf("cheapskate_%s.json", timestamp)
	jsonFileMeta := &drive.File{
		Name:     jsonFileName,
		Parents:  []string{s.config.FolderID},
		MimeType: "application/json",
	}
	_, err = svc.Files.Create(jsonFileMeta).Media(jsonFile).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to upload JSON: %w", err)
	}

	s.config.LastSync = time.Now().Format(time.RFC3339)

	return nil
}

func (s *GDriveService) ListBackups() ([]drive.File, error) {
	if !s.IsConnected() {
		return nil, fmt.Errorf("not connected to Google Drive")
	}

	ctx := context.Background()

	svc, err := drive.NewService(ctx, option.WithHTTPClient(s.getClient()))
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("'%s' in parents and name contains 'cheapskate_' and name contains '.db' and trashed=false", s.config.FolderID)
	fileList, err := svc.Files.List().
		Q(query).
		Spaces("drive").
		Fields("files(id, name, createdTime, modifiedTime, size)").
		OrderBy("createdTime desc").
		Context(ctx).Do()

	if err != nil {
		return nil, err
	}

	return fileList.Files, nil
}

func (s *GDriveService) DownloadBackup(fileID, destPath string) error {
	if !s.IsConnected() {
		return fmt.Errorf("not connected to Google Drive")
	}

	ctx := context.Background()

	svc, err := drive.NewService(ctx, option.WithHTTPClient(s.getClient()))
	if err != nil {
		return err
	}

	resp, err := svc.Files.Get(fileID).Download()
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	return err
}

func (s *GDriveService) Disconnect() error {
	if s.config == nil {
		return nil
	}

	tokenPath := s.getTokenPath()
	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	s.token = nil
	s.tokenSource = nil
	s.drive = nil
	s.config.Connected = false
	s.config.FolderID = ""
	s.config.FolderName = ""
	s.config.Email = ""
	s.config.LastSync = ""

	return nil
}

func (s *GDriveService) GetConfig() *GDriveConfig {
	return s.config
}

func (s *GDriveService) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"enabled":   s.config.Enabled,
		"connected": s.IsConnected(),
	}

	if s.config != nil {
		status["folder_id"] = s.config.FolderID
		status["folder_name"] = s.config.FolderName
		status["email"] = s.config.Email
		status["last_sync"] = s.config.LastSync
	}

	return status
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseCredentials(credJSON string) (clientID, clientSecret string, err error) {
	var creds struct {
		Web struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		} `json:"web"`
	}

	if err := json.Unmarshal([]byte(credJSON), &creds); err != nil {
		return "", "", err
	}

	return creds.Web.ClientID, creds.Web.ClientSecret, nil
}