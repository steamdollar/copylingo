package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/model"
)

// SessionMaterialRepository manages the session_materials join table.
type SessionMaterialRepository struct {
	db *sqlx.DB
}

func NewSessionMaterialRepository(db *sqlx.DB) *SessionMaterialRepository {
	return &SessionMaterialRepository{db: db}
}

// CreateSessionMaterials inserts multiple session material entries.
func (r *SessionMaterialRepository) CreateSessionMaterials(
	ctx context.Context,
	sessionMaterials []model.SessionMaterial,
) error {
	if len(sessionMaterials) == 0 {
		return nil
	}

	if _, err := r.db.NamedExecContext(ctx, `
		INSERT INTO session_materials (session_id, material_id, material_order)
		VALUES (:session_id, :material_id, :material_order)
	`, sessionMaterials); err != nil {
		return fmt.Errorf("SessionMaterialRepository.CreateSessionMaterials count=%d: %w", len(sessionMaterials), err)
	}

	return nil
}
