package repository

import (
	"context"
	"fmt"

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
	// Auto-generate number if empty
	if req.Number == "" {
		var floorNum int
		err := r.db.QueryRow(ctx, `SELECT floor_number FROM floors WHERE id = $1`, req.FloorID).Scan(&floorNum)
		if err != nil {
			return nil, err
		}

		rows, err := r.db.Query(ctx, `SELECT number FROM rooms WHERE floor_id = $1`, req.FloorID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		existingNums := make(map[string]bool)
		for rows.Next() {
			var num string
			if err := rows.Scan(&num); err == nil {
				existingNums[num] = true
			}
		}

		nextNum := ""
		for yy := 1; yy <= 99; yy++ {
			candidate := fmt.Sprintf("%d%02d", floorNum, yy)
			if !existingNums[candidate] {
				nextNum = candidate
				break
			}
		}
		if nextNum == "" {
			return nil, fmt.Errorf("no available room numbers on floor %d", floorNum)
		}
		req.Number = nextNum
	}

	var room models.Room
	err := r.db.QueryRow(ctx, `
		INSERT INTO rooms (floor_id, property_id, room_type_id, number, status, pos_x, pos_y) 
		VALUES ($1, (SELECT property_id FROM floors WHERE id = $1), $2, $3, $4, $5, $6) 
		RETURNING id, floor_id, property_id, room_type_id, number, status, pos_x, pos_y, false AS has_bookings, created_at, updated_at`,
		req.FloorID, req.RoomTypeID, req.Number, req.Status, req.PosX, req.PosY,
	).Scan(
		&room.ID, &room.FloorID, &room.PropertyID, &room.RoomTypeID, &room.Number, &room.Status,
		&room.PosX, &room.PosY, &room.HasBookings, &room.CreatedAt, &room.UpdatedAt,
	)
	return &room, err
}

func (r *RoomRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Room, error) {
	var room models.Room
	err := r.db.QueryRow(ctx, `
		SELECT id, floor_id, property_id, room_type_id, number, status, pos_x, pos_y, 
		       (SELECT EXISTS (SELECT 1 FROM bookings WHERE room_id = $1)) AS has_bookings,
		       created_at, updated_at 
		FROM rooms WHERE id = $1`, id).Scan(
		&room.ID, &room.FloorID, &room.PropertyID, &room.RoomTypeID, &room.Number, &room.Status,
		&room.PosX, &room.PosY, &room.HasBookings, &room.CreatedAt, &room.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &room, err
}

func (r *RoomRepository) ListByFloor(ctx context.Context, floorID uuid.UUID) ([]*models.Room, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, floor_id, property_id, room_type_id, number, status, pos_x, pos_y, 
		       (SELECT EXISTS (SELECT 1 FROM bookings WHERE room_id = rooms.id)) AS has_bookings,
		       created_at, updated_at 
		FROM rooms WHERE floor_id = $1 ORDER BY number ASC`, floorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*models.Room
	for rows.Next() {
		var room models.Room
		if err := rows.Scan(
			&room.ID, &room.FloorID, &room.PropertyID, &room.RoomTypeID, &room.Number, &room.Status,
			&room.PosX, &room.PosY, &room.HasBookings, &room.CreatedAt, &room.UpdatedAt,
		); err != nil {
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
		WHERE id = $3 
		RETURNING id, floor_id, property_id, room_type_id, number, status, pos_x, pos_y, 
		          (SELECT EXISTS (SELECT 1 FROM bookings WHERE room_id = $3)) AS has_bookings,
		          created_at, updated_at`,
		posX, posY, id).Scan(
		&room.ID, &room.FloorID, &room.PropertyID, &room.RoomTypeID, &room.Number, &room.Status,
		&room.PosX, &room.PosY, &room.HasBookings, &room.CreatedAt, &room.UpdatedAt,
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
				-- Jerarquía de prioridad: inactive > occupied > blocked > pending >
				-- cleaning > available. "cleaning" es estado operacional de la
				-- habitación (housekeeping en curso); no debe ser vendible.
				WHEN r.status = 'inactive' THEN 'inactive'
				WHEN b_in.id IS NOT NULL THEN 'occupied'
				WHEN rb.id IS NOT NULL THEN 'blocked'
				WHEN b_conf.id IS NOT NULL THEN 'pending'
				WHEN r.status = 'cleaning' THEN 'cleaning'
				ELSE 'available'
			END AS availability_state,
			(SELECT EXISTS (SELECT 1 FROM bookings WHERE room_id = r.id)) AS has_bookings,
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
		-- room_blocks: date-bound (mantaintenance/owner use son rangos planificados)
		LEFT JOIN room_blocks rb ON rb.room_id = r.id AND rb.start_date < $2 AND rb.end_date > $1
		-- b_in (checked_in): estado operacional ACTUAL. NO se filtra por fechas: un
		-- huésped físicamente en la habitación ocupa el room sea cual sea el rango
		-- consultado. Una reserva con check-in el 2 jun debe verse como occupied
		-- aunque consultemos el mapa para 19-20 jun (BT-17).
		LEFT JOIN bookings b_in ON b_in.room_id = r.id AND b_in.status = 'checked_in'
		-- b_conf (confirmed): pendientes de check-in, sí filtran por fecha (plan futuro)
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
		var hasBookings bool
		var abID, pbID, blID *uuid.UUID
		var bReason, bNotes, bStart, bEnd *string

		// Guest fields
		var activeGuestName, activeGuestPhone, activeGuestNationality *string
		var activeCheckIn, activeCheckOut *string
		var pendingGuestName, pendingGuestPhone, pendingGuestNationality *string
		var pendingCheckIn, pendingCheckOut *string

		if err := rows.Scan(&fID, &label, &fNum, &sOrder, &rID, &rNum, &posX, &posY, &rStatus, &rtID, &rtName, &state, &hasBookings, &abID, &pbID, &blID,
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
				RoomType:    models.RoomTypeRef{ID: rtID, Name: rtName},
				HasBookings: hasBookings,
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

// Update - Actualización parcial de un room. Acepta cualquier subconjunto de los
// campos editables de models.UpdateRoomRequest. Se usa desde el InventoryService
// para transicionar el estado operacional (cleaning → active, etc.) sin tener
// que conocer el SQL de UPDATE directamente.
func (r *RoomRepository) Update(ctx context.Context, id uuid.UUID, req *models.UpdateRoomRequest) (*models.Room, error) {
	query := "UPDATE rooms SET "
	args := []interface{}{}
	idx := 1

	if req.RoomTypeID != nil {
		query += fmt.Sprintf("room_type_id = $%d, ", idx)
		args = append(args, *req.RoomTypeID)
		idx++
	}
	if req.Number != nil {
		query += fmt.Sprintf("number = $%d, ", idx)
		args = append(args, *req.Number)
		idx++
	}
	if req.Status != nil {
		query += fmt.Sprintf("status = $%d, ", idx)
		args = append(args, *req.Status)
		idx++
	}
	if req.PosX != nil {
		query += fmt.Sprintf("pos_x = $%d, ", idx)
		args = append(args, *req.PosX)
		idx++
	}
	if req.PosY != nil {
		query += fmt.Sprintf("pos_y = $%d, ", idx)
		args = append(args, *req.PosY)
		idx++
	}

	if len(args) == 0 {
		return r.GetByID(ctx, id)
	}

	query += fmt.Sprintf("updated_at = NOW() WHERE id = $%d RETURNING id, floor_id, room_type_id, number, status, pos_x, pos_y, created_at, updated_at", idx)
	args = append(args, id)

	var room models.Room
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&room.ID, &room.FloorID, &room.RoomTypeID, &room.Number, &room.Status,
		&room.PosX, &room.PosY, &room.CreatedAt, &room.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &room, err
}

func (r *RoomRepository) Delete(ctx context.Context, id uuid.UUID) error {
	var hasBookings bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM bookings WHERE room_id = $1)
	`, id).Scan(&hasBookings)
	if err != nil {
		return err
	}

	if hasBookings {
		return fmt.Errorf("cannot delete room with booking history")
	}

	commandTag, err := r.db.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, id)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *RoomRepository) ListRoomTypes(ctx context.Context, propertyID uuid.UUID) ([]*models.RoomType, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, property_id, name, max_occupancy, created_at, updated_at
		FROM room_types
		WHERE property_id = $1
		ORDER BY name ASC
	`, propertyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roomTypes := make([]*models.RoomType, 0)
	for rows.Next() {
		var roomType models.RoomType
		if err := rows.Scan(
			&roomType.ID,
			&roomType.PropertyID,
			&roomType.Name,
			&roomType.MaxOccupancy,
			&roomType.CreatedAt,
			&roomType.UpdatedAt,
		); err != nil {
			return nil, err
		}
		roomTypes = append(roomTypes, &roomType)
	}

	return roomTypes, rows.Err()
}
