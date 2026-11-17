package get

import (
	"context"
	"time"

	"example-mongo/db"
	"example-mongo/models"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func init() {
	rpc.Register(rpc.FunctionConfig[Input, Output]{
		Namespace:    "user",
		FunctionName: "get",
		ContentType:  "application/json",
		Metadata: rpc.FunctionMetadata{
			Description: "Retrieves user profile by ID or email address",
			Tags:        []string{"user", "profile"},
		},
		Middlewares: Middlewares,
		Handler:     Run,
	})
}

func Run(ctx *rpc.Context, input Input) (*Output, error) {
	if input.ID == "" && input.Email == "" {
		return nil, rpc.NewError(rpc.ErrValidation, "Either ID or Email must be provided")
	}

	filter := bson.M{}
	if input.ID != "" {
		filter["_id"] = input.ID
	} else {
		filter["email"] = input.Email
	}

	mCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var user models.User
	err := db.Users().FindOne(mCtx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, rpc.NewError(rpc.ErrNotFound, "User not found")
		}
		return nil, rpc.NewError(rpc.ErrInternal, "Database error: "+err.Error())
	}

	return &Output{User: user}, nil
}
