package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
)

func setupTestDB(t *testing.T) (*pgxpool.Pool, *InventoryService, *BookingService) {
	ctx := context.Background()
	// Use default local test container connection
	dbURL := "postgres://teren:teren123@localhost:5432/teren_hotels?sslmode=disable"
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	roomRepo := repository.NewRoomRepository(db)
	roomBlockRepo := repository.NewRoomBlockRepository(db)
	bookingRepo := repository.NewBookingRepository(db)

	inventorySvc := NewInventoryService(db, roomRepo, roomBlockRepo)
	bookingSvc := NewBookingService(db, bookingRepo, inventorySvc)

	return db, inventorySvc, bookingSvc
}

type testFixture struct {
	PropertyID uuid.UUID
	UserID     uuid.UUID
	FloorID    uuid.UUID
	RoomTypeID uuid.UUID
	GuestID    uuid.UUID
}

func createTestFixture(ctx context.Context, t *testing.T, db *pgxpool.Pool) testFixture {
	fixture := testFixture{
		PropertyID: uuid.New(),
		UserID:     uuid.New(),
		FloorID:    uuid.New(),
		RoomTypeID: uuid.New(),
		GuestID:    uuid.New(),
	}

	// Insert Property
	_, err := db.Exec(ctx, `
		INSERT INTO properties (id, name, slug, currency, timezone)
		VALUES ($1, 'Test Property', $2, 'IDR', 'UTC')
	`, fixture.PropertyID, "test-slug-"+fixture.PropertyID.String()[:8])
	if err != nil {
		t.Fatalf("failed to create property: %v", err)
	}

	// Insert User
	_, err = db.Exec(ctx, `
		INSERT INTO users (id, property_id, name, email, role)
		VALUES ($1, $2, 'Test User', $3, 'admin')
	`, fixture.UserID, fixture.PropertyID, "user-"+fixture.UserID.String()[:8]+"@test.com")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Insert Floor
	_, err = db.Exec(ctx, `
		INSERT INTO floors (id, property_id, floor_number, label, sort_order)
		VALUES ($1, $2, 1, 'Floor 1', 1)
	`, fixture.FloorID, fixture.PropertyID)
	if err != nil {
		t.Fatalf("failed to create floor: %v", err)
	}

	// Insert Room Type
	_, err = db.Exec(ctx, `
		INSERT INTO room_types (id, property_id, name, max_occupancy)
		VALUES ($1, $2, 'Standard', 2)
	`, fixture.RoomTypeID, fixture.PropertyID)
	if err != nil {
		t.Fatalf("failed to create room type: %v", err)
	}

	// Insert Guest
	_, err = db.Exec(ctx, `
		INSERT INTO guests (id, property_id, full_name)
		VALUES ($1, $2, 'Test Guest')
	`, fixture.GuestID, fixture.PropertyID)
	if err != nil {
		t.Fatalf("failed to create guest: %v", err)
	}

	t.Cleanup(func() {
		// Cascade deletes will clean up rooms, blocks, bookings, etc.
		_, _ = db.Exec(context.Background(), "DELETE FROM users WHERE id = $1", fixture.UserID)
		_, _ = db.Exec(context.Background(), "DELETE FROM guests WHERE id = $1", fixture.GuestID)
		_, _ = db.Exec(context.Background(), "DELETE FROM properties WHERE id = $1", fixture.PropertyID)
	})

	return fixture
}

func createTestRoom(ctx context.Context, t *testing.T, db *pgxpool.Pool, f testFixture, number string, x, y int, status string) uuid.UUID {
	roomID := uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO rooms (id, property_id, floor_id, room_type_id, number, pos_x, pos_y, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, roomID, f.PropertyID, f.FloorID, f.RoomTypeID, number, x, y, status)
	if err != nil {
		t.Fatalf("failed to create room %s: %v", number, err)
	}
	return roomID
}

// ==========================================
// 1.1 Inventory Service (GetMapWithAvailability)
// ==========================================

func TestGetMap_AvailableRoom(t *testing.T) {
	ctx := context.Background()
	db, inventorySvc, _ := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	roomID := createTestRoom(ctx, t, db, f, "101", 0, 0, "active")

	// Query map for today where room has no bookings
	dateFrom, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	dateTo, _ := time.Parse("2006-01-02", time.Now().AddDate(0, 0, 1).Format("2006-01-02"))

	res, err := inventorySvc.GetMap(ctx, models.MapAvailabilityRequest{
		PropertyID: f.PropertyID,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
	})
	if err != nil {
		t.Fatalf("GetMap failed: %v", err)
	}

	if len(res.Floors) == 0 || len(res.Floors[0].Rooms) == 0 {
		t.Fatal("expected rooms to be returned")
	}

	room := res.Floors[0].Rooms[0]
	if room.ID != roomID {
		t.Fatalf("expected room ID %v, got %v", roomID, room.ID)
	}
	if room.Availability != "available" {
		t.Fatalf("expected availability 'available', got '%s'", room.Availability)
	}
	if room.ActiveBookingID != nil {
		t.Fatalf("expected active booking ID to be nil, got %v", room.ActiveBookingID)
	}
}

func TestGetMap_OccupiedRoom(t *testing.T) {
	ctx := context.Background()
	db, inventorySvc, _ := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	roomID := createTestRoom(ctx, t, db, f, "102", 0, 1, "active")

	// Create a checked_in booking
	bookingID := uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO bookings (id, property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
		VALUES ($1, $2, $3, $4, $5, NOW() - INTERVAL '1 hour', NOW() + INTERVAL '1 day', 100000, 'ota', 'checked_in')
	`, bookingID, f.PropertyID, roomID, f.GuestID, f.UserID)
	if err != nil {
		t.Fatalf("failed to create booking: %v", err)
	}

	dateFrom, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	dateTo, _ := time.Parse("2006-01-02", time.Now().AddDate(0, 0, 1).Format("2006-01-02"))

	res, err := inventorySvc.GetMap(ctx, models.MapAvailabilityRequest{
		PropertyID: f.PropertyID,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
	})
	if err != nil {
		t.Fatalf("GetMap failed: %v", err)
	}

	room := res.Floors[0].Rooms[0]
	if room.Availability != "occupied" {
		t.Fatalf("expected availability 'occupied', got '%s'", room.Availability)
	}
	if room.ActiveBookingID == nil || *room.ActiveBookingID != bookingID {
		t.Fatalf("expected active booking %v, got %v", bookingID, room.ActiveBookingID)
	}
}

func TestGetMap_PendingRoom(t *testing.T) {
	ctx := context.Background()
	db, inventorySvc, _ := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	roomID := createTestRoom(ctx, t, db, f, "103", 0, 2, "active")

	// Create a confirmed booking (pending check-in)
	bookingID := uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO bookings (id, property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
		VALUES ($1, $2, $3, $4, $5, NOW() - INTERVAL '1 hour', NOW() + INTERVAL '1 day', 100000, 'ota', 'confirmed')
	`, bookingID, f.PropertyID, roomID, f.GuestID, f.UserID)
	if err != nil {
		t.Fatalf("failed to create booking: %v", err)
	}

	dateFrom, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	dateTo, _ := time.Parse("2006-01-02", time.Now().AddDate(0, 0, 1).Format("2006-01-02"))

	res, err := inventorySvc.GetMap(ctx, models.MapAvailabilityRequest{
		PropertyID: f.PropertyID,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
	})
	if err != nil {
		t.Fatalf("GetMap failed: %v", err)
	}

	room := res.Floors[0].Rooms[0]
	if room.Availability != "pending" {
		t.Fatalf("expected availability 'pending', got '%s'", room.Availability)
	}
	if room.PendingBookingID == nil || *room.PendingBookingID != bookingID {
		t.Fatalf("expected pending booking %v, got %v", bookingID, room.PendingBookingID)
	}
}

func TestGetMap_BlockedRoom(t *testing.T) {
	ctx := context.Background()
	db, inventorySvc, _ := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	roomID := createTestRoom(ctx, t, db, f, "104", 0, 3, "active")

	// Create a room block
	blockID := uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO room_blocks (id, room_id, created_by, start_date, end_date, reason)
		VALUES ($1, $2, $3, CURRENT_DATE - INTERVAL '1 day', CURRENT_DATE + INTERVAL '2 days', 'maintenance')
	`, blockID, roomID, f.UserID)
	if err != nil {
		t.Fatalf("failed to create room block: %v", err)
	}

	dateFrom, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	dateTo, _ := time.Parse("2006-01-02", time.Now().AddDate(0, 0, 1).Format("2006-01-02"))

	res, err := inventorySvc.GetMap(ctx, models.MapAvailabilityRequest{
		PropertyID: f.PropertyID,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
	})
	if err != nil {
		t.Fatalf("GetMap failed: %v", err)
	}

	room := res.Floors[0].Rooms[0]
	if room.Availability != "blocked" {
		t.Fatalf("expected availability 'blocked', got '%s'", room.Availability)
	}
	if room.BlockID == nil || *room.BlockID != blockID {
		t.Fatalf("expected block %v, got %v", blockID, room.BlockID)
	}
}

func TestGetMap_PriorityLogic(t *testing.T) {
	ctx := context.Background()
	db, inventorySvc, _ := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	roomID := createTestRoom(ctx, t, db, f, "105", 0, 4, "active")

	// Create BOTH check-in booking and room block for the same room & date range
	bookingID := uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO bookings (id, property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
		VALUES ($1, $2, $3, $4, $5, NOW() - INTERVAL '1 hour', NOW() + INTERVAL '1 day', 100000, 'ota', 'checked_in')
	`, bookingID, f.PropertyID, roomID, f.GuestID, f.UserID)
	if err != nil {
		t.Fatalf("failed to create booking: %v", err)
	}

	blockID := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO room_blocks (id, room_id, created_by, start_date, end_date, reason)
		VALUES ($1, $2, $3, CURRENT_DATE - INTERVAL '1 day', CURRENT_DATE + INTERVAL '2 days', 'maintenance')
	`, blockID, roomID, f.UserID)
	if err != nil {
		t.Fatalf("failed to create room block: %v", err)
	}

	dateFrom, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	dateTo, _ := time.Parse("2006-01-02", time.Now().AddDate(0, 0, 1).Format("2006-01-02"))

	res, err := inventorySvc.GetMap(ctx, models.MapAvailabilityRequest{
		PropertyID: f.PropertyID,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
	})
	if err != nil {
		t.Fatalf("GetMap failed: %v", err)
	}

	room := res.Floors[0].Rooms[0]
	// Priority Logic: Booking > Block, so it should be "occupied"
	if room.Availability != "occupied" {
		t.Fatalf("expected availability 'occupied' due to priority logic, got '%s'", room.Availability)
	}
}

func TestGetMap_InactiveRoom(t *testing.T) {
	ctx := context.Background()
	db, inventorySvc, _ := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	// Create inactive room
	roomID := createTestRoom(ctx, t, db, f, "106In", 0, 5, "inactive")

	// Even if there is a booking/block, inactive rooms must always show "inactive"
	bookingID := uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO bookings (id, property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
		VALUES ($1, $2, $3, $4, $5, NOW() - INTERVAL '1 hour', NOW() + INTERVAL '1 day', 100000, 'ota', 'checked_in')
	`, bookingID, f.PropertyID, roomID, f.GuestID, f.UserID)
	if err != nil {
		t.Fatalf("failed to create booking: %v", err)
	}

	dateFrom, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	dateTo, _ := time.Parse("2006-01-02", time.Now().AddDate(0, 0, 1).Format("2006-01-02"))

	res, err := inventorySvc.GetMap(ctx, models.MapAvailabilityRequest{
		PropertyID: f.PropertyID,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
	})
	if err != nil {
		t.Fatalf("GetMap failed: %v", err)
	}

	room := res.Floors[0].Rooms[0]
	if room.Availability != "inactive" {
		t.Fatalf("expected availability 'inactive', got '%s'", room.Availability)
	}
}

// ==========================================
// 1.2 Booking Service (Assignment & Status)
// ==========================================

func TestAssign_RoomAvailable(t *testing.T) {
	ctx := context.Background()
	db, _, bookingSvc := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	roomID := createTestRoom(ctx, t, db, f, "107", 0, 6, "active")

	// Assign to available room
	booking, err := bookingSvc.CreateBooking(ctx, models.CreateBookingRequest{
		PropertyID:  f.PropertyID,
		RoomID:      roomID,
		GuestID:     f.GuestID,
		CreatedBy:   f.UserID,
		CheckIn:     time.Now().Add(24 * time.Hour),
		CheckOut:    time.Now().Add(48 * time.Hour),
		TotalAmount: 200000,
		Source:      "ota",
		Status:      "confirmed",
	})
	if err != nil {
		t.Fatalf("expected successful booking assignment, got: %v", err)
	}

	if booking.ID == uuid.Nil {
		t.Fatal("expected booking ID to be generated")
	}
}

func TestAssign_RoomOccupied(t *testing.T) {
	ctx := context.Background()
	db, _, bookingSvc := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	roomID := createTestRoom(ctx, t, db, f, "108", 0, 7, "active")

	checkinStart := time.Now().Truncate(24 * time.Hour)
	checkinEnd := checkinStart.Add(24 * time.Hour)

	// Create occupied booking
	_, err := db.Exec(ctx, `
		INSERT INTO bookings (id, property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 100000, 'ota', 'checked_in')
	`, uuid.New(), f.PropertyID, roomID, f.GuestID, f.UserID, checkinStart, checkinEnd)
	if err != nil {
		t.Fatalf("failed to create active booking: %v", err)
	}

	// Try assigning during overlapping period
	_, err = bookingSvc.CreateBooking(ctx, models.CreateBookingRequest{
		PropertyID:  f.PropertyID,
		RoomID:      roomID,
		GuestID:     f.GuestID,
		CreatedBy:   f.UserID,
		CheckIn:     checkinStart.Add(1 * time.Hour),
		CheckOut:    checkinEnd.Add(1 * time.Hour),
		TotalAmount: 200000,
		Source:      "ota",
		Status:      "confirmed",
	})

	if err == nil {
		t.Fatal("expected ROOM_UNAVAILABLE error, got success")
	}

	var bErr *BusinessError
	if !errors.As(err, &bErr) || bErr.Code != "ROOM_UNAVAILABLE" {
		t.Fatalf("expected BusinessError with Code ROOM_UNAVAILABLE, got: %v", err)
	}
}

func TestAssign_RoomBlocked(t *testing.T) {
	ctx := context.Background()
	db, _, bookingSvc := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	roomID := createTestRoom(ctx, t, db, f, "109", 0, 8, "active")

	blockStart := time.Now().Truncate(24 * time.Hour)
	blockEnd := blockStart.Add(2 * 24 * time.Hour)

	// Create a room block
	_, err := db.Exec(ctx, `
		INSERT INTO room_blocks (room_id, created_by, start_date, end_date, reason)
		VALUES ($1, $2, $3, $4, 'maintenance')
	`, roomID, f.UserID, blockStart, blockEnd)
	if err != nil {
		t.Fatalf("failed to block room: %v", err)
	}

	// Try assigning booking during block
	_, err = bookingSvc.CreateBooking(ctx, models.CreateBookingRequest{
		PropertyID:  f.PropertyID,
		RoomID:      roomID,
		GuestID:     f.GuestID,
		CreatedBy:   f.UserID,
		CheckIn:     blockStart.Add(12 * time.Hour),
		CheckOut:    blockStart.Add(36 * time.Hour),
		TotalAmount: 200000,
		Source:      "ota",
		Status:      "confirmed",
	})

	if err == nil {
		t.Fatal("expected ROOM_UNAVAILABLE error on blocked room, got success")
	}

	var bErr *BusinessError
	if !errors.As(err, &bErr) || bErr.Code != "ROOM_UNAVAILABLE" {
		t.Fatalf("expected BusinessError with Code ROOM_UNAVAILABLE, got: %v", err)
	}
}

func TestCheckIn_Flow(t *testing.T) {
	ctx := context.Background()
	db, _, bookingSvc := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	roomID := createTestRoom(ctx, t, db, f, "110", 0, 9, "active")

	bookingID := uuid.New()
	// Insert confirmed booking
	_, err := db.Exec(ctx, `
		INSERT INTO bookings (id, property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
		VALUES ($1, $2, $3, $4, $5, CURRENT_DATE, CURRENT_DATE + INTERVAL '1 day', 100000, 'ota', 'confirmed')
	`, bookingID, f.PropertyID, roomID, f.GuestID, f.UserID)
	if err != nil {
		t.Fatalf("failed to create confirmed booking: %v", err)
	}

	// Perform check-in
	err = bookingSvc.CheckIn(ctx, bookingID)
	if err != nil {
		t.Fatalf("failed to check in: %v", err)
	}

	var status string
	err = db.QueryRow(ctx, "SELECT status FROM bookings WHERE id = $1", bookingID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query status: %v", err)
	}

	if status != "checked_in" {
		t.Fatalf("expected status 'checked_in', got '%s'", status)
	}
}

func TestCheckOut_Flow(t *testing.T) {
	ctx := context.Background()
	db, _, bookingSvc := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	roomID := createTestRoom(ctx, t, db, f, "111", 0, 10, "active")

	bookingID := uuid.New()
	// Insert checked_in booking
	_, err := db.Exec(ctx, `
		INSERT INTO bookings (id, property_id, room_id, guest_id, created_by, check_in, check_out, total_amount, source, status)
		VALUES ($1, $2, $3, $4, $5, CURRENT_DATE - INTERVAL '1 day', CURRENT_DATE, 100000, 'ota', 'checked_in')
	`, bookingID, f.PropertyID, roomID, f.GuestID, f.UserID)
	if err != nil {
		t.Fatalf("failed to create checked_in booking: %v", err)
	}

	// Perform check-out
	err = bookingSvc.CheckOut(ctx, bookingID)
	if err != nil {
		t.Fatalf("failed to check out: %v", err)
	}

	var status string
	err = db.QueryRow(ctx, "SELECT status FROM bookings WHERE id = $1", bookingID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query status: %v", err)
	}

	if status != "checked_out" {
		t.Fatalf("expected status 'checked_out', got '%s'", status)
	}
}

// ==========================================
// 1.3 Database Constraints (Integrity)
// ==========================================

func TestUniquePosition(t *testing.T) {
	ctx := context.Background()
	db, _, _ := setupTestDB(t)
	f := createTestFixture(ctx, t, db)

	// Create Room A at (1,1)
	createTestRoom(ctx, t, db, f, "RoomA", 1, 1, "active")

	// Try creating Room B at same floor & coordinates (1,1)
	roomIDB := uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO rooms (id, property_id, floor_id, room_type_id, number, pos_x, pos_y, status)
		VALUES ($1, $2, $3, $4, 'RoomB', 1, 1, 'active')
	`, roomIDB, f.PropertyID, f.FloorID, f.RoomTypeID)

	if err == nil {
		t.Fatal("expected unique position constraint violation, got success")
	}

	if !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected unique/duplicate key violation error, got: %v", err)
	}
}

func TestUniqueRoomNumber(t *testing.T) {
	ctx := context.Background()
	db, _, _ := setupTestDB(t)
	f := createTestFixture(ctx, t, db)

	// Create Room "101" on Floor 1
	createTestRoom(ctx, t, db, f, "101", 2, 2, "active")

	// Create a second Floor on same property
	floorID2 := uuid.New()
	_, err := db.Exec(ctx, `
		INSERT INTO floors (id, property_id, floor_number, label, sort_order)
		VALUES ($1, $2, 2, 'Floor 2', 2)
	`, floorID2, f.PropertyID)
	if err != nil {
		t.Fatalf("failed to create second floor: %v", err)
	}
	defer func() {
		_, _ = db.Exec(ctx, "DELETE FROM floors WHERE id = $1", floorID2)
	}()

	// Try to create Room "101" on Floor 2.
	// Since Room numbers must be unique per property (not just per floor), this should fail.
	roomID2 := uuid.New()
	_, err = db.Exec(ctx, `
		INSERT INTO rooms (id, property_id, floor_id, room_type_id, number, pos_x, pos_y, status)
		VALUES ($1, $2, $3, $4, '101', 3, 3, 'active')
	`, roomID2, f.PropertyID, floorID2, f.RoomTypeID)

	if err == nil {
		t.Fatal("expected unique room number per property violation, got success")
	}

	if !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected unique/duplicate key violation error, got: %v", err)
	}
}

func TestBlockOverlapConstraint(t *testing.T) {
	ctx := context.Background()
	db, inventorySvc, _ := setupTestDB(t)
	f := createTestFixture(ctx, t, db)
	roomID := createTestRoom(ctx, t, db, f, "112", 0, 11, "active")

	// Block Room for May 27-28
	start := time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)

	_, err := inventorySvc.BlockRoom(ctx, models.CreateRoomBlockRequest{
		RoomID:    roomID,
		CreatedBy: f.UserID,
		StartDate: start,
		EndDate:   end,
		Reason:    "maintenance",
	})
	if err != nil {
		t.Fatalf("failed to create first block: %v", err)
	}

	// Try blocking for overlapping range (May 27-29)
	_, err = inventorySvc.BlockRoom(ctx, models.CreateRoomBlockRequest{
		RoomID:    roomID,
		CreatedBy: f.UserID,
		StartDate: start,
		EndDate:   time.Date(2026, 5, 29, 0, 0, 0, 0, time.UTC),
		Reason:    "maintenance",
	})

	if err == nil {
		t.Fatal("expected BLOCK_CONFLICT error on overlapping blocks, got success")
	}

	var bErr *BusinessError
	if !errors.As(err, &bErr) || bErr.Code != "BLOCK_CONFLICT" {
		t.Fatalf("expected BusinessError with Code BLOCK_CONFLICT, got: %v", err)
	}
}
