package target

import (
	"github.com/alterasi/stos/example/source"
	"time"
)

type UserDTO struct {
	Name      string
	Age       int
	Role      source.UserRole
	Childrens []ChildrenDTO
	CH        ChildrenDTO
	Gender    string
	Birthday  time.Time
}

type ChildrenDTO struct {
	Name   string
	Age    int
	Role   source.UserRole
	Gender string
}
