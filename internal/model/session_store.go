package model

import "errors"

var (
	ErrSessionStoreNotFound = errors.New("session state not found")
	ErrSessionStoreCorrupt  = errors.New("session state is corrupt")
)
