package main

import (
	"log"
	"net"
	"net/smtp"
	"time"
)

var emailTimer *time.Timer
var emailBuffer string

func debouncedSendEmail(to string, body string, config MadnsConfig) {
	if emailTimer == nil {
		emailBuffer = emailBuffer + body
		emailDelay := time.Duration(config.SMTPDelay)
		if emailDelay == 0 {
			emailDelay = 60 // default to 60 second email delay
		}
		emailTimer = time.NewTimer(emailDelay * time.Second)
		go func() {
			<-emailTimer.C
			emailTimer.Stop()
			smtpSend(to, emailBuffer, config)
			emailBuffer = ""
			emailTimer = nil
		}()
	} else {
		emailBuffer = emailBuffer + body
	}
}

func smtpSend(to string, body string, config MadnsConfig) {
	from := config.SMTPUser
	pass := config.SMTPPassword
	authhost, _, err := net.SplitHostPort(config.SMTPServer)
	if err != nil {
		log.Printf("SMTPServer field syntax error: %s", err)
		return
	}
	msg := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: Madns Alert\n\n" +
		body

	err = smtp.SendMail(config.SMTPServer,
		smtp.PlainAuth("", from, pass, authhost),
		from, []string{to}, []byte(msg))

	if err != nil {
		log.Printf("SMTP error: %s", err)
		return
	}

	log.Print("sent email to " + to)
}
