package updateprofile

import "github.com/CellsWeb/crpc-go/pkg/rpc"

type Input struct {
	Name   string            `json:"name" doc:"User display name"`
	Avatar *rpc.UploadedFile `json:"avatar" doc:"Uploaded avatar image"`
}

type Output struct {
	Success   bool   `json:"success"`
	AvatarURL string `json:"avatarUrl"`
}
