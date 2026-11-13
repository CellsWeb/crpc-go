package create

import "example-mongo/models"

type Input struct {
	Email string `json:"email" doc:"User email address"`
	Name  string `json:"name" doc:"Full user name"`
	Role  string `json:"role,omitempty" doc:"User role (admin, user)"`
}

type Output struct {
	User models.User `json:"user"`
}
