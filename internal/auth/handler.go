package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"iec104-sim/internal/model"
)

type AuthHandler struct {
	store *UserStore
}

func NewAuthHandler(store *UserStore) *AuthHandler {
	return &AuthHandler{store: store}
}

func (h *AuthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/auth/login", h.handleLogin)
	mux.HandleFunc("/api/v1/auth/me", h.handleMe)
	mux.HandleFunc("/api/v1/users", h.handleUsers)
	mux.HandleFunc("/api/v1/users/", h.handleUserByID)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// AuthMiddleware validates JWT token and injects user info into context
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing authorization token")
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := ValidateToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		r.Header.Set("X-User-ID", claims.UserID)
		r.Header.Set("X-Username", claims.Username)
		r.Header.Set("X-Role", claims.Role)
		next.ServeHTTP(w, r)
	})
}

func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	user, err := h.store.Authenticate(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	token, err := GenerateToken(user.ID, user.Username, string(user.Role))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	h.store.UpdateLastLogin(req.Username)
	writeJSON(w, http.StatusOK, model.LoginResponse{
		Token: token,
		User:  *user,
	})
}

func (h *AuthHandler) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	username := r.Header.Get("X-Username")
	user, ok := h.store.GetByUsername(username)
	if !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Role") != "admin" {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		users := h.store.List()
		writeJSON(w, http.StatusOK, map[string]interface{}{"users": users})
	case http.MethodPost:
		var req model.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		user, err := h.store.Create(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, user)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET/POST required")
	}
}

func (h *AuthHandler) handleUserByID(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Role") != "admin" {
		writeError(w, http.StatusForbidden, "admin required")
		return
	}
	username := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	if username == "" {
		writeError(w, http.StatusBadRequest, "missing username")
		return
	}
	if r.Method == http.MethodDelete {
		if err := h.store.Delete(username); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "DELETE required")
}
