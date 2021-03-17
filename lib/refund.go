package lib

import (
	"fmt"
	"math"
	"strings"

	stripe "github.com/stripe/stripe-go/v72"
)

func (r *Runner) processStripeRefund(bt *stripe.BalanceTransaction, payout *stripe.Payout) error {
	var ledgerEntry strings.Builder

	ledgerEntry.WriteString(fmt.Sprintf("%s * Stripe Customer Refund\n", r.getFormattedDate(bt.Created)))
	ledgerEntry.WriteString(fmt.Sprintf("\t; Correlates to Stripe payout %s from %s for amount %s\n", payout.ID, r.getFormattedDate(payout.ArrivalDate), r.getFormattedAmount(payout.Amount, payout.Currency, false)))

	bankAcctKey := fmt.Sprintf("ledger_accounts.bank_account_%s", strings.ToLower(payout.Destination.ID))
	if !r.viper.IsSet(bankAcctKey) {
		r.logger.Warnf("No account map set for %s, using the default value of %s instead", bankAcctKey, "Assets:Bank")
	}
	r.viper.SetDefault(bankAcctKey, "Assets:Bank")

	var accTaxAmt int64

	if bt.Source != nil && bt.Source.Refund != nil && bt.Source.Refund.Charge != nil && bt.Source.Refund.Charge.Invoice != nil {
		for _, taxAmt := range bt.Source.Refund.Charge.Invoice.TotalTaxAmounts {
			taxRateAcctKey := fmt.Sprintf("ledger_accounts.tax_account_%s", strings.ToLower(taxAmt.TaxRate.ID))
			if !r.viper.IsSet(taxRateAcctKey) {
				r.logger.Warnf("No account map set for %s, using the default value of %s instead", taxRateAcctKey, "Liabilities:SalesTax")
			}
			r.viper.SetDefault(taxRateAcctKey, "Liabilities:SalesTax")

			normalizedTaxAmt := taxAmt.Amount
			if bt.Currency != bt.Source.Refund.Charge.Currency {
				normalizedTaxAmt = int64(math.Round(float64(taxAmt.Amount) * bt.ExchangeRate))
				accTaxAmt += normalizedTaxAmt
			} else {
				accTaxAmt += normalizedTaxAmt
			}

			ledgerEntry.WriteString(fmt.Sprintf("\t%s\t\t%s\n", r.viper.GetString(taxRateAcctKey), r.getFormattedAmount(normalizedTaxAmt, bt.Currency, true)))
		}
	}

	incomeSrc := r.viper.Get("ledger_accounts.income")
	if bt.Source != nil && bt.Source.Refund.Charge != nil && bt.Source.Refund.Charge.Customer != nil {
		incomeSrc = fmt.Sprintf("%s:Customer-%s", r.viper.Get("ledger_accounts.income"), bt.Source.Refund.Charge.Customer.ID)
	}

	// Account for the original Stripe fee when calculating the net income (loss)
	var origStripeFee int64 = 0
	if bt.Source != nil && bt.Source.Refund != nil && bt.Source.Refund.Charge != nil && bt.Source.Refund.Charge.BalanceTransaction != nil {
		origStripeFee = bt.Source.Refund.Charge.BalanceTransaction.Fee
	}
	if origStripeFee > 0 {
		ledgerEntry.WriteString(fmt.Sprintf("\t; Original Stripe fee: %s\n", r.getFormattedAmount(origStripeFee, payout.Currency, false)))
	}

	ledgerEntry.WriteString(fmt.Sprintf("\t%s\t\t%s\n", incomeSrc, r.getFormattedAmount(bt.Amount-accTaxAmt+origStripeFee, bt.Currency, true)))
	ledgerEntry.WriteString(fmt.Sprintf("\t%s\t\t%s\n", r.viper.GetString("ledger_accounts.stripe_fees"), r.getFormattedAmount(bt.Fee, bt.Currency, false)))
	ledgerEntry.WriteString(fmt.Sprintf("\t%s\t\t%s\n", r.viper.GetString(bankAcctKey), r.getFormattedAmount(bt.Net+origStripeFee, bt.Currency, false)))
	fmt.Fprintln(r.outputWriter, ledgerEntry.String())

	return nil
}
