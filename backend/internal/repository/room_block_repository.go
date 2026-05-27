package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
)

type RoomBlockRepository struct {
	db *pgxpool.Pool
}

func NewRoomBlockRepository(db *pgxpool.Pool) *RoomBlockRepository {
	return &RoomBlockRepository{db: db}
}

func (r *RoomBlockRepository) Create(ctx context.Context, req *models.CreateRoomBlockRequest) (*models.RoomBlock, error) {
	if req.CreatedBy == uuid.Nil {
		// Fallback para desarrollo/pruebas cuando aún no hay JWT Auth implementado
		_ = r.db.QueryRow(ctx, "SELECT id FROM users LIMIT 1").Scan(&req.CreatedBy)
	}

	var block models.RoomBlock
	err := r.db.QueryRow(ctx, `
		INSERT INTO room_blocks (room_id, created_by, start_date, end_date, reason, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, room_id, created_by, start_date, end_date, reason, notes, created_at, updated_at
	`, req.RoomID, req.CreatedBy, req.StartDate, req.EndDate, req.Reason, req.Notes).Scan(
		&block.ID, &block.RoomID, &block.CreatedBy, &block.StartDate, &block.EndDate,
		&block.Reason, &block.Notes, &block.CreatedAt, &block.UpdatedAt,
	)
	return &block, err
}

// GetOverlapping devuelve bloqueos que solapan con el rango dado.
// Clave para AC-07 y AC-03.
func (r *RoomBlockRepository) GetOverlapping(ctx context.Context, roomID uuid.UUID, start, end time.Time) ([]*models.RoomBlock, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, room_id, created_by, start_date, end_date, reason, notes, created_at, updated_at
		FROM room_blocks
		WHERE room_id = $1 AND start_date < $3 AND end_date > $2
		ORDER BY start_date ASC
	`, roomID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []*models.RoomBlock
	for rows.Next() {
		var b models.RoomBlock
		if err := rows.Scan(&b.ID, &b.RoomID, &b.CreatedBy, &b.StartDate, &b.EndDate, &b.Reason, &b.Notes, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		blocks = append(blocks, &b)
	}
	return blocks, rows.Err()
}

func (r *RoomBlockRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM room_blocks WHERE id = $1`, id)
	return err
}
