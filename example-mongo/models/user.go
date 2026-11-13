package models

type User struct {
	ID        string `bson:"_id,omitempty" json:"id" doc:"Unique user identifier"`
	Email     string `bson:"email" json:"email" doc:"User email address"`
	Name      string `bson:"name" json:"name" doc:"Full name of user"`
	Avatar    string `bson:"avatar,omitempty" json:"avatar,omitempty" doc:"URL to user avatar image"`
	Role      string `bson:"role" json:"role" doc:"User authorization role"`
	CreatedAt int64  `bson:"createdAt" json:"createdAt" doc:"Unix timestamp when account was created"`
}
