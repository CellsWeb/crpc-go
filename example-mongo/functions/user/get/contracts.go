package get

import (
	"example-mongo/models"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

const (
	Namespace    = "user"
	FunctionName = "get"
	ContentType  = "application/json"
)

var Metadata = rpc.FunctionMetadata{
	Description: "Retrieves user profile by ID or email address",
	IsPublic:    true,
	Tags:        []string{"user", "profile"},
}

type Input struct {
	ID    string `json:"id,omitempty" doc:"User ID"`
	Email string `json:"email,omitempty" doc:"User email"`
}

type Output struct {
	User models.User `json:"user"`
}
