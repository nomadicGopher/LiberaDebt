package main

type OllamaRequest struct {
	Url     string  `json:"url"`
	Headers Headers `json:"headers"`
	Data    Data    `json:"data"`
}

type Headers struct {
	ContentType string `json:"Content-Type"`
}

type Data struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}
