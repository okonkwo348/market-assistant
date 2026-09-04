package auth

import "context"

// VerificationCodeSender delivers a verification code to a user's phone.
// The auth service remains independent of the concrete delivery provider.
type VerificationCodeSender interface {
	Send(ctx context.Context, phoneNumber, code string) error
}
