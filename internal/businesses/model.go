package businesses

import (
	"time"

	"github.com/google/uuid"
)

type Business struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	CreatedAt time.Time
}
