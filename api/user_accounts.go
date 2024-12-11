package api

import "github.com/glasskube/cloud/internal/types"

type CreateUserAccountRequest struct {
	Email    string         `json:"email"`
	Name     string         `json:"name"`
	UserRole types.UserRole `json:"userRole"`
}

type UpdateUserAccountRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}
