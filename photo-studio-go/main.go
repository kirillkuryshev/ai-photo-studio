package main

import (
	"database/sql"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"strconv"
	"bytes"
	"io"
	"mime/multipart"
	"os"
	"time"
)

var db *sql.DB

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type HistoryItem struct {
	ID          int    `json:"id"`
	AiModel     string `json:"ai_model"`
	Status      string `json:"status"`
	ImageBefore string `json:"image_before"`
	ImageAfter  string `json:"image_after"`
	CreatedAt   string `json:"created_at"`
}

type HistoryResponse struct {
	Data []HistoryItem `json:"data"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
    Message string `json:"message"`
    Token   string `json:"token"`
}

func getUserIDFromToken(r *http.Request) (int, error) {
	authHeader := r.Header.Get("Authorization")

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return 0, fmt.Errorf("invalid token")
	}

	plainTextToken := strings.TrimPrefix(authHeader, "Bearer ")

	parts := strings.SplitN(plainTextToken, "|", 2)

	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid token")
	}

	tokenID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid token")
	}

	token := parts[1]

	tokenHash := sha256.Sum256([]byte(token))
	tokenHashString := hex.EncodeToString(tokenHash[:])

	userID := 0

	err = db.QueryRow(
		`SELECT user_id
		FROM personal_access_tokens
		WHERE id = ? AND token = ?`,
		tokenID,
		tokenHashString,
	).Scan(&userID)

	if err != nil {
		return 0, fmt.Errorf("invalid token")
	}

	return userID, nil
}

func generateToken() (string, error) {
    tokenBytes := make([]byte, 32)
    _, err := rand.Read(tokenBytes)
    if err != nil {
        return "", err
    }
    token := hex.EncodeToString(tokenBytes)
    return token, nil
}

func sendError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	response := ErrorResponse{
		Message: message,
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(response)
}

func userHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID, err := getUserIDFromToken(r)
	if err != nil {
		sendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	historyItems := []HistoryItem{}
	rows, err := db.Query(
		`SELECT id, ai_model, status, image_before, image_after, created_at
		FROM user_history
		WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		item := HistoryItem{}
		err := rows.Scan(
			&item.ID,
			&item.AiModel,
			&item.Status,
			&item.ImageBefore,
			&item.ImageAfter,
			&item.CreatedAt,
		)
		if err != nil {
			sendError(w, "Database error", http.StatusInternalServerError)
			return
		}

		historyItems = append(historyItems, item)
	}
	if err := rows.Err(); err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}
	response := HistoryResponse{
		Data: historyItems,
	}
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		sendError(w, "JSON error", http.StatusInternalServerError)
		return
	}
	w.Write(jsonBytes)
}

func login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request := LoginRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&request)
	if err != nil {
		sendError(w, err.Error(), http.StatusBadRequest)
		return
	}
	savedPassword := ""
	err = db.QueryRow(
		"SELECT password FROM users WHERE email = ?",
		request.Email,
	).Scan(&savedPassword)
	if err == sql.ErrNoRows {
		sendError(w, "Error email or password", http.StatusUnauthorized)
		return
	}
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}
	err = bcrypt.CompareHashAndPassword(
		[]byte(savedPassword),
		[]byte(request.Password),
	)
	if err != nil {
		sendError(w, "Error email or password", http.StatusUnauthorized)
		return
	}
	userID := 0

	err = db.QueryRow(
		"SELECT id FROM users WHERE email = ?",
		request.Email,
	).Scan(&userID)
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	token, err := generateToken()
	if err != nil {
		sendError(w, "Token generation error", http.StatusInternalServerError)
		return
	}

	tokenHash := sha256.Sum256([]byte(token))
	tokenHashString := hex.EncodeToString(tokenHash[:])

	tokenResult, err := db.Exec(
		`INSERT INTO personal_access_tokens
		(user_id, name, token, abilities, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		userID,
		"frontend",
		tokenHashString,
		`["*"]`,
	)
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	tokenID, err := tokenResult.LastInsertId()
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	plainTextToken := fmt.Sprintf("%d|%s", tokenID, token)

	response := AuthResponse{
		Message: "Succesful log in!",
		Token:   plainTextToken,
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		sendError(w, "JSON error", http.StatusInternalServerError)
		return
	}

	w.Write(jsonBytes)
}

func signup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request := RegisterRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&request)
	if err != nil {
		sendError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Name == "" {
		sendError(w, "Name is required", http.StatusBadRequest)
		return
	}
	if request.Email == "" {
		sendError(w, "Email is required", http.StatusBadRequest)
		return
	}
	_, err = mail.ParseAddress(request.Email)
	if err != nil {
		sendError(w, "Invalid email", http.StatusBadRequest)
		return
	}
	if request.Password == "" {
		sendError(w, "Password is required", http.StatusBadRequest)
		return
	}
	if len(request.Password) < 6 {
		sendError(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}
	savedEmail := ""
	err = db.QueryRow(
		"SELECT email FROM users WHERE email = ?",
		request.Email,
	).Scan(&savedEmail)
	if err == nil {
		sendError(w, "User already exists", http.StatusConflict)
		return
	}
	if err != sql.ErrNoRows {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(request.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		sendError(w, "Password hash error", http.StatusInternalServerError)
		return
	}
	result, err := db.Exec(
		"INSERT INTO users (name, email, password) VALUES (?, ?, ?)",
		request.Name,
		request.Email,
		string(hashedPassword),
	)
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}
	userID, err := result.LastInsertId()
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}
	token, err := generateToken()
	if err != nil {
		sendError(w, "Token generation error", http.StatusInternalServerError)
		return
	}
	tokenHash := sha256.Sum256([]byte(token))
	tokenHashString := hex.EncodeToString(tokenHash[:])
	tokenResult, err := db.Exec(
		`INSERT INTO personal_access_tokens
		(user_id, name, token, abilities, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		userID,
		"frontend",
		tokenHashString,
		`["*"]`,
	)
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}
	tokenID, err := tokenResult.LastInsertId()
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}
	plainTextToken := fmt.Sprintf("%d|%s", tokenID, token)
	response := AuthResponse{
		Message: "Registration successful!",
		Token:   plainTextToken,
	}
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		sendError(w, "JSON error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(jsonBytes)
}

func transformPhoto(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromToken(r)
	if err != nil {
		sendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		sendError(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("photo")
	if err != nil {
		sendError(w, "Photo is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileNameBefore := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileHeader.Filename)
	pathBefore := "user_histories/" + fileNameBefore

	err = os.MkdirAll("user_histories", 0755)
	if err != nil {
		sendError(w, "Failed to create storage directory", http.StatusInternalServerError)
		return
	}

	originalFile, err := os.Create(pathBefore)
	if err != nil {
		sendError(w, "Failed to save photo", http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(originalFile, file)
	originalFile.Close()

	if err != nil {
		sendError(w, "Failed to save photo", http.StatusInternalServerError)
		return
	}

	historyResult, err := db.Exec(
		`INSERT INTO user_history
		(user_id, ai_model, status, image_before, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		userID,
		"Restore Old Photos",
		"in_progress",
		pathBefore,
	)
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	historyID, err := historyResult.LastInsertId()
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	fileBytes, err := os.ReadFile(pathBefore)
	if err != nil {
		sendError(w, "Failed to read photo", http.StatusInternalServerError)
		return
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("photo", fileHeader.Filename)
	if err != nil {
		sendError(w, "Failed to prepare photo", http.StatusInternalServerError)
		return
	}

	_, err = part.Write(fileBytes)
	if err != nil {
		sendError(w, "Failed to prepare photo", http.StatusInternalServerError)
		return
	}

	writer.Close()

	request, err := http.NewRequest(
		http.MethodPost,
		"http://127.0.0.1:8001/restore",
		body,
	)
	if err != nil {
		sendError(w, "AI request error", http.StatusInternalServerError)
		return
	}

	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		db.Exec(
			"UPDATE user_history SET status = ?, updated_at = datetime('now') WHERE id = ?",
			"failed",
			historyID,
		)

		sendError(w, "AI service error", http.StatusInternalServerError)
		return
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		db.Exec(
			"UPDATE user_history SET status = ?, updated_at = datetime('now') WHERE id = ?",
			"failed",
			historyID,
		)

		sendError(w, "AI service error", http.StatusInternalServerError)
		return
	}

	resultBytes, err := io.ReadAll(response.Body)
	if err != nil {
		sendError(w, "Failed to read AI response", http.StatusInternalServerError)
		return
	}

	fileNameAfter := fmt.Sprintf("%d_result.jpg", time.Now().UnixNano())
	pathAfter := "user_histories/" + fileNameAfter

	err = os.WriteFile(pathAfter, resultBytes, 0644)
	if err != nil {
		sendError(w, "Failed to save result", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec(
		`UPDATE user_history
		SET status = ?, image_after = ?, updated_at = datetime('now')
		WHERE id = ?`,
		"completed",
		pathAfter,
		historyID,
	)
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	w.Write(resultBytes)
}

func upscale4K(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromToken(r)
	if err != nil {
		sendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		sendError(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("photo")
	if err != nil {
		sendError(w, "Photo is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileNameBefore := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileHeader.Filename)
	pathBefore := "user_histories/" + fileNameBefore

	err = os.MkdirAll("user_histories", 0755)
	if err != nil {
		sendError(w, "Failed to create storage directory", http.StatusInternalServerError)
		return
	}

	originalFile, err := os.Create(pathBefore)
	if err != nil {
		sendError(w, "Failed to save photo", http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(originalFile, file)
	originalFile.Close()

	if err != nil {
		sendError(w, "Failed to save photo", http.StatusInternalServerError)
		return
	}

	historyResult, err := db.Exec(
		`INSERT INTO user_history
		(user_id, ai_model, status, image_before, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		userID,
		"Upscale Image to 4K",
		"in_progress",
		pathBefore,
	)
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	historyID, err := historyResult.LastInsertId()
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	fileBytes, err := os.ReadFile(pathBefore)
	if err != nil {
		sendError(w, "Failed to read photo", http.StatusInternalServerError)
		return
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("photo", fileHeader.Filename)
	if err != nil {
		sendError(w, "Failed to prepare photo", http.StatusInternalServerError)
		return
	}

	_, err = part.Write(fileBytes)
	if err != nil {
		sendError(w, "Failed to prepare photo", http.StatusInternalServerError)
		return
	}

	writer.Close()

	request, err := http.NewRequest(
		http.MethodPost,
		"http://127.0.0.1:8001/upscale-4k",
		body,
	)
	if err != nil {
		sendError(w, "AI request error", http.StatusInternalServerError)
		return
	}

	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		db.Exec(
			"UPDATE user_history SET status = ?, updated_at = datetime('now') WHERE id = ?",
			"failed",
			historyID,
		)

		sendError(w, "AI service error", http.StatusInternalServerError)
		return
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		db.Exec(
			"UPDATE user_history SET status = ?, updated_at = datetime('now') WHERE id = ?",
			"failed",
			historyID,
		)

		sendError(w, "AI service error", http.StatusInternalServerError)
		return
	}

	resultBytes, err := io.ReadAll(response.Body)
	if err != nil {
		sendError(w, "Failed to read AI response", http.StatusInternalServerError)
		return
	}

	fileNameAfter := fmt.Sprintf("%d_result.jpg", time.Now().UnixNano())
	pathAfter := "user_histories/" + fileNameAfter

	err = os.WriteFile(pathAfter, resultBytes, 0644)
	if err != nil {
		sendError(w, "Failed to save result", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec(
		`UPDATE user_history
		SET status = ?, image_after = ?, updated_at = datetime('now')
		WHERE id = ?`,
		"completed",
		pathAfter,
		historyID,
	)
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	w.Write(resultBytes)
}

func colorize(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromToken(r)
	if err != nil {
		sendError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		sendError(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("photo")
	if err != nil {
		sendError(w, "Photo is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileNameBefore := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileHeader.Filename)
	pathBefore := "user_histories/" + fileNameBefore

	err = os.MkdirAll("user_histories", 0755)
	if err != nil {
		sendError(w, "Failed to create storage directory", http.StatusInternalServerError)
		return
	}

	originalFile, err := os.Create(pathBefore)
	if err != nil {
		sendError(w, "Failed to save photo", http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(originalFile, file)
	originalFile.Close()

	if err != nil {
		sendError(w, "Failed to save photo", http.StatusInternalServerError)
		return
	}

	historyResult, err := db.Exec(
		`INSERT INTO user_history
		(user_id, ai_model, status, image_before, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		userID,
		"Colorize Photos",
		"in_progress",
		pathBefore,
	)
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	historyID, err := historyResult.LastInsertId()
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	fileBytes, err := os.ReadFile(pathBefore)
	if err != nil {
		sendError(w, "Failed to read photo", http.StatusInternalServerError)
		return
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("photo", fileHeader.Filename)
	if err != nil {
		sendError(w, "Failed to prepare photo", http.StatusInternalServerError)
		return
	}

	_, err = part.Write(fileBytes)
	if err != nil {
		sendError(w, "Failed to prepare photo", http.StatusInternalServerError)
		return
	}

	writer.Close()

	request, err := http.NewRequest(
		http.MethodPost,
		"http://127.0.0.1:8001/colorize",
		body,
	)
	if err != nil {
		sendError(w, "AI request error", http.StatusInternalServerError)
		return
	}

	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		db.Exec(
			"UPDATE user_history SET status = ?, updated_at = datetime('now') WHERE id = ?",
			"failed",
			historyID,
		)

		sendError(w, "AI service error", http.StatusInternalServerError)
		return
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		db.Exec(
			"UPDATE user_history SET status = ?, updated_at = datetime('now') WHERE id = ?",
			"failed",
			historyID,
		)

		sendError(w, "AI service error", http.StatusInternalServerError)
		return
	}

	resultBytes, err := io.ReadAll(response.Body)
	if err != nil {
		sendError(w, "Failed to read AI response", http.StatusInternalServerError)
		return
	}

	fileNameAfter := fmt.Sprintf("%d_result.jpg", time.Now().UnixNano())
	pathAfter := "user_histories/" + fileNameAfter

	err = os.WriteFile(pathAfter, resultBytes, 0644)
	if err != nil {
		sendError(w, "Failed to save result", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec(
		`UPDATE user_history
		SET status = ?, image_after = ?, updated_at = datetime('now')
		WHERE id = ?`,
		"completed",
		pathAfter,
		historyID,
	)
	if err != nil {
		sendError(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	w.Write(resultBytes)
}

func main() {
	var err error
	db, err = sql.Open(
		"sqlite",
		"database.sqlite",
	)
	if err != nil {
		fmt.Println("Database connection error:", err)
		return
	}
	err = db.Ping()
	if err != nil {
		fmt.Println("Database ping error:", err)
		return
	}
	fmt.Println("Database connected")
    
	http.HandleFunc("/api/user-history", userHistory)
	http.HandleFunc("/api/login", login)
	http.HandleFunc("/api/signup", signup)
	http.HandleFunc("/api/photos/transform", transformPhoto)
	http.HandleFunc("/api/photos/upscale-4k", upscale4K)
	http.HandleFunc("/api/photos/colorize", colorize)

	fileServer := http.FileServer(http.Dir("."))
	http.Handle("/storage/", http.StripPrefix("/storage/", fileServer))

	fmt.Println("Server is running on port 8000...")
	http.ListenAndServe(":8000", cors(http.DefaultServeMux))
}