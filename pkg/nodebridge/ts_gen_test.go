package nodebridge_test

import (
	"strings"
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/nodebridge"
	"github.com/CellsWeb/crpc-go/pkg/rpc"
	"github.com/CellsWeb/crpc-go/pkg/schema"
)

type ProfileInput struct {
	UserID string `json:"userId" doc:"Unique ID of user"`
}

type ProfileOutput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func TestGenerateTypeScriptDeclarations(t *testing.T) {
	functions := make(map[string]*rpc.FunctionDefinition)

	functions["user.getProfile"] = &rpc.FunctionDefinition{
		Namespace:    "user",
		FunctionName: "getProfile",
		InputSchema:  schema.FromStruct(ProfileInput{}),
		OutputSchema: schema.FromStruct(ProfileOutput{}),
	}

	ts := nodebridge.GenerateTypeScriptDeclarations(functions)

	if !strings.Contains(ts, "export type UserGetProfileInput = {") {
		t.Errorf("Expected UserGetProfileInput type definition, got:\n%s", ts)
	}
	if !strings.Contains(ts, "userId: string;") {
		t.Errorf("Expected userId: string field in TS output, got:\n%s", ts)
	}
	if !strings.Contains(ts, `"user.getProfile": {`) {
		t.Errorf("Expected user.getProfile entry in RpcContractMap, got:\n%s", ts)
	}
}
