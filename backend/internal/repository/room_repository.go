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
		RETURNING id, floor_id, room_type_id, number, status, pos_x, pos_y, created_at, updated_at
	`, req.FloorID, req.RoomTypeID, req.Number, req.Status, req.PosX, req.PosY).Scan(
		&room.ID, &room.FloorID, &room.RoomTypeID, &room.Number, &room.Status,
		&room.PosX, &room.PosY, &room.CreatedAt, &room.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *RoomRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Room, error) {
	var room models.Room
	err := r.db.QueryRow(ctx, `
		SELECT id, floor_id, room_type_id, number, status, pos_x, pos_y, created_at, updated_at
		FROM rooms WHERE id = $1
	`, id).Scan(
		&room.ID, &room.FloorID, &room.RoomTypeID, &room.Number, &room.Status,
		&room.PosX, &room.PosY, &room.CreatedAt, &room.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *RoomRepository) ListByFloor(ctx context.Context, floorID uuid.UUID) ([]*models.Room, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, floor_id, room_type_id, number, status, pos_x, pos_y, created_at, updated_at
		FROM rooms WHERE floor_id = $1 ORDER BY number ASC
	`, floorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*models.Room
	for rows.Next() {
		var room models.Room
		err := rows.Scan(
			&room.ID, &room.FloorID, &room.RoomTypeID, &room.Number, &room.Status,
			&room.PosX, &room.PosY, &room.CreatedAt, &room.UpdatedAt,
		)
		if err != nil {
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
		RETURNING id, floor_id, room_type_id, number, status, pos_x, pos_y, created_at, updated_at
	`, posX, posY, id).Scan(
		&room.ID, &room.FloorID, &room.RoomTypeID, &room.Number, &room.Status,
		&room.PosX, &room.PosY, &room.CreatedAt, &room.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &room, nil
}
