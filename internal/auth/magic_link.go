package auth

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

type EmailSender struct {
	ses       *ses.Client
	fromEmail string
	appURL    string
}

func NewEmailSender(fromEmail, appURL string) (*EmailSender, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	return &EmailSender{
		ses:       ses.NewFromConfig(cfg),
		fromEmail: fromEmail,
		appURL:    appURL,
	}, nil
}

func (e *EmailSender) SendMagicLink(ctx context.Context, email, token string) error {
	magicLink := fmt.Sprintf("%s/api/auth/verify?token=%s", e.appURL, token)

	subject := "Sign in to Colette DN"
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f5f5f5; margin: 0; padding: 40px 20px;">
    <div style="max-width: 480px; margin: 0 auto; background: white; border-radius: 12px; padding: 40px; box-shadow: 0 2px 8px rgba(0,0,0,0.08);">
        <div style="text-align: center; margin-bottom: 32px;">
            <span style="font-size: 24px;">✦</span>
            <span style="font-family: 'Times New Roman', serif; font-size: 28px; font-weight: normal; margin-left: 8px;">Colette</span>
            <span style="font-size: 12px; color: #666; margin-left: 4px;">DN</span>
        </div>

        <h1 style="font-size: 20px; font-weight: 500; text-align: center; margin-bottom: 24px; color: #333;">
            Sign in to your account
        </h1>

        <p style="color: #666; text-align: center; margin-bottom: 32px; line-height: 1.5;">
            Click the button below to sign in. This link expires in 15 minutes.
        </p>

        <div style="text-align: center; margin-bottom: 32px;">
            <a href="%s" style="display: inline-block; background: #333; color: white; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 500;">
                Sign in to Colette DN
            </a>
        </div>

        <p style="color: #999; font-size: 13px; text-align: center; margin-bottom: 16px;">
            If you didn't request this email, you can safely ignore it.
        </p>

        <hr style="border: none; border-top: 1px solid #eee; margin: 24px 0;">

        <p style="color: #999; font-size: 12px; text-align: center;">
            If the button doesn't work, copy and paste this link:<br>
            <a href="%s" style="color: #666; word-break: break-all;">%s</a>
        </p>
    </div>
</body>
</html>
`, magicLink, magicLink, magicLink)

	textBody := fmt.Sprintf(`Sign in to Colette DN

Click this link to sign in (expires in 15 minutes):
%s

If you didn't request this email, you can safely ignore it.
`, magicLink)

	_, err := e.ses.SendEmail(ctx, &ses.SendEmailInput{
		Source: aws.String(e.fromEmail),
		Destination: &types.Destination{
			ToAddresses: []string{email},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data:    aws.String(subject),
				Charset: aws.String("UTF-8"),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data:    aws.String(htmlBody),
					Charset: aws.String("UTF-8"),
				},
				Text: &types.Content{
					Data:    aws.String(textBody),
					Charset: aws.String("UTF-8"),
				},
			},
		},
	})

	return err
}
