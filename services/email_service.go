package services

import (
	"fmt"
	"net/smtp"

	"cbt-core-api/config"
)

type EmailService interface {
	SendActivationCode(toEmail, username, orderName, activationCode string) error
}

type emailServiceImpl struct{}

func NewEmailService() EmailService {
	return &emailServiceImpl{}
}

func (s *emailServiceImpl) SendActivationCode(toEmail, username, orderName, activationCode string) error {
	host := config.Env.SMTPHost
	port := config.Env.SMTPPort
	user := config.Env.SMTPUser
	pass := config.Env.SMTPPass

	// If no SMTP configured, we just log it and simulate success
	if user == "" || pass == "" {
		fmt.Printf("===================================================\n")
		fmt.Printf("[MOCK EMAIL] To: %s\n", toEmail)
		fmt.Printf("[MOCK EMAIL] Activation Code for %s: %s\n", orderName, activationCode)
		fmt.Printf("===================================================\n")
		return nil
	}

	auth := smtp.PlainAuth("", user, pass, host)

	subject := "Your CBT Activation Code for " + orderName
	body := fmt.Sprintf("Hello %s,\n\nYour payment has been confirmed! Here is your Activation Code to provision your VM: %s\n\nPlease enter this code on the CBT Dashboard to start your server.\n\nThank you for choosing Cloud Baja Tegal.", username, activationCode)

	msg := []byte("To: " + toEmail + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")

	err := smtp.SendMail(host+":"+port, auth, user, []string{toEmail}, msg)
	return err
}
