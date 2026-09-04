package auth

import (
	"context"
	"log/slog"
)

// LogVerificationCodeSender is a development sender that logs delivery
// instead of calling an external messaging provider.
type LogVerificationCodeSender struct {
	logger *slog.Logger
}

func NewLogVerificationCodeSender(logger *slog.Logger) *LogVerificationCodeSender {
	return &LogVerificationCodeSender{logger: logger}
}

func (s *LogVerificationCodeSender) Send(_ context.Context, phoneNumber, code string) error {
	if s.logger != nil {
		s.logger.Info("verification code generated", "phone_number", phoneNumber, "code", code)
	}
	return nil
}
