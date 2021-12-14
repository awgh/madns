package main

import (
	"io/ioutil"
	"bytes"
	"log"
	"net/http"
	"encoding/json"
)

type SlackMessage struct {
	Text	string	`json:"text"`
}

func sendSlackMessage(webhook string, body string) {

	msg, err := json.Marshal(SlackMessage{Text: body})

	if err != nil {
		log.Printf("Failed to marshal slack message body: %s", err)
		return
	}

	req, err := http.NewRequest("POST", webhook, bytes.NewBuffer(msg))

	if err != nil {
		log.Printf("Failed to create request: %s", err)
		return
	}

	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		log.Printf("Failed to send request: %s", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := ioutil.ReadAll(resp.Body)
		log.Printf("HTTP request failed with status %s, response %s", resp.Status, string(respBody))
		return
	}

}
