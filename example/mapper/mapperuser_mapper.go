package mapper

import (
"github.com/alterasi/stos/example/source"
"github.com/alterasi/stos/example/target"
)

type mapperUserImpl struct {}

func NewMapperUserImpl() MapperUser {
	return &mapperUserImpl{}
}

func (impl *mapperUserImpl) Convert(objSource source.User) target.UserDTO {
	objTarget := target.UserDTO{}
	objTarget.Name = objSource.Name
	objTarget.Age = objSource.Age
	objTarget.Gender = objSource.Gender
	if len(objSource.Children) > 0 {
		objTarget.Children = make([]target.ChildrenDTO, len(objSource.Children))
		for i, v := range objSource.Children {
			objTarget.Children[i] = impl.mapChildrenToChildrenDTO(v)
		}
	}
	objTarget.Birthday = objSource.Birthday
	return objTarget
}

func (impl *mapperUserImpl) mapChildrenToChildrenDTO(objSource source.Children) target.ChildrenDTO {
	objTarget := target.ChildrenDTO{}
	objTarget.Name = objSource.Name
	objTarget.Age = objSource.Age
	objTarget.Gender = objSource.Gender
	objTarget.Role = objSource.Role

	return objTarget
}

