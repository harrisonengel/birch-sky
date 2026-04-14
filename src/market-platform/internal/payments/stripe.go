package payments

import (
	"context"
	"fmt"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/paymentintent"
)

type PaymentProcessor interface {
	CreatePaymentIntent(ctx context.Context, amountCents int, currency string) (clientSecret string, paymentID string, err error)
	ConfirmPayment(ctx context.Context, paymentID string) error
}

type StripeProcessor struct{}

func NewStripeProcessor(apiKey string) *StripeProcessor {
	stripe.Key = apiKey
	return &StripeProcessor{}
}

// StubProcessor auto-succeeds all payment operations. Used in dev/demo
// when no STRIPE_SECRET_KEY is configured.
type StubProcessor struct {
	nextID int
}

func NewStubProcessor() *StubProcessor {
	return &StubProcessor{}
}

func (p *StubProcessor) CreatePaymentIntent(_ context.Context, amountCents int, currency string) (string, string, error) {
	p.nextID++
	paymentID := fmt.Sprintf("stub_pi_%d", p.nextID)
	return "stub_secret_" + paymentID, paymentID, nil
}

func (p *StubProcessor) ConfirmPayment(_ context.Context, paymentID string) error {
	return nil // always succeeds
}

func (p *StripeProcessor) CreatePaymentIntent(_ context.Context, amountCents int, currency string) (string, string, error) {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(int64(amountCents)),
		Currency: stripe.String(currency),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		return "", "", fmt.Errorf("create payment intent: %w", err)
	}
	return pi.ClientSecret, pi.ID, nil
}

func (p *StripeProcessor) ConfirmPayment(_ context.Context, paymentID string) error {
	pi, err := paymentintent.Get(paymentID, nil)
	if err != nil {
		return fmt.Errorf("get payment intent: %w", err)
	}
	if pi.Status == stripe.PaymentIntentStatusSucceeded {
		return nil
	}
	if pi.Status == stripe.PaymentIntentStatusRequiresConfirmation {
		_, err = paymentintent.Confirm(paymentID, &stripe.PaymentIntentConfirmParams{})
		if err != nil {
			return fmt.Errorf("confirm payment: %w", err)
		}
		return nil
	}
	return fmt.Errorf("payment intent status: %s", pi.Status)
}
