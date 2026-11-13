package hello

type Input struct {
	Name string `json:"name" doc:"Your full name for personalized greeting"`
}

type Output struct {
	Greeting string `json:"greeting" doc:"Personalized greeting response message"`
}
