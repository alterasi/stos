package mapper

import (
	"github.com/alterasi/stos/example/source"
	"github.com/alterasi/stos/example/target"
)

type MapperUser interface {
	Convert(source source.User) (target target.UserDTO)
}
