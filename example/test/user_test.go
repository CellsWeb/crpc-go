package test

import (
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/rpc"

	_ "example/functions/user/getProfile"
	_ "example/functions/user/updateProfile"
	_ "example/middlewares/auth"
	getProfile "example/functions/user/getProfile"
	updateProfile "example/functions/user/updateProfile"
)

func setupTestApp(t *testing.T) *rpc.CoreRPC {
	app, err := rpc.New(rpc.Config{
		RpcPath: "/rpc",
		CORS: &rpc.CorsConfig{
			AllowOrigins: []string{"*"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to initialize CoreRPC: %v", err)
	}

	app.UseGlobalRegistry()
	return app
}

func TestExampleUserGetProfile(t *testing.T) {
	app := setupTestApp(t)

	caller := rpc.NewTestCaller(app, rpc.TestOptions{
		Headers: map[string]string{
			"authorization": "Bearer valid_token",
		},
	})

	var output getProfile.Output
	err := caller.Call("user.getProfile", getProfile.Input{ID: "usr_101"}, &output)
	if err != nil {
		t.Fatalf("Expected successful Call, got %v", err)
	}

	if output.ID != "usr_101" {
		t.Errorf("Expected ID 'usr_101', got %q", output.ID)
	}
	if output.Status != "active" {
		t.Errorf("Expected status 'active', got %q", output.Status)
	}
}

func TestExampleUserUpdateProfile(t *testing.T) {
	app := setupTestApp(t)

	caller := rpc.NewTestCaller(app)

	uploaded := rpc.NewUploadedFile("avatar.png", "image/png", []byte("fake_image_bytes"), "")

	var output updateProfile.Output
	err := caller.Call("user.updateProfile", map[string]any{
		"name":   "Minhazur Rahman",
		"avatar": uploaded,
	}, &output)

	if err != nil {
		t.Fatalf("Expected successful Call with UploadedFile, got %v", err)
	}

	if !output.Success {
		t.Errorf("Expected Success true, got false")
	}
	if output.AvatarURL != "/uploads/avatar.png" {
		t.Errorf("Expected AvatarURL '/uploads/avatar.png', got %q", output.AvatarURL)
	}
}
