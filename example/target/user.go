package target

import (
	"github.com/alterasi/stos/example/source"
	"time"
)

type UserDTO struct {
	Name     string
	Age      int
	Role     source.UserRole
	Children []*UserDTO
	Gender   string
	Birthday time.Time
}
