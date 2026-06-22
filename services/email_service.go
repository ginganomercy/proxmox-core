package services

import (
	"fmt"
	"net/smtp"

	"cbt-core-api/config"
)

type EmailService interface {
	SendActivationCode(toEmail, username, orderName, activationCode string) error
	SendPasswordReset(toEmail, username, resetToken string) error
	SendOrderInvoice(toEmail, username, orderName string, cores, memory, storage int, totalCost float64) error
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

	htmlBody := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<meta charset="UTF-8">
		<style>
			body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f8fafc; margin: 0; padding: 20px; color: #334155; }
			.container { max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 16px; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1); }
			.header { background: linear-gradient(135deg, #4f46e5 0%%, #3b82f6 100%%); color: white; padding: 30px 20px; text-align: center; }
			.header h1 { margin: 0; font-size: 24px; font-weight: 800; }
			.content { padding: 30px 20px; }
			.content p { line-height: 1.6; margin-bottom: 20px; }
			.code-box { background-color: #eef2ff; border: 2px dashed #6366f1; border-radius: 12px; padding: 20px; text-align: center; margin: 30px 0; }
			.code { font-size: 32px; font-weight: 900; color: #4f46e5; letter-spacing: 4px; font-family: monospace; }
			.footer { background-color: #f8fafc; padding: 20px; text-align: center; font-size: 12px; color: #94a3b8; border-top: 1px solid #e2e8f0; }
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h1>Cloud Baja Tegal</h1>
				<p style="margin: 5px 0 0 0; opacity: 0.9;">Aktivasi Virtual Machine</p>
			</div>
			<div class="content">
				<p>Halo <strong>%s</strong>,</p>
				<p>Pembayaran Anda untuk pesanan <strong>%s</strong> telah kami konfirmasi! VM Anda kini siap untuk dihidupkan (Provisioning).</p>
				<p>Silakan salin 6-digit Kode Aktivasi di bawah ini dan masukkan ke dalam Dashboard CBT Anda:</p>
				
				<div class="code-box">
					<div class="code">%s</div>
				</div>

				<p>Terima kasih telah mempercayakan infrastruktur cloud Anda kepada Cloud Baja Tegal.</p>
			</div>
			<div class="footer">
				&copy; 2026 Cloud Baja Tegal. All rights reserved.<br>
				Pesan ini dibuat otomatis oleh sistem.
			</div>
		</div>
	</body>
	</html>
	`, username, orderName, activationCode)

	headers := make(map[string]string)
	headers["From"] = user
	headers["To"] = toEmail
	headers["Subject"] = "Kode Aktivasi VM: " + orderName
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

func (s *emailServiceImpl) SendOrderInvoice(toEmail, username, orderName string, cores, memory, storage int, totalCost float64) error {
	host := config.Env.SMTPHost
	port := config.Env.SMTPPort
	user := config.Env.SMTPUser
	pass := config.Env.SMTPPass

	if user == "" || pass == "" {
		fmt.Printf("===================================================\n")
		fmt.Printf("[MOCK EMAIL INVOICE] To: %s\n", toEmail)
		fmt.Printf("[MOCK EMAIL INVOICE] VM Name: %s, Total: Rp %.0f\n", orderName, totalCost)
		fmt.Printf("===================================================\n")
		return nil
	}

	auth := smtp.PlainAuth("", user, pass, host)

	subject := "Invoice Pesanan VM: " + orderName

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
			.header { background: linear-gradient(135deg, #4f46e5 0%%, #3b82f6 100%%); color: white; padding: 30px 20px; text-align: center; }
			.header h1 { margin: 0; font-size: 24px; font-weight: 800; }
			.content { padding: 30px 20px; }
			.content p { line-height: 1.6; margin-bottom: 20px; }
			.table-container { background-color: #f1f5f9; border-radius: 12px; padding: 20px; margin-bottom: 20px; }
			table { width: 100%%; border-collapse: collapse; }
			td { padding: 8px 0; font-size: 14px; }
			.label { font-weight: 600; color: #64748b; width: 40%%; }
			.value { font-weight: 700; color: #0f172a; text-align: right; }
			.total-row td { border-top: 2px dashed #cbd5e1; padding-top: 16px; margin-top: 8px; font-size: 16px; }
			.total-row .value { color: #4f46e5; font-size: 20px; }
			.btn { display: inline-block; background-color: #25D366; color: white; text-decoration: none; font-weight: bold; padding: 14px 24px; border-radius: 12px; margin-top: 10px; width: 100%%; box-sizing: border-box; text-align: center; }
			.footer { background-color: #f8fafc; padding: 20px; text-align: center; font-size: 12px; color: #94a3b8; border-top: 1px solid #e2e8f0; }
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h1>Cloud Baja Tegal</h1>
				<p style="margin: 5px 0 0 0; opacity: 0.9;">Tagihan Pesanan Virtual Machine</p>
			</div>
			<div class="content">
				<p>Halo <strong>%s</strong>,</p>
				<p>Terima kasih telah melakukan pemesanan server VPS di Cloud Baja Tegal. Pesanan Anda telah kami terima dan saat ini berstatus <strong>PENDING</strong> (Menunggu Pembayaran).</p>
				
				<div class="table-container">
					<table>
						<tr><td class="label">Nama VM</td><td class="value">%s</td></tr>
						<tr><td class="label">CPU Cores</td><td class="value">%d Cores</td></tr>
						<tr><td class="label">RAM (Memory)</td><td class="value">%s</td></tr>
						<tr><td class="label">Storage (NVMe)</td><td class="value">%d GB</td></tr>
						<tr class="total-row">
							<td class="label">Total Pembayaran</td>
							<td class="value">Rp %.0f</td>
						</tr>
					</table>
				</div>

				<p style="font-size: 13px; color: #64748b;">* Biaya di atas adalah sistem sekali bayar (One-Time Payment) untuk selamanya.</p>
				<p>Untuk menyelesaikan pesanan dan mendapatkan <strong>Kode Aktivasi VM</strong> Anda, silakan lakukan konfirmasi pembayaran dengan menghubungi Admin melalui WhatsApp.</p>
				
				<a href="https://wa.me/62856117933?text=Halo%%20Admin%%20CBT,%%20saya%%20ingin%%20konfirmasi%%20pembayaran%%20untuk%%20pesanan%%20VM:%%20%s%%20dengan%%20email%%20%s" class="btn">Konfirmasi Pembayaran via WhatsApp</a>
			</div>
			<div class="footer">
				&copy; 2026 Cloud Baja Tegal. All rights reserved.<br>
				Pesan ini dibuat otomatis oleh sistem.
			</div>
		</div>
	</body>
	</html>
	`, username, orderName, cores, memoryStr, storage, totalCost, orderName, toEmail)

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
