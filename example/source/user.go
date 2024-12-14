package source

import "time"

type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleManager  UserRole = "manager"
	RoleEmployee UserRole = "employee"
	RoleGuest    UserRole = "guest"
)

type User struct {
	Name     string
	Age      int
	Gender   string
	WifeName string
	Role     UserRole
	Children []*User
	Birthday time.Time
}