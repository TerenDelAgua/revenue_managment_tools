package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
)

type RoomRepository struct {
	db *pgxpool.Pool
}

func NewRoomRepository(db *pgxpool.Pool) *RoomRepository {
	return &RoomRepository{db: db}
}

func (r *RoomRepository) Create(ctx context.Context, req *models.CreateRoomRequest) (*models.Room, error) {
	var room models.Room
	err := r.db.QueryRow(ctx, `
		INSERT INTO rooms (floor_id, room_type_id, number, status, pos_x, pos_y) 
		VALUES ($1, $2, $3, $4, $5, $6) 
		RETURNING id, floor_id, room_type_id, number, status, pos_x, pos_y, created_at, updated_at`,
		req.FloorID, req.RoomTypeID, req.Number, req.Status, req.PosX, req.PosY,
	).Scan(
		&room.ID, &room.FloorID, &room.RoomTypeID, &room.Number, &room.Status,
		&room.PosX, &room.PosY, &room.CreatedAt, &room.UpdatedAt,
	)
	return &room, err
}

func (r *RoomRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Room, error) {
	var room models.Room
	err := r.db.QueryRow(ctx, `
		SELECT id, floor_id, room_type_id, number, status, pos_x, pos_y, created_at, updated_at 
		FROM rooms WHERE id = $1`, id).Scan(
		&room.ID, &room.FloorID, &room.RoomTypeID, &room.Number, &room.Status,
		&room.PosX, &room.PosY, &room.CreatedAt, &room.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &room, err
}

func (r *RoomRepository) ListByFloor(ctx context.Context, floorID uuid.UUID) ([]*models.Room, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, floor_id, room_type_id, number, status, pos_x, pos_y, created_at, updated_at 
		FROM rooms WHERE floor_id = $1 ORDER BY number ASC`, floorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*models.Room
	for rows.Next() {
		var room models.Room
		if err := rows.Scan(&room.ID, &room.FloorID, &room.RoomTypeID, &room.Number, &room.Status, &room.PosX, &room.PosY, &room.CreatedAt, &room.UpdatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, &room)
	}
	return rooms, rows.Err()
}

func (r *RoomRepository) UpdatePosition(ctx context.Context, id uuid.UUID, posX, posY int) (*models.Room, error) {
	var room models.Room
	err := r.db.QueryRow(ctx, `
		UPDATE rooms SET pos_x = $1, pos_y = $2, updated_at = NOW() 
		WHERE id = $3 RETURNING id, floor_id, room_type_id, number, status, pos_x, pos_y, created_at, updated_at`,
		posX, posY, id).Scan(
		&room.ID, &room.FloorID, &room.RoomTypeID, &room.Number, &room.Status,
		&room.PosX, &room.PosY, &room.CreatedAt, &room.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &room, err
}

// GetMapWithAvailability - Spec §3.1
// Devuelve estructura anidada Floors → Rooms con estado derivado.
func (r *RoomRepository) GetMapWithAvailability(ctx context.Context, req models.MapAvailabilityRequest) (*models.MapResponse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT 
			f.id AS floor_id, f.label, f.floor_number, f.sort_order,
			r.id AS room_id, r.number, r.pos_x, r.pos_y, r.status AS room_status,
			rt.id AS room_type_id, rt.name AS room_type_name,
			CASE
				WHEN r.status = 'inactive' THEN 'inactive'
				WHEN b_in.id IS NOT NULL THEN 'occupied'
				WHEN rb.id IS NOT NULL THEN 'blocked'
				WHEN b_conf.id IS NOT NULL THEN 'pending'
				ELSE 'available'
			END AS availability_state,
			b_in.id AS active_booking_id, 
			b_conf.id AS pending_booking_id, 
			rb.id AS block_id,
			rb.reason::text AS block_reason,
			rb.notes AS block_notes,
			rb.start_date::text AS block_start_date,
			rb.end_date::text AS block_end_date,
			-- Guest info for active booking
			g_in.full_name AS active_guest_name,
			g_in.phone AS active_guest_phone,
			g_in.nationality AS active_guest_nationality,
			b_in.check_in::text AS active_check_in,
			b_in.check_out::text AS active_check_out,
			-- Guest info for pending booking
			g_conf.full_name AS pending_guest_name,
			g_conf.phone AS pending_guest_phone,
			g_conf.nationality AS pending_guest_nationality,
			b_conf.check_in::text AS pending_check_in,
			b_conf.check_out::text AS pending_check_out
		FROM rooms r
		JOIN floors f ON r.floor_id = f.id
		JOIN room_types rt ON r.room_type_id = rt.id
		LEFT JOIN room_blocks rb ON rb.room_id = r.id AND rb.start_date < $2 AND rb.end_date > $1
		LEFT JOIN bookings b_in ON b_in.room_id = r.id AND b_in.status = 'checked_in' AND b_in.check_in < $2 AND b_in.check_out > $1
		LEFT JOIN bookings b_conf ON b_conf.room_id = r.id AND b_conf.status = 'confirmed' AND b_conf.check_in < $2 AND b_conf.check_out > $1
		LEFT JOIN guests g_in ON g_in.id = b_in.guest_id
		LEFT JOIN guests g_conf ON g_conf.id = b_conf.guest_id
		WHERE f.property_id = $3
		ORDER BY f.sort_order ASC, f.floor_number ASC, r.pos_y ASC, r.pos_x ASC`,
		req.DateFrom, req.DateTo, req.PropertyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Agrupación en memoria O(N) para evitar duplicados por LEFT JOIN
	floorMap := make(map[uuid.UUID]*models.FloorMap)
	roomMap := make(map[uuid.UUID]*models.RoomMap)
	var floorOrder []uuid.UUID

	for rows.Next() {
		var fID, rID, rtID uuid.UUID
		var label, rNum, rtName, state string
		var fNum, sOrder, posX, posY int
		var rStatus string
		var abID, pbID, blID *uuid.UUID
		var bReason, bNotes, bStart, bEnd *string

		// Guest fields
		var activeGuestName, activeGuestPhone, activeGuestNationality *string
		var activeCheckIn, activeCheckOut *string
		var pendingGuestName, pendingGuestPhone, pendingGuestNationality *string
		var pendingCheckIn, pendingCheckOut *string

		if err := rows.Scan(&fID, &label, &fNum, &sOrder, &rID, &rNum, &posX, &posY, &rStatus, &rtID, &rtName, &state, &abID, &pbID, &blID,
			&bReason, &bNotes, &bStart, &bEnd,
			&activeGuestName, &activeGuestPhone, &activeGuestNationality, &activeCheckIn, &activeCheckOut,
			&pendingGuestName, &pendingGuestPhone, &pendingGuestNationality, &pendingCheckIn, &pendingCheckOut); err != nil {
			return nil, err
		}

		// Init Floor if new
		if _, exists := floorMap[fID]; !exists {
			floorMap[fID] = &models.FloorMap{ID: fID, Label: label, FloorNumber: fNum, SortOrder: sOrder}
			floorOrder = append(floorOrder, fID)
		}

		// Init Room if new
		if _, exists := roomMap[rID]; !exists {
			roomMap[rID] = &models.RoomMap{
				ID: rID, Number: rNum, PosX: posX, PosY: posY,
				RoomType: models.RoomTypeRef{ID: rtID, Name: rtName},
			}
			floorMap[fID].Rooms = append(floorMap[fID].Rooms, roomMap[rID])
		}

		// Overwrite state/IDs (último registro gana, suficiente para MVP)
		roomMap[rID].Availability = state
		roomMap[rID].ActiveBookingID = abID
		roomMap[rID].PendingBookingID = pbID
		roomMap[rID].BlockID = blID
		roomMap[rID].BlockReason = bReason
		roomMap[rID].BlockNotes = bNotes
		roomMap[rID].BlockStartDate = bStart
		roomMap[rID].BlockEndDate = bEnd

		// Overwrite guest info
		if activeGuestName != nil {
			roomMap[rID].ActiveGuestName = activeGuestName
			roomMap[rID].ActiveGuestPhone = activeGuestPhone
			roomMap[rID].ActiveGuestNationality = activeGuestNationality
			roomMap[rID].ActiveCheckIn = activeCheckIn
			roomMap[rID].ActiveCheckOut = activeCheckOut
		}
		if pendingGuestName != nil {
			roomMap[rID].PendingGuestName = pendingGuestName
			roomMap[rID].PendingGuestPhone = pendingGuestPhone
			roomMap[rID].PendingGuestNationality = pendingGuestNationality
			roomMap[rID].PendingCheckIn = pendingCheckIn
			roomMap[rID].PendingCheckOut = pendingCheckOut
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build response
	floors := make([]models.FloorMap, 0, len(floorOrder))
	for _, fID := range floorOrder {
		floors = append(floors, *floorMap[fID])
	}

	return &models.MapResponse{
		PropertyID: req.PropertyID,
		DateFrom:   req.DateFrom.Format("2006-01-02"),
		DateTo:     req.DateTo.Format("2006-01-02"),
		Floors:     floors,
	}, nil
}

// BatchUpdatePositions - Spec §5.2 (AC-13: Transaccional all-or-nothing)
func (r *RoomRepository) BatchUpdatePositions(ctx context.Context, updates []models.RoomPositionUpdate) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	for _, u := range updates {
		_, err := tx.Exec(ctx, `
			UPDATE rooms SET pos_x = $1, pos_y = $2, updated_at = NOW() WHERE id = $3`,
			u.PosX, u.PosY, u.ID)
		if err != nil {
			return 0, err // Rollback automático por defer
		}
	}

	return len(updates), tx.Commit(ctx)
}
