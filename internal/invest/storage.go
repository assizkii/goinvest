package invest

import (
	"errors"
)

// ErrNotFound is substituted in storage implementation and prevent abstraction leakage
// for example, it can replace sql.ErrNoRows.
var ErrNotFound = errors.New("record was not found in database")

// Storage abstracts database interactions for entities.
type Storage interface {
}
