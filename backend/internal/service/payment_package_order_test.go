//go:build unit

package service

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type packageOrderUserRepo struct {
	UserRepository
	user *User
}

func (r packageOrderUserRepo) GetByID(context.Context, int64) (*User, error) { return r.user, nil }

type packageOrderBalancer struct{ payment.LoadBalancer }

func (packageOrderBalancer) SelectInstance(context.Context, string, payment.PaymentType, payment.Strategy, float64) (*payment.InstanceSelection, error) {
	// EasyPay popup generates a redirect locally: no external payment request.
	return &payment.InstanceSelection{
		InstanceID: "1", ProviderKey: payment.TypeEasyPay, SupportedTypes: payment.TypeAlipay,
		Config: map[string]string{
			"pid": "test", "pkey": "test", "apiBase": "https://payments.example.com",
			"notifyUrl": "https://app.example.com/notify", "returnUrl": "https://app.example.com/purchase",
			"paymentMode": "popup",
		},
	}, nil
}

func TestCreateOrderPreservesRechargePackages(t *testing.T) {
	for _, tc := range []struct {
		name                                           string
		amount, multiplier, fee, credited, bonus, paid float64
		tiers, mode                                    string
	}{
		{name: "starter", amount: 50, multiplier: 1, credited: 50, paid: 50},
		{name: "100 gift", amount: 100, multiplier: 1, credited: 120, bonus: 20, paid: 100},
		{name: "500 gift", amount: 500, multiplier: 1, credited: 650, bonus: 150, paid: 500},
		{name: "1000 gift", amount: 1000, multiplier: 1, credited: 1400, bonus: 400, paid: 1000},
		{name: "conversion and fee", amount: 100, multiplier: 0.14, fee: 2.5, credited: 16.8, bonus: 2.8, paid: 102.5},
		{name: "non-package has no gift", amount: 200, multiplier: 1, credited: 200, paid: 200},
		{name: "explicit tiers replace package", amount: 100, multiplier: 1, credited: 110, bonus: 10, paid: 100, tiers: `[{"min_amount":100,"bonus_percent":10}]`},
		{name: "explicit discount replaces package", amount: 100, multiplier: 1, credited: 100, bonus: 10, paid: 90, mode: "discount", tiers: `[{"min_amount":100,"bonus_percent":10}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			u, err := client.User.Create().SetEmail("package@example.com").SetPasswordHash("hash").Save(ctx)
			require.NoError(t, err)
			svc := &PaymentService{
				entClient:    client,
				userRepo:     packageOrderUserRepo{user: &User{ID: u.ID, Email: u.Email, Status: payment.EntityStatusActive}},
				loadBalancer: packageOrderBalancer{},
				configService: &PaymentConfigService{settingRepo: &paymentConfigSettingRepoStub{values: map[string]string{
					SettingPaymentEnabled: "true", SettingBalanceRechargeMult: fmt.Sprint(tc.multiplier),
					SettingRechargeFeeRate: fmt.Sprint(tc.fee), SettingRechargeBonusTiers: tc.tiers, SettingRechargeBonusMode: tc.mode,
				}}},
			}
			resp, err := svc.CreateOrder(ctx, CreateOrderRequest{UserID: u.ID, Amount: tc.amount, PaymentType: payment.TypeAlipay})
			require.NoError(t, err)
			order, err := client.PaymentOrder.Get(ctx, resp.OrderID)
			require.NoError(t, err)
			require.Equal(t, tc.credited, order.Amount)
			require.Equal(t, tc.bonus, order.BonusAmount)
			require.Equal(t, tc.paid, order.PayAmount)
			require.Equal(t, tc.credited, resp.Amount)
			redirect, err := url.Parse(*order.PayURL)
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf("%.2f", tc.paid), redirect.Query().Get("money"))
			require.Equal(t, tc.credited-tc.bonus, paymentOrderAmountWithoutBonus(order))
		})
	}
}
