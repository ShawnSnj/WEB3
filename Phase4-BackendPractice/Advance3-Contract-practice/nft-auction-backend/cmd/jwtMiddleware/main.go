package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// UserClaims 结构体定义了 JWT Payload 中包含的自定义数据。
// It must embed jwt.RegisteredClaims for standard fields like exp, iat.
type UserClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// ⚠️ IMPORTANT: In production, this key must be complex and securely stored (e.g., in KMS).
const secretKey = "your_super_secure_and_long_secret_key"

// ContextKey is used to store and retrieve data from the request context.
type ContextKey string

const UserIDKey ContextKey = "userID"

// --------------------------- JWT Core Logic (Simplified from previous example) ---------------------------

// GenerateJWT creates a new JWT token.
func GenerateJWT(userID string, role string) (string, error) {
	expirationTime := time.Now().Add(1 * time.Hour)
	claims := &UserClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "SecurityAPI",
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

// ValidateJWT validates the token string and returns the claims if valid.
func ValidateJWT(tokenString string) (*UserClaims, error) {
	claims := &UserClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// --------------------------- MIDDLEWARE IMPLEMENTATION ---------------------------

// AuthMiddleware is the core middleware function for JWT authentication.
// It takes an http.Handler (the next handler in the chain) and returns a new http.Handler.
func AuthMiddleware(next http.Handler) http.Handler {
	// The http.HandlerFunc type is used to wrap a function that matches the signature.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract the token from the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// The header typically looks like "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Invalid Authorization format. Expected 'Bearer <token>'", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// 2. Validate the token
		claims, err := ValidateJWT(tokenString)
		if err != nil {
			log.Printf("JWT validation error: %v", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// 3. Token is valid. Attach user info to the request context
		// This allows subsequent handlers (like ProtectedHandler) to access the user ID.
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		r = r.WithContext(ctx)

		// 4. Pass the request to the next handler in the chain
		next.ServeHTTP(w, r)
	})
}

// --------------------------- HANDLERS ---------------------------

// LoginHandler simulates user login and issues a JWT token.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// In a real app, you would verify username/password here.
	// For this example, we hardcode a successful login.
	testUserID := "user-12345"
	testRole := "standard"

	token, err := GenerateJWT(testUserID, testRole)
	if err != nil {
		http.Error(w, "Could not generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Login successful. Use this token for protected routes.",
		"token":   token,
	})
}

// PublicHandler is a route that does not require authentication.
func PublicHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "This is a public endpoint. Anyone can access.",
	})
}

// ProtectedHandler is a route that requires the AuthMiddleware.
// It retrieves the user ID from the request context.
func ProtectedHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve the userID that AuthMiddleware placed in the context.
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		// This should theoretically not happen if middleware is working correctly.
		http.Error(w, "Internal server error: user context missing", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Welcome, %s! This data is protected.", userID),
		"data":    "Sensitive Blockchain Configuration Data",
	})
}

// --------------------------- MAIN ---------------------------

func main() {
	// Setup handlers
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/public", PublicHandler)

	// Apply AuthMiddleware to the ProtectedHandler
	// http.Handle expects an http.Handler, so we use AuthMiddleware(http.HandlerFunc(...))
	protectedHandlerWithAuth := AuthMiddleware(http.HandlerFunc(ProtectedHandler))
	http.Handle("/protected", protectedHandlerWithAuth)

	port := ":8080"
	fmt.Printf("Server listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
