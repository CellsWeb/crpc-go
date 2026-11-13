package updateprofile

import (
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.Register(rpc.FunctionConfig[Input, Output]{
		Namespace:    "user",
		FunctionName: "updateProfile",
		ContentType:  "multipart/form-data",
		Middlewares:  Middlewares,
		Handler:      Run,
	})
}

func Run(ctx *rpc.Context, input Input) (*Output, error) {
	_ = time.Now()
	avatarURL := ""
	if input.Avatar != nil {
		avatarURL = "/uploads/" + input.Avatar.Filename()
	}
	return &Output{
		Success:   true,
		AvatarURL: avatarURL,
	}, nil
}
