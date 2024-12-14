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
	// Pointer to non-pointer simple copy with nil check
	if objSource.Role != nil {
		objTarget.Role = *objSource.Role
	} else {
		// Handle nil pointer case (use zero value)
		var zeroValue source.UserRole
		objTarget.Role = zeroValue
	}
	// Slice of pointers to slice of structs mapping
if len(objSource.Childrens) > 0 {
	objTarget.Childrens = make([]target.ChildrenDTO, len(objSource.Childrens))
	for i, v := range objSource.Childrens {
		if v != nil {
			objTarget.Childrens[i] = impl.mapChildrenToChildrenDTO(*v)
		}
	}
}
	// Pointer to non-pointer mapping with nil check
	if objSource.CH != nil {
		objTarget.CH = impl.mapChildrenToChildrenDTO(*objSource.CH)
	} else {
		// Handle nil pointer case (use zero value or skip)
		var zeroValue target.ChildrenDTO
		objTarget.CH = zeroValue
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

