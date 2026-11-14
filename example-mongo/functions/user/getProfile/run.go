package getprofile

import (
	"time"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
)

func init() {
	rpc.Register(rpc.FunctionConfig[Input, Output]{
		Namespace:    "user",
		FunctionName: "getProfile",
		ContentType:  "application/json",
		Middlewares:  Middlewares,
		Handler:      Run,
	})
}

func Run(ctx *rpc.Context, input Input) (*Output, error) {
	userID := input.ID

	// Extract authenticated user namespaced under "auth" middleware
	if authVal, ok := ctx.Get("auth"); ok {
		if authMap, ok := authVal.(map[string]any); ok {
			if userMap, ok := authMap["user"].(map[string]any); ok {
				if id, ok := userMap["id"].(string); ok && userID == "" {
					userID = id
				}
			}
		}
	}

	return &Output{
		ID:        userID,
		Status:    "active",
		Timestamp: time.Now().Unix(),
	}, nil
}
