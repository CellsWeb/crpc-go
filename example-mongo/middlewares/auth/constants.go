package auth

import "github.com/CellsWeb/crpc-go/pkg/schema"

type Options struct {
	Roles    []string `json:"roles,omitempty"`
	Optional bool     `json:"optional,omitempty"`
}

var OptionsSchema = schema.Object(map[string]*schema.SchemaRule{
	"roles":    schema.Optional(schema.Array(schema.String())).Describe("Allowed user roles for this endpoint"),
	"optional": schema.Optional(schema.Boolean()).Describe("Allow unauthenticated access if token is missing"),
})

var HeadersSchema = schema.Object(map[string]*schema.SchemaRule{
	"authorization": schema.Optional(schema.String()).Describe("Bearer JWT token for authentication"),
	"token":         schema.Optional(schema.String()).Describe("Alternative token header"),
})
