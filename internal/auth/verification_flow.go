package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// VerificationFlow coordinates code generation with delivery.
type VerificationFlow struct {
	verification *VerificationService
	sender       VerificationCodeSender
}

func NewVerificationFlow(verification *VerificationService, sender VerificationCodeSender) (*VerificationFlow, error) {
	if verification == nil {
		return nil, errors.New("verification service is nil")
	}
	if sender == nil {
		return nil, errors.New("verification code sender is nil")
	}

	return &VerificationFlow{
		verification: verification,
		sender:       sender,
	}, nil
}

func (f *VerificationFlow) RequestCode(ctx context.Context, userID uuid.UUID, phoneNumber string) error {
	if userID == uuid.Nil {
		return errors.New("user id is required")
	}
	if phoneNumber == "" {
		return errors.New("phone number is required")
	}

	code, err := f.verification.Issue(ctx, userID)
	if err != nil {
		return err
	}

	if err := f.sender.Send(ctx, phoneNumber, code); err != nil {
		return err
	}

	return nil
}
