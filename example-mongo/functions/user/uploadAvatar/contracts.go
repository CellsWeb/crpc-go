package uploadavatar

import (
	"example-mongo/models"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

const (
	Namespace    = "user"
	FunctionName = "uploadAvatar"
	ContentType  = "multipart/form-data"
)

var Metadata = rpc.FunctionMetadata{
	Description: "Uploads and updates a user's avatar image via multipart form",
	Tags:        []string{"user", "avatar", "upload"},
}

type Input struct {
	Email  string            `json:"email" doc:"User email address"`
	Avatar *rpc.UploadedFile `json:"avatar" doc:"Binary avatar image file"`
}

type Output struct {
	User      models.User `json:"user"`
	AvatarURL string      `json:"avatarUrl"`
}
