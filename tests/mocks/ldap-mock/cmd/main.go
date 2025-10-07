package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// LDAPMockServer implements a simple HTTP-based LDAP mock server for testing
type LDAPMockServer struct {
	users  map[string]*LDAPUser
	groups map[string]*LDAPGroup
}

// LDAPUser represents a mock LDAP user
type LDAPUser struct {
	DN               string   `json:"dn"`
	Username         string   `json:"username"`
	Password         string   `json:"password"`
	Email            string   `json:"email"`
	DisplayName      string   `json:"displayName"`
	Groups           []string `json:"groups"`
	DistinguishedName string   `json:"distinguishedName"`
}

// LDAPGroup represents a mock LDAP group
type LDAPGroup struct {
	DN          string   `json:"dn"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

// AuthRequest represents an authentication request
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	Success bool              `json:"success"`
	User    *LDAPUser         `json:"user,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// SearchRequest represents a search request
type SearchRequest struct {
	BaseDN string `json:"baseDN"`
	Filter string `json:"filter"`
}

// SearchResponse represents a search response
type SearchResponse struct {
	Success bool          `json:"success"`
	Entries []*LDAPUser   `json:"entries,omitempty"`
	Groups  []*LDAPGroup  `json:"groups,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// NewLDAPMockServer creates a new LDAP mock server
func NewLDAPMockServer() *LDAPMockServer {
	// Create test users and groups
	users := map[string]*LDAPUser{
		"testuser": {
			DN:               "cn=testuser,ou=users,dc=test,dc=com",
			Username:         "testuser",
			Password:         "testpass",
			Email:            "testuser@test.com",
			DisplayName:      "Test User",
			DistinguishedName: "cn=testuser,ou=users,dc=test,dc=com",
			Groups:           []string{"ext_writer", "ext_reader", "int_writer", "int_reader"},
		},
		"admin": {
			DN:               "cn=admin,ou=users,dc=test,dc=com",
			Username:         "admin",
			Password:         "adminpass",
			Email:            "admin@test.com",
			DisplayName:      "Admin User",
			DistinguishedName: "cn=admin,ou=users,dc=test,dc=com",
			Groups:           []string{"ext_writer", "ext_reader", "int_writer", "int_reader", "admin"},
		},
	}

	groups := map[string]*LDAPGroup{
		"ext_writer": {
			DN:          "cn=ext_writer,ou=groups,dc=test,dc=com",
			Name:        "ext_writer",
			Description: "External SMTS Writers",
			Members:     []string{"cn=testuser,ou=users,dc=test,dc=com", "cn=admin,ou=users,dc=test,dc=com"},
		},
		"ext_reader": {
			DN:          "cn=ext_reader,ou=groups,dc=test,dc=com",
			Name:        "ext_reader",
			Description: "External SMTS Readers",
			Members:     []string{"cn=testuser,ou=users,dc=test,dc=com", "cn=admin,ou=users,dc=test,dc=com"},
		},
		"int_writer": {
			DN:          "cn=int_writer,ou=groups,dc=test,dc=com",
			Name:        "int_writer",
			Description: "Internal SMTS Writers",
			Members:     []string{"cn=testuser,ou=users,dc=test,dc=com", "cn=admin,ou=users,dc=test,dc=com"},
		},
		"int_reader": {
			DN:          "cn=int_reader,ou=groups,dc=test,dc=com",
			Name:        "int_reader",
			Description: "Internal SMTS Readers",
			Members:     []string{"cn=testuser,ou=users,dc=test,dc=com", "cn=admin,ou=users,dc=test,dc=com"},
		},
		"admin": {
			DN:          "cn=admin,ou=groups,dc=test,dc=com",
			Name:        "admin",
			Description: "Administrators",
			Members:     []string{"cn=admin,ou=users,dc=test,dc=com"},
		},
	}

	return &LDAPMockServer{
		users:  users,
		groups: groups,
	}
}

// handleAuth handles authentication requests
func (s *LDAPMockServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}

	log.Printf("Authentication request for user: %s", req.Username)

	// Find user
	user, exists := s.users[req.Username]
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	// Check password
	if user.Password != req.Password {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Error:   "Invalid password",
		})
		return
	}

	// Successful authentication
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		User:    user,
	})

	log.Printf("Successful authentication for user: %s", req.Username)
}

// handleSearch handles search requests
func (s *LDAPMockServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Invalid JSON: %v"}`, err), http.StatusBadRequest)
		return
	}

	log.Printf("Search request: BaseDN=%s, Filter=%s", req.BaseDN, req.Filter)

	var entries []*LDAPUser
	var groups []*LDAPGroup

	// Handle user searches
	if strings.Contains(req.BaseDN, "ou=users") || req.BaseDN == "dc=test,dc=com" || req.BaseDN == "" {
		for _, user := range s.users {
			if s.matchesFilter(user, req.Filter) {
				entries = append(entries, user)
			}
		}
	}

	// Handle group searches
	if strings.Contains(req.BaseDN, "ou=groups") || req.BaseDN == "dc=test,dc=com" || req.BaseDN == "" {
		for _, group := range s.groups {
			if s.matchesGroupFilter(group, req.Filter) {
				groups = append(groups, group)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(SearchResponse{
		Success: true,
		Entries: entries,
		Groups:  groups,
	})

	log.Printf("Search completed with %d user entries and %d group entries", len(entries), len(groups))
}

// matchesFilter checks if a user matches the search filter
func (s *LDAPMockServer) matchesFilter(user *LDAPUser, filter string) bool {
	// Simple filter matching for common patterns
	if strings.Contains(filter, fmt.Sprintf("(uid=%s)", user.Username)) {
		return true
	}
	if strings.Contains(filter, fmt.Sprintf("(cn=%s)", user.Username)) {
		return true
	}
	if strings.Contains(filter, "(objectClass=*)") {
		return true
	}
	if filter == "" {
		return true
	}
	return false
}

// matchesGroupFilter checks if a group matches the search filter
func (s *LDAPMockServer) matchesGroupFilter(group *LDAPGroup, filter string) bool {
	// Simple filter matching for common patterns
	if strings.Contains(filter, fmt.Sprintf("(cn=%s)", group.Name)) {
		return true
	}
	if strings.Contains(filter, "(objectClass=*)") {
		return true
	}
	if filter == "" {
		return true
	}
	if strings.Contains(filter, "(member=") {
		// Check if this group contains the specified member
		for _, member := range strings.Split(filter, "(member=") {
			if len(member) > 0 {
				memberDN := strings.TrimSuffix(strings.Split(member, ")")[0], ")")
				for _, m := range group.Members {
					if m == memberDN {
						return true
					}
				}
			}
		}
	}
	return false
}

// handleHealth handles health check
func (s *LDAPMockServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "ldap-mock",
	})
}

func main() {
	// Get port from environment or use default
	port := os.Getenv("LDAP_MOCK_PORT")
	if port == "" {
		port = "1389"
	}

	// Start LDAP mock server
	server := NewLDAPMockServer()

	// Setup HTTP routes
	http.HandleFunc("/auth", server.handleAuth)
	http.HandleFunc("/search", server.handleSearch)
	http.HandleFunc("/health", server.handleHealth)

	log.Printf("LDAP Mock Server starting on port %s", port)
	log.Printf("Available endpoints:")
	log.Printf("  POST /auth - Authenticate user")
	log.Printf("  POST /search - Search users/groups")
	log.Printf("  GET /health - Health check")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start LDAP mock server: %v", err)
	}
}