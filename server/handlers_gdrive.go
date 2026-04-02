package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/calexandrepcjr/cheapskate-finance-tracker/client/templates"
	"github.com/calexandrepcjr/cheapskate-finance-tracker/server/db"
)

const (
	gdriveAPIURL         = "https://www.googleapis.com/drive/v3"
	gdriveUploadURL     = "https://www.googleapis.com/upload/drive/v3"
	gdriveOAuthAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	gdriveOAuthTokenURL = "https://oauth2.googleapis.com/token"
	gdriveScope         = "https://www.googleapis.com/auth/drive.file"
)

type GDriveConfig struct {
	Enabled      bool   `json:"enabled"`
	FolderID     string `json:"folder_id"`
	FolderName   string `json:"folder_name"`
	LastSyncAt   string `json:"last_sync_at"`
	AutoBackup   bool   `json:"auto_backup"`
	ClientID     string `json:"-"`
	ClientSecret string `json:"-"`
	AccessToken  string `json:"-"`
	RefreshToken string `json:"-"`
}

type GDriveStatusResponse struct {
	Enabled      bool   `json:"enabled"`
	Connected    bool   `json:"connected"`
	FolderID     string `json:"folder_id"`
	FolderName   string `json:"folder_name"`
	LastSyncAt   string `json:"last_sync_at"`
	AutoBackup   bool   `json:"auto_backup"`
	AuthURL      string `json:"auth_url,omitempty"`
	ClientID     string `json:"-"`
}

type GDriveConnectRequest struct {
	AuthCode    string `json:"auth_code"`
	FolderID    string `json:"folder_id"`
	FolderName  string `json:"folder_name"`
	AutoBackup  bool   `json:"auto_backup"`
}

type GDriveConnectResponse struct {
	Success  bool   `json:"success"`
	FolderID string `json:"folder_id,omitempty"`
	Message  string `json:"message"`
}

type GDriveBackupResponse struct {
	Success    bool   `json:"success"`
	FileID     string `json:"file_id,omitempty"`
	Checksum   string `json:"checksum"`
	BackupSize int64  `json:"backup_size"`
	Message    string `json:"message"`
}

type GDriveRestoreResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (app *Application) HandleGDriveStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cfg, err := app.Q.GetGdriveConfig(ctx)
	if err != nil {
		templates.GDriveStatusError("Failed to load configuration").Render(r.Context(), w)
		return
	}

	resp := GDriveStatusResponse{
		Enabled:     cfg.Enabled == 1,
		Connected:   cfg.Enabled == 1 && cfg.FolderID.String != "",
		FolderID:    cfg.FolderID.String,
		FolderName:  cfg.FolderName.String,
		AutoBackup:  cfg.AutoBackup == 1,
		LastSyncAt:  "",
	}

	if cfg.LastSyncAt.Valid {
		resp.LastSyncAt = cfg.LastSyncAt.Time.UTC().Format(time.RFC3339)
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID != "" {
		authURL := buildAuthURL(clientID)
		resp.AuthURL = authURL
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (app *Application) HandleGDriveConnect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method == "GET" {
		clientID := os.Getenv("GOOGLE_CLIENT_ID")
		if clientID == "" {
			templates.GDriveConnectError("Google OAuth not configured. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables.").Render(r.Context(), w)
			return
		}

		authURL := buildAuthURL(clientID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"auth_url": authURL})
		return
	}

	var req GDriveConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		templates.GDriveConnectError("Invalid request").Render(r.Context(), w)
		return
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		templates.GDriveConnectError("OAuth credentials not configured").Render(r.Context(), w)
		return
	}

	token, err := exchangeCodeForToken(clientID, clientSecret, req.AuthCode)
	if err != nil {
		templates.GDriveConnectError("Failed to authenticate: " + err.Error()).Render(r.Context(), w)
		return
	}

	err = app.Q.UpsertGdriveConfig(ctx, db.UpsertGdriveConfigParams{
		Enabled:    req.FolderID != "",
		FolderID:   req.FolderID,
		FolderName: req.FolderName,
		AutoBackup: req.AutoBackup,
	})
	if err != nil {
		templates.GDriveConnectError("Failed to save configuration").Render(r.Context(), w)
		return
	}

	storeTokenInEnv(token.AccessToken, token.RefreshToken)

	resp := GDriveConnectResponse{
		Success:  true,
		FolderID: req.FolderID,
		Message:  "Successfully connected to Google Drive",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (app *Application) HandleGDriveDisconnect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := app.Q.DeleteGdriveConfig(ctx)
	if err != nil {
		templates.GDriveDisconnectError("Failed to disconnect").Render(r.Context(), w)
		return
	}

	clearTokensFromEnv()

	templates.GDriveDisconnected().Render(r.Context(), w)
}

func (app *Application) HandleGDriveBackup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cfg, err := app.Q.GetGdriveConfig(ctx)
	if err != nil || cfg.FolderID.String == "" {
		templates.GDriveBackupError("Google Drive not connected").Render(r.Context(), w)
		return
	}

	tmpFile, err := os.CreateTemp("", "cheapskate-gdrive-*.db")
	if err != nil {
		templates.GDriveBackupError("Failed to create backup").Render(r.Context(), w)
		return
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := sqliteBackup(app.DB, tmpPath); err != nil {
		templates.GDriveBackupError("Failed to create backup: " + err.Error()).Render(r.Context(), w)
		return
	}

	fileData, err := os.ReadFile(tmpPath)
	if err != nil {
		templates.GDriveBackupError("Failed to read backup").Render(r.Context(), w)
		return
	}

	checksum := computeChecksum(fileData)

	accessToken := getAccessToken()
	if accessToken == "" {
		templates.GDriveBackupError("Not authenticated").Render(r.Context(), w)
		return
	}

	folderID := cfg.FolderID.String
	filename := fmt.Sprintf("cheapskate-backup-%s.db", time.Now().Format("2006-01-02"))

	fileID, err := uploadToDrive(accessToken, folderID, filename, fileData)
	if err != nil {
		templates.GDriveBackupError("Failed to upload: " + err.Error()).Render(r.Context(), w)
		return
	}

	app.Q.UpdateGdriveLastSync(ctx)

	_ = app.Q.InsertBackupMetadata(ctx, db.InsertBackupMetadataParams{
		Version:       "1.0",
		SchemaVersion: 1,
		Checksum:      checksum,
	})

	resp := GDriveBackupResponse{
		Success:    true,
		FileID:     fileID,
		Checksum:   checksum,
		BackupSize: int64(len(fileData)),
		Message:    "Backup uploaded to Google Drive",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (app *Application) HandleGDriveRestore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cfg, err := app.Q.GetGdriveConfig(ctx)
	if err != nil || cfg.FolderID.String == "" {
		templates.GDriveRestoreError("Google Drive not connected").Render(r.Context(), w)
		return
	}

	accessToken := getAccessToken()
	if accessToken == "" {
		templates.GDriveRestoreError("Not authenticated").Render(r.Context(), w)
		return
	}

	folderID := cfg.FolderID.String

	files, err := listBackups(accessToken, folderID)
	if err != nil || len(files) == 0 {
		templates.GDriveRestoreError("No backup found").Render(r.Context(), w)
		return
	}

	latestFile := files[0]

	fileContent, err := downloadFromDrive(accessToken, latestFile.ID)
	if err != nil {
		templates.GDriveRestoreError("Failed to download: " + err.Error()).Render(r.Context(), w)
		return
	}

	tmpFile, err := os.CreateTemp("", "cheapskate-restore-*.db")
	if err != nil {
		templates.GDriveRestoreError("Failed to create temp file").Render(r.Context(), w)
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(fileContent); err != nil {
		tmpFile.Close()
		templates.GDriveRestoreError("Failed to write temp file").Render(r.Context(), w)
		return
	}
	tmpFile.Close()

	if err := sqliteRestore(app.DB, tmpPath); err != nil {
		templates.GDriveRestoreError("Failed to restore: " + err.Error()).Render(r.Context(), w)
		return
	}

	resp := GDriveRestoreResponse{
		Success: true,
		Message: "Data restored from Google Drive backup",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func buildAuthURL(clientID string) string {
	redirectURI := "http://localhost:8080/api/gdrive/callback"
	return fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&access_type=offline&prompt=consent",
		gdriveOAuthAuthURL,
		clientID,
		redirectURI,
		gdriveScope)
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func exchangeCodeForToken(clientID, clientSecret, code string) (*oauthTokenResponse, error) {
	data := fmt.Sprintf("code=%s&client_id=%s&client_secret=%s&redirect_uri=http://localhost:8080/api/gdrive/callback&grant_type=authorization_code",
		code, clientID, clientSecret)

	req, err := http.NewRequest("POST", gdriveOAuthTokenURL, bytes.NewBufferString(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var token oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

type driveFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func uploadToDrive(accessToken, folderID, filename string, data []byte) (string, error) {
	metadata := map[string]interface{}{
		"name":     filename,
		"parents":  []string{folderID},
	}
	metadataJSON, _ := json.Marshal(metadata)

	req, err := http.NewRequest("POST", gdriveUploadURL+"/files?uploadType=multipart", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	boundary := "-------314159265358979323846"
	body := &bytes.Buffer{}
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: application/json; charset=utf-8\r\n\r\n")
	body.Write(metadataJSON)
	body.WriteString("\r\n--" + boundary + "\r\n")
	body.WriteString("application/octet-stream\r\n\r\n")
	body.Write(data)
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req.Body = io.NopCloser(body)
	req.Header.Set("Content-Type", "multipart/related; boundary="+boundary)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed: %s", string(respBody))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.ID, nil
}

func listBackups(accessToken, folderID string) ([]driveFile, error) {
	url := fmt.Sprintf("%s/files?q='%s'+in+parents+and+name+contains+'cheapskate-backup'&orderBy=createdTime+desc&fields=files(id,name)",
		gdriveAPIURL, folderID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Files []driveFile `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Files, nil
}

func downloadFromDrive(accessToken, fileID string) ([]byte, error) {
	url := fmt.Sprintf("%s/files/%s?alt=media", gdriveAPIURL, fileID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func computeChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}

func getAccessToken() string {
	return os.Getenv("GOOGLE_ACCESS_TOKEN")
}

func storeTokenInEnv(accessToken, refreshToken string) {
	os.Setenv("GOOGLE_ACCESS_TOKEN", accessToken)
	os.Setenv("GOOGLE_REFRESH_TOKEN", refreshToken)
}

func clearTokensFromEnv() {
	os.Unsetenv("GOOGLE_ACCESS_TOKEN")
	os.Unsetenv("GOOGLE_REFRESH_TOKEN")
}

func (app *Application) HandleGDriveOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No authorization code provided", http.StatusBadRequest)
		return
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		templates.GDriveConnectError("OAuth credentials not configured").Render(r.Context(), w)
		return
	}

	token, err := exchangeCodeForToken(clientID, clientSecret, code)
	if err != nil {
		templates.GDriveConnectError("Failed to authenticate: " + err.Error()).Render(r.Context(), w)
		return
	}

	storeTokenInEnv(token.AccessToken, token.RefreshToken)

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>Google Drive Connected</title>
			<meta http-equiv="refresh" content="2;url=/settings">
		</head>
		<body style="font-family: sans-serif; padding: 40px; text-align: center;">
			<h2>Successfully connected to Google Drive!</h2>
			<p>Redirecting back to settings...</p>
			<script>
				setTimeout(function() {
					window.location.href = '/settings';
				}, 2000);
			</script>
		</body>
		</html>
	`)
}

func (app *Application) HandleGDriveAutoBackup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	autoBackup := r.FormValue("auto_backup") == "on"

	cfg, err := app.Q.GetGdriveConfig(ctx)
	if err != nil || !cfg.Enabled.Bool {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "not_configured"})
		return
	}

	err = app.Q.UpsertGdriveConfig(ctx, db.UpsertGdriveConfigParams{
		Enabled:    true,
		FolderID:   cfg.FolderID.String,
		FolderName: cfg.FolderName.String,
		AutoBackup: autoBackup,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"auto_backup": autoBackup})
}