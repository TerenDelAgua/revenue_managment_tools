package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
)

func TestCreateRoom_ValidCoordinates(t *testing.T) {
	// BT-14: Create Room with pos_x: 15 (Out of bounds 0-11)
	handler := NewRoomHandler(nil) // repo is nil since we expect it to fail validation before repo access

	reqPayload := models.CreateRoomRequest{
		FloorID:    uuid.New(),
		RoomTypeID: uuid.New(),
		Number:     "101",
		Status:     "active",
		PosX:       15, // Out of bounds
		PosY:       5,
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", "/rooms", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to create http request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422 Unprocessable Entity, got %d", rr.Code)
	}

	var apiErr struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&apiErr); err != nil {
		t.Fatalf("failed to decode response error body: %v", err)
	}

	if apiErr.Code != "OUT_OF_BOUNDS" {
		t.Errorf("expected error code 'OUT_OF_BOUNDS', got '%s'", apiErr.Code)
	}
}
