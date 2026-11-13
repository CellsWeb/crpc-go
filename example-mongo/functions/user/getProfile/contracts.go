package getprofile

type Input struct {
	ID string `json:"id" doc:"Target resource identifier"`
}

type Output struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}
