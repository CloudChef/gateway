package main

type ListenerConfig struct {
	url       string
	port      int
	clientKey string
	sslPort   int
}

type RegisterResponse struct {
	Port    int    `json:"port"`
	Ip      string `json:"ip"`
	SslPort int    `json:"sslPort"`
}

type ProxyConfig struct {
	ControllerAddress string            `yaml:"CONTROLLER_ADDRESS"`
	ClientKey         string            `yaml:"CLIENT_KEY"`
	LogPath           string            `yaml:"LOG_PATH"`
	DefaultService    map[string]string `yaml:"DEFAULT_SERVICE"`
	VERSION           string            `yaml:"VERSION"`
	BUILD_ID          string            `yaml:"BUILD_ID"`
	BUILD_REF         string            `yaml:"BUILD_REF"`
}

type AddPortRequest struct {
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	Lan       string `json:"lan"`
	ClientKey string `json:"clientKey"`
}
