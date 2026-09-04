package users

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID
	PhoneNumber string
	CreatedAt   time.Time
}
