// Copyright (c) 2021 上海骞云信息科技有限公司. All rights reserved
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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
	LogLevel          string            `yaml:"LOGLEVEL"`
}

type AddPortRequest struct {
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	Lan       string `json:"lan"`
	ClientKey string `json:"clientKey"`
}
