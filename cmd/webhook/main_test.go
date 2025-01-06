package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func TestGetPort(t *testing.T) {
	tests := []struct {
		name        string
		envPort     string
		want        string
		clearEnvVar bool
	}{
		{
			name:    "uses environment variable",
			envPort: "9090",
			want:    "9090",
		},
		{
			name:        "uses default when not set",
			clearEnvVar: true,
			want:        "8080",
		},
		{
			name:    "handles empty string",
			envPort: "",
			want:    "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.clearEnvVar {
				os.Unsetenv("PORT")
			} else {
				os.Setenv("PORT", tt.envPort)
			}

			if got := getPort(); got != tt.want {
				t.Errorf("getPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMiddlewareChaining(t *testing.T) {
	executionOrder := []string{}

	middleware1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionOrder = append(executionOrder, "middleware1_before")
			next.ServeHTTP(w, r)
			executionOrder = append(executionOrder, "middleware1_after")
		})
	}

	middleware2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionOrder = append(executionOrder, "middleware2_before")
			next.ServeHTTP(w, r)
			executionOrder = append(executionOrder, "middleware2_after")
		})
	}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executionOrder = append(executionOrder, "handler")
	})

	handler := chainMiddleware(finalHandler, middleware1, middleware2)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Check execution order
	expected := []string{
		"middleware1_before",
		"middleware2_before",
		"handler",
		"middleware2_after",
		"middleware1_after",
	}

	if !reflect.DeepEqual(executionOrder, expected) {
		t.Errorf("middleware execution order = %v, want %v", executionOrder, expected)
	}
}
