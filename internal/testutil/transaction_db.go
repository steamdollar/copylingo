package testutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"

	"github.com/jmoiron/sqlx"
)

// TransactionDB gives service mocks a real sql transaction boundary without running SQL or a database.
func TransactionDB() *sqlx.DB {
	return sqlx.NewDb(
		sql.OpenDB(transactionConnector{}),
		"test-transaction",
	)
}

type transactionConnector struct{}

func (transactionConnector) Connect(context.Context) (driver.Conn, error) {
	return transactionConnection{}, nil
}

func (transactionConnector) Driver() driver.Driver {
	return transactionDriver{}
}

type transactionDriver struct{}

func (transactionDriver) Open(string) (driver.Conn, error) {
	return transactionConnection{}, nil
}

type transactionConnection struct{}

func (transactionConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("test transaction database does not execute SQL")
}

func (transactionConnection) Close() error { return nil }

func (transactionConnection) Begin() (driver.Tx, error) { return transactionTx{}, nil }

func (transactionConnection) BeginTx(
	context.Context,
	driver.TxOptions,
) (driver.Tx, error) {
	return transactionTx{}, nil
}

type transactionTx struct{}

func (transactionTx) Commit() error   { return nil }
func (transactionTx) Rollback() error { return nil }
