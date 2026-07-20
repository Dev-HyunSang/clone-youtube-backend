package service

import (
	"fmt"
	"net/smtp"

	"github.com/dev-hyunsang/clone-youtube-backend/internal/config"
)

type EmailSender interface {
	SendVerificationCode(to, code string) error
}

type smtpEmailSender struct{}

func NewSMTPEmailSender() EmailSender {
	return &smtpEmailSender{}
}

func (s *smtpEmailSender) SendVerificationCode(to, code string) error {
	host := config.GetEnv("SMTP_HOST")
	addr := fmt.Sprintf("%s:%s", host, config.GetEnv("SMTP_PORT"))
	auth := smtp.PlainAuth("", config.GetEnv("SMTP_USERNAME"), config.GetEnv("SMTP_PASSWORD"), host)

	msg := fmt.Appendf(nil, "To: %s\r\nSubject: 이메일 인증\r\n\r\n인증 코드: %s (5분 내 입력해주세요)", to, code)

	return smtp.SendMail(addr, auth, config.GetEnv("SMTP_FROM"), []string{to}, msg)
}
