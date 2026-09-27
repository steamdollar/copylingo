package testutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"

	"github.com/jmoiron/sqlx"
)

// TransactionDB gives service mocks a real database/sql transaction boundary
// without executing SQL or depending on a running database.
func TransactionDB() *sqlx.DB {
	return sqlx.NewDb(
		sql.OpenDB(transactionConnector{}),
		"test-transaction",
	)
}

type transactionConnector struct{}

func (transactionConnector) Connect(context.Context) (driver.Conn, error) {
	return transactionConn{}, nil
}

func (transactionConnector) Driver() driver.Driver {
	return transactionDriver{}
}

type transactionDriver struct{}

func (transactionDriver) Open(string) (driver.Conn, error) {
	return transactionConn{}, nil
}

type transactionConn struct{}

func (transactionConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("test transaction database does not execute SQL")
}

func (transactionConn) Close() error { return nil }

func (transactionConn) Begin() (driver.Tx, error) { return transactionTx{}, nil }

func (transactionConn) BeginTx(
	context.Context,
	driver.TxOptions,
) (driver.Tx, error) {
	return transactionTx{}, nil
}

type transactionTx struct{}

func (transactionTx) Commit() error   { return nil }
func (transactionTx) Rollback() error { return nil }
