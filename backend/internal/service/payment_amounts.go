package service

import (
	"math"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const defaultBalanceRechargeMultiplier = 1.0

type RechargePackage struct {
	Amount float64 `json:"amount"`
	Bonus  float64 `json:"bonus"`
}

var defaultRechargePackages = []RechargePackage{
	{Amount: 50, Bonus: 0},
	{Amount: 100, Bonus: 20},
	{Amount: 500, Bonus: 150},
	{Amount: 1000, Bonus: 400},
}

func DefaultRechargePackages() []RechargePackage {
	packages := make([]RechargePackage, len(defaultRechargePackages))
	copy(packages, defaultRechargePackages)
	return packages
}

func normalizeBalanceRechargeMultiplier(multiplier float64) float64 {
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
		return defaultBalanceRechargeMultiplier
	}
	return multiplier
}

// normalizeSubscriptionUSDToCNYRate 将非法值归一为 0（换算关闭）。
// 与余额倍率不同，0 是合法状态：表示订阅保持 price 直付的存量行为。
func normalizeSubscriptionUSDToCNYRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return 0
	}
	return rate
}

func calculateCreditedBalance(paymentAmount, multiplier float64) float64 {
	return calculateCreditedBalanceWithBonus(paymentAmount, 0, multiplier)
}

func rechargeBonusForAmount(paymentAmount float64) float64 {
	amount := decimal.NewFromFloat(paymentAmount).Round(2)
	for _, pkg := range defaultRechargePackages {
		if amount.Equal(decimal.NewFromFloat(pkg.Amount)) {
			return pkg.Bonus
		}
	}
	return 0
}

func calculateCreditedBalanceWithBonus(paymentAmount, bonus, multiplier float64) float64 {
	return decimal.NewFromFloat(paymentAmount).
		Add(decimal.NewFromFloat(bonus)).
		Mul(decimal.NewFromFloat(normalizeBalanceRechargeMultiplier(multiplier))).
		Round(2).
		InexactFloat64()
}

func calculateGatewayRefundAmount(orderAmount, payAmount, refundAmount float64, currency string) float64 {
	if orderAmount <= 0 || payAmount <= 0 || refundAmount <= 0 {
		return 0
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	if math.Abs(refundAmount-orderAmount) <= paymentAmountToleranceForCurrency(currency) {
		return decimal.NewFromFloat(payAmount).Round(fractionDigits).InexactFloat64()
	}
	return decimal.NewFromFloat(payAmount).
		Mul(decimal.NewFromFloat(refundAmount)).
		Div(decimal.NewFromFloat(orderAmount)).
		Round(fractionDigits).
		InexactFloat64()
}
