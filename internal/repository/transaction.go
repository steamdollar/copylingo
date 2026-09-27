package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// WithinTx runs service work in one transaction on the supplied DB pool.
func WithinTx(
	ctx context.Context,
	db *sqlx.DB,
	work func(*sqlx.Tx) error,
) error {
	tx, err := db.BeginTxx(
		ctx,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"WithinTx begin: %w",
			err,
		)
	}
	defer tx.Rollback()

	if err := work(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf(
			"WithinTx commit: %w",
			err,
		)
	}
	return nil
}
