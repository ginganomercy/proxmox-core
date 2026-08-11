package services

import (
	"fmt"
	"net/smtp"

	"cbt-core-api/config"
)

type EmailService interface {
	SendPasswordReset(toEmail, username, resetToken string) error
	SendVMProvisioningNotification(toEmail, username, orderName string, cores, memory, storage int) error
}

type emailServiceImpl struct{}

func NewEmailService() EmailService {
	return &emailServiceImpl{}
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

	resetLink := fmt.Sprintf("https://cloud-dashboard.pbjt.web.id/reset-password?token=%s", resetToken)
	
	htmlBody := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<meta charset="UTF-8">
		<style>
			body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f8fafc; margin: 0; padding: 20px; color: #334155; }
			.container { max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 16px; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1); }
			.header { background: linear-gradient(135deg, #ef4444 0%%, #f97316 100%%); color: white; padding: 30px 20px; text-align: center; }
			.header h1 { margin: 0; font-size: 24px; font-weight: 800; }
			.content { padding: 30px 20px; }
			.content p { line-height: 1.6; margin-bottom: 20px; }
			.btn { display: inline-block; background-color: #ef4444; color: white; text-decoration: none; font-weight: bold; padding: 14px 24px; border-radius: 12px; margin-top: 10px; text-align: center; }
			.footer { background-color: #f8fafc; padding: 20px; text-align: center; font-size: 12px; color: #94a3b8; border-top: 1px solid #e2e8f0; }
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h1>Cloud Baja Tegal</h1>
				<p style="margin: 5px 0 0 0; opacity: 0.9;">Reset Password Akun</p>
			</div>
			<div class="content">
				<p>Halo <strong>%s</strong>,</p>
				<p>Kami menerima permintaan untuk mereset kata sandi (password) akun Cloud Baja Tegal Anda.</p>
				<p>Jika Anda merasa melakukan permintaan ini, silakan klik tombol di bawah ini untuk mengatur kata sandi baru. Link ini hanya berlaku selama 1 jam.</p>
				
				<div style="text-align: center; margin: 30px 0;">
					<a href="%s" class="btn">Reset Password Sekarang</a>
				</div>

				<p style="font-size: 13px; color: #64748b;">Jika Anda tidak pernah meminta reset password, abaikan email ini. Akun Anda tetap aman.</p>
			</div>
			<div class="footer">
				&copy; 2026 Cloud Baja Tegal. All rights reserved.<br>
				Pesan ini dibuat otomatis oleh sistem.
			</div>
		</div>
	</body>
	</html>
	`, username, resetLink)

	headers := make(map[string]string)
	headers["From"] = user
	headers["To"] = toEmail
	headers["Subject"] = "Reset Password Request - Cloud Baja Tegal"
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"UTF-8\""

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + htmlBody

	err := smtp.SendMail(host+":"+port, auth, user, []string{toEmail}, []byte(message))
	return err
}

func (s *emailServiceImpl) SendVMProvisioningNotification(toEmail, username, orderName string, cores, memory, storage int) error {
	host := config.Env.SMTPHost
	port := config.Env.SMTPPort
	user := config.Env.SMTPUser
	pass := config.Env.SMTPPass

	if user == "" || pass == "" {
		fmt.Printf("===================================================\n")
		fmt.Printf("[MOCK EMAIL NOTIFICATION] To: %s\n", toEmail)
		fmt.Printf("[MOCK EMAIL NOTIFICATION] VM Name: %s\n", orderName)
		fmt.Printf("===================================================\n")
		return nil
	}

	auth := smtp.PlainAuth("", user, pass, host)

	subject := "Pemberitahuan: VM Anda sedang dibuat (" + orderName + ")"

	memoryStr := fmt.Sprintf("%d MB", memory)
	if memory >= 1024 {
		memoryStr = fmt.Sprintf("%d GB", memory/1024)
	}

	htmlBody := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<meta charset="UTF-8">
		<style>
			body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f8fafc; margin: 0; padding: 20px; color: #334155; }
			.container { max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 16px; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1); }
			.header { background: linear-gradient(135deg, #10b981 0%%, #059669 100%%); color: white; padding: 30px 20px; text-align: center; }
			.header h1 { margin: 0; font-size: 24px; font-weight: 800; }
			.content { padding: 30px 20px; }
			.content p { line-height: 1.6; margin-bottom: 20px; }
			.table-container { background-color: #f1f5f9; border-radius: 12px; padding: 20px; margin-bottom: 20px; }
			table { width: 100%%; border-collapse: collapse; }
			td { padding: 8px 0; font-size: 14px; }
			.label { font-weight: 600; color: #64748b; width: 40%%; }
			.value { font-weight: 700; color: #0f172a; text-align: right; }
			.footer { background-color: #f8fafc; padding: 20px; text-align: center; font-size: 12px; color: #94a3b8; border-top: 1px solid #e2e8f0; }
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h1>Cloud Baja Tegal</h1>
				<p style="margin: 5px 0 0 0; opacity: 0.9;">Pemberitahuan Pembuatan Virtual Machine</p>
			</div>
			<div class="content">
				<p>Halo <strong>%s</strong>,</p>
				<p>Administrator telah memulai proses pembuatan Virtual Machine Anda. VM Anda saat ini sedang dalam tahap <strong>PROVISIONING</strong> dan akan segera beroperasi.</p>
				
				<div class="table-container">
					<table>
						<tr><td class="label">Nama VM</td><td class="value">%s</td></tr>
						<tr><td class="label">CPU Cores</td><td class="value">%d Cores</td></tr>
						<tr><td class="label">RAM (Memory)</td><td class="value">%s</td></tr>
						<tr><td class="label">Storage (NVMe)</td><td class="value">%d GB</td></tr>
					</table>
				</div>

				<p>Silakan pantau status VM Anda secara berkala melalui Dashboard. Tidak ada tindakan lebih lanjut yang perlu Anda lakukan.</p>
			</div>
			<div class="footer">
				&copy; 2026 Cloud Baja Tegal. All rights reserved.<br>
				Pesan ini dibuat otomatis oleh sistem.
			</div>
		</div>
	</body>
	</html>
	`, username, orderName, cores, memoryStr, storage)

	headers := make(map[string]string)
	headers["From"] = user
	headers["To"] = toEmail
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"UTF-8\""

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + htmlBody

	err := smtp.SendMail(host+":"+port, auth, user, []string{toEmail}, []byte(message))
	return err
}
