package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStartGreetingsRejectsInvalidInput(t *testing.T) {
	for _, body := range []string{
		`not JSON`,
		`{"cities":[]}`,
		`{"cities":[" "]}`,
		`{"cities":["` + strings.Repeat("a", 81) + `"]}`,
		`{"cities":[` + strings.Repeat(`"Tokyo",`, 32) + `"Seattle"]}`,
	} {
		t.Run(body, func(t *testing.T) {
			response := httptest.NewRecorder()
			StartGreetings(response, httptest.NewRequest(http.MethodPost, "/api/greetings", strings.NewReader(body)))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d; want 400", response.Code)
			}
		})
	}
}

func TestSayHello(t *testing.T) {
	greeting, err := SayHello(context.Background(), "Seattle")
	if err != nil || greeting != "Hello, Seattle!" {
		t.Fatalf("greeting = %q, error = %v", greeting, err)
	}
}

func TestStartGreetingsRequiresDurableClient(t *testing.T) {
	response := httptest.NewRecorder()
	StartGreetings(response, httptest.NewRequest(http.MethodPost, "/api/greetings", strings.NewReader(`{"cities":["Tokyo"]}`)))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want 500 without host-injected client", response.Code)
	}
}
