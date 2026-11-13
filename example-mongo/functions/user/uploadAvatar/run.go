package uploadavatar

import (
	"context"
	"time"

	"example-mongo/db"
	"example-mongo/models"
	"example-mongo/utils/imagehandler"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func init() {
	rpc.Register(rpc.FunctionConfig[Input, Output]{
		Namespace:    Namespace,
		FunctionName: FunctionName,
		ContentType:  ContentType,
		Metadata:     Metadata,
		Middlewares:  Middlewares,
		Handler:      Run,
	})
}

func Run(ctx *rpc.Context, input Input) (*Output, error) {
	if input.Email == "" {
		return nil, rpc.NewError(rpc.ErrValidation, "Email is required")
	}
	if input.Avatar == nil {
		return nil, rpc.NewError(rpc.ErrValidation, "Avatar file is required")
	}

	mCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. Fetch user from MongoDB
	var user models.User
	err := db.Users().FindOne(mCtx, bson.M{"email": input.Email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, rpc.NewError(rpc.ErrNotFound, "User not found")
		}
		return nil, rpc.NewError(rpc.ErrInternal, "Database error: "+err.Error())
	}

	// 2. Process and save uploaded image using imagehandler
	savedPath, err := imagehandler.ProcessImageField(input.Avatar, "avatars", user.Avatar)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInternal, "Failed saving avatar: "+err.Error())
	}

	// 3. Update MongoDB user with new avatar path
	user.Avatar = savedPath
	_, err = db.Users().UpdateOne(mCtx, bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{"avatar": savedPath},
	})
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInternal, "Failed updating database with new avatar: "+err.Error())
	}

	return &Output{
		User:      user,
		AvatarURL: savedPath,
	}, nil
}
