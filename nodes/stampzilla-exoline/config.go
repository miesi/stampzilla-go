package main

type Config struct {
	Interval  string
	Host      string
	Variables []Variables
}

type Variables struct {
	Name       string `json:"name"`
	LoadNumber int    `json:"loadNumber"`
	Cell       int    `json:"cell"`
	Type       string `json:"type"`
	Write      bool   `json:"write"`
}

func NewConfig() *Config {
	return &Config{}
}
