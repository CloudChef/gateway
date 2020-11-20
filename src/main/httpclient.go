package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
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
	url = "http://" + httpClient.controllerAddress + url
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return &http.Response{}, err
	}
	req.Header.Add("Content-Type", "application/json;charset=UTF-8")
	req.Header.Add("CloudChef-Authenticate", token)
	client := http.Client{}
	log.Println("Request url:", url)
	log.Println("Request method:", method)
	log.Println("Request body:", body)
	res, err := client.Do(req)
	log.Println("Response:", res)
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
	url := "/cloud-proxies/register"
	for k, v := range proxyConfig.DefaultService {
		lans[k] = v
	}
	req["clientKey"] = httpClient.clientKey
	req["lans"] = lans
	jsonReq, _ := json.Marshal(req)
	res, err := httpClient.SendRequest(url, "POST", bytes.NewBuffer(jsonReq))

	if err != nil {
		log.Println("Register Failed...", err)
		return registerResponse, err
	}
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return registerResponse, err
	}
	log.Printf("Get Response from controller: %v", string(resBody))
	err = json.Unmarshal(resBody, &registerResponse)
	if err != nil {
		return registerResponse, err
	}
	return registerResponse, nil
}
