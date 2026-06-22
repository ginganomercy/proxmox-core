package services

import (
	"fmt"
	"net/smtp"

	"cbt-core-api/config"
)

type EmailService interface {
	SendActivationCode(toEmail, username, orderName, activationCode string) error
	SendPasswordReset(toEmail, username, resetToken string) error
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

func (s *emailServiceImpl) SendPasswordReset(toEmail, username, resetToken string) error {
	host := config.Env.SMTPHost
	port := config.Env.SMTPPort
	user := config.Env.SMTPUser
	pass := config.Env.SMTPPass

	if user == "" || pass == "" {
		fmt.Printf("===================================================\n")
		fmt.Printf("[MOCK EMAIL] To: %s\n", toEmail)
		fmt.Printf("[MOCK EMAIL] Password Reset Link: https://cloud-dashboard.pbjt.web.id/reset-password?token=%s\n", resetToken)
		fmt.Printf("===================================================\n")
		return nil
	}

	auth := smtp.PlainAuth("", user, pass, host)

	subject := "Password Reset Request - Cloud Baja Tegal"
	resetLink := fmt.Sprintf("https://cloud-dashboard.pbjt.web.id/reset-password?token=%s", resetToken)
	body := fmt.Sprintf("Hello %s,\n\nYou recently requested to reset your password for your Cloud Baja Tegal account.\n\nClick the link below to reset it:\n%s\n\nIf you did not request a password reset, please ignore this email or reply to let us know. This password reset is only valid for the next 1 hour.\n\nThanks,\nThe CBT Team", username, resetLink)

	msg := []byte("To: " + toEmail + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")

	err := smtp.SendMail(host+":"+port, auth, user, []string{toEmail}, msg)
	return err
}
