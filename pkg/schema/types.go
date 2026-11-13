package schema

// ValidationErrorDetail represents a single field-level schema validation failure
type ValidationErrorDetail struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// FileValidator is the interface required for multipart file validation
type FileValidator interface {
	Filename() string
	Size() int64
	MimeType() string
}
