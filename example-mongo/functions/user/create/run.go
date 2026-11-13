package create

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"example-mongo/db"
	"example-mongo/models"

	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func init() {
	rpc.Register(rpc.FunctionConfig[Input, Output]{
		Namespace:    "user",
		FunctionName: "create",
		ContentType:  "application/json",
		Middlewares:  Middlewares,
		Handler:      Run,
	})
}

func Run(ctx *rpc.Context, input Input) (*Output, error) {
	if input.Email == "" || input.Name == "" {
		return nil, rpc.NewError(rpc.ErrValidation, "Email and Name are required")
	}

	role := input.Role
	if role == "" {
		role = "user"
	}

	// Generate clean ID
	idBytes := make([]byte, 8)
	_, _ = rand.Read(idBytes)
	userID := "usr_" + hex.EncodeToString(idBytes)

	user := models.User{
		ID:        userID,
		Email:     input.Email,
		Name:      input.Name,
		Role:      role,
		CreatedAt: time.Now().Unix(),
	}

	mCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Check if email already exists
	var existing models.User
	err := db.Users().FindOne(mCtx, bson.M{"email": input.Email}).Decode(&existing)
	if err == nil {
		return nil, rpc.NewError(rpc.ErrValidation, "Email already registered")
	}

	_, err = db.Users().InsertOne(mCtx, user)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInternal, "Failed to persist user in database")
	}

	return &Output{User: user}, nil
}
