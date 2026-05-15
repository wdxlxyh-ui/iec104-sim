package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"iec104-sim/internal/model"

	"golang.org/x/crypto/bcrypt"
)

type UserStore struct {
	mu       sync.RWMutex
	filePath string
	users    map[string]*model.User // key: username
}

func NewUserStore(filePath string) *UserStore {
	s := &UserStore{
		filePath: filePath,
		users:    make(map[string]*model.User),
	}
	s.load()
	s.seedDefault()
	return s
}

func (s *UserStore) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	var users []*model.User
	if err := json.Unmarshal(data, &users); err != nil {
		return
	}
	for _, u := range users {
		s.users[u.Username] = u
	}
}

func (s *UserStore) save() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*model.User, 0, len(s.users))
	for _, u := range s.users {
		list = append(list, u)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(s.filePath, data, 0644)
}

func (s *UserStore) seedDefault() {
	if _, ok := s.users["admin"]; ok {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("Test1234"), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("failed to hash default password: %v", err))
	}
	s.users["admin"] = &model.User{
		ID:        "admin-001",
		Username:  "admin",
		PasswordHash: string(hash),
		Role:      model.RoleAdmin,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	s.save()
}

func (s *UserStore) Authenticate(username, password string) (*model.User, error) {
	s.mu.RLock()
	u, ok := s.users[username]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	if !u.Enabled {
		return nil, fmt.Errorf("user disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid password")
	}
	return u, nil
}

func (s *UserStore) GetByUsername(username string) (*model.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	if !ok {
		return nil, false
	}
	return u, true
}

func (s *UserStore) List() []*model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*model.User, 0, len(s.users))
	for _, u := range s.users {
		list = append(list, u)
	}
	return list
}

func (s *UserStore) Create(req model.CreateUserRequest) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[req.Username]; ok {
		return nil, fmt.Errorf("username already exists")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	role := model.UserRole(req.Role)
	if role != model.RoleAdmin && role != model.RoleViewer {
		role = model.RoleViewer
	}
	u := &model.User{
		ID:           fmt.Sprintf("user-%d", time.Now().UnixNano()),
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         role,
		Enabled:      true,
		CreatedAt:    time.Now(),
	}
	s.users[req.Username] = u
	s.save()
	return u, nil
}

func (s *UserStore) Delete(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[username]; !ok {
		return fmt.Errorf("user not found")
	}
	if username == "admin" {
		return fmt.Errorf("cannot delete admin")
	}
	delete(s.users, username)
	s.save()
	return nil
}

func (s *UserStore) UpdateLastLogin(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[username]; ok {
		u.LastLogin = time.Now()
		s.save()
	}
}
