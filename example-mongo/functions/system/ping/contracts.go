package ping

type Input struct {
	Message string `json:"message,omitempty"`
}

type Output struct {
	Reply     string `json:"reply"`
	Timestamp int64  `json:"timestamp"`
}
