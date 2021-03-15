// Copyright (c) 2021 Qianyun, Inc. All rights reserved.
package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
)

type HttpClient struct {
	token             string
	controllerAddress string
	clientKey         string
}

func NewHttpClient(controllerAddress, token, clientKey string) *HttpClient {
	return &HttpClient{token: token, controllerAddress: controllerAddress, clientKey: clientKey}
}

func (httpClient *HttpClient) SendRequest(url, method string, body io.Reader) (*http.Response, error) {
	token := httpClient.token
	if strings.Contains(httpClient.controllerAddress, "http") {
		url = httpClient.controllerAddress + url
	} else {
		url = "http://" + httpClient.controllerAddress + url
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return &http.Response{}, err
	}
	req.Header.Add("Content-Type", "application/json;charset=UTF-8")
	req.Header.Add("CloudChef-Authenticate", token)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: tr}
	log.Info("Request url:", url)
	log.Info("Request method:", method)
	log.Info("Request body:", body)
	res, err := client.Do(req)
	log.Info("Response:", res)
	if err != nil {
		return res, err
	}
	return res, err
}

func (httpClient *HttpClient) AddPort(name, lan string) error {
	url := "/cloud-proxies/request-connection"
	method := "POST"
	req := AddPortRequest{Name: name, Protocol: "tcp", Lan: lan, ClientKey: httpClient.clientKey}
	jsonReq, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = httpClient.SendRequest(url, method, bytes.NewReader(jsonReq))
	if err != nil {
		return err
	}
	return nil
}

func (httpClient *HttpClient) Register() (RegisterResponse, error) {
	var registerResponse RegisterResponse
	var lans = map[string]string{}
	var req = map[string]interface{}{}
	var hostName string
	var ip []string
	hostName, err := os.Hostname()
	if err != nil {
		log.Warn("Get HostName failed...")
	}
	addrs, _ := net.InterfaceAddrs()
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = append(ip, ipnet.IP.String())
			}
		}
	}
	url := "/cloud-proxies/register"
	for k, v := range proxyConfig.DefaultService {
		lans[k] = v
	}
	req["clientKey"] = httpClient.clientKey
	req["lans"] = lans
	req["hostName"] = hostName
	req["ip"] = strings.Join(ip, ",")
	req["version"] = proxyConfig.VERSION + "-" + proxyConfig.BUILD_ID
	jsonReq, _ := json.Marshal(req)
	res, err := httpClient.SendRequest(url, "POST", bytes.NewBuffer(jsonReq))

	if err != nil {
		log.Error("Register Failed...", err)
		return registerResponse, err
	}
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return registerResponse, err
	}
	log.Info("Get Response from controller: ", string(resBody))
	err = json.Unmarshal(resBody, &registerResponse)
	if err != nil {
		return registerResponse, err
	}
	return registerResponse, nil
}
