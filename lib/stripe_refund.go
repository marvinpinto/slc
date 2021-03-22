package lib

import (
	"fmt"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v72"
)

func (r *StripeRunner) processStripeRefund(bt *stripe.BalanceTransaction, payout *stripe.Payout) error {
	var trLines []TransactionPosting

	bankAcctKey := fmt.Sprintf("ledger_accounts.bank_account_%s", strings.ToLower(payout.Destination.ID))
	if !r.viper.IsSet(bankAcctKey) {
		r.logger.Warnf("No account map set for %s, using the default value of %s instead", bankAcctKey, "Assets:Bank")
	}
	r.viper.SetDefault(bankAcctKey, "Assets:Bank")

	accTaxAmt := Zero()
	if bt.Source != nil && bt.Source.Refund != nil && bt.Source.Refund.Charge != nil && bt.Source.Refund.Charge.Invoice != nil {
		for _, taxAmt := range bt.Source.Refund.Charge.Invoice.TotalTaxAmounts {
			taxRateAcctKey := fmt.Sprintf("ledger_accounts.tax_account_%s", strings.ToLower(taxAmt.TaxRate.ID))
			if !r.viper.IsSet(taxRateAcctKey) {
				r.logger.Warnf("No account map set for %s, using the default value of %s instead", taxRateAcctKey, "Liabilities:SalesTax")
			}
			r.viper.SetDefault(taxRateAcctKey, "Liabilities:SalesTax")

			normalizedTaxAmt := Zero().SetInt64(taxAmt.Amount)
			if bt.Currency != bt.Source.Refund.Charge.Currency {
				// normalizedTaxAmt *= exchange rate
				normalizedTaxAmt.Mul(normalizedTaxAmt, Zero().SetFloat64(bt.ExchangeRate))
			}
			accTaxAmt.Add(accTaxAmt, normalizedTaxAmt)

			// Tax liability line
			trLines = append(trLines, TransactionPosting{
				Account: r.viper.GetString(taxRateAcctKey),
				// -1 * (normalizedTaxAmt / 100)
				Amount:   Zero().Neg(Zero().Quo(normalizedTaxAmt, Zero().SetFloat64(100))),
				Currency: string(bt.Currency),
			})
		}
	}

	incomeSrc := r.viper.GetString("ledger_accounts.income")
	if bt.Source != nil && bt.Source.Refund.Charge != nil && bt.Source.Refund.Charge.Customer != nil {
		incomeSrc = fmt.Sprintf("%s:Customer-%s", r.viper.Get("ledger_accounts.income"), bt.Source.Refund.Charge.Customer.ID)
	}

	// Account for the original Stripe fee when calculating the net income (loss)
	var origStripeFee int64 = 0
	if bt.Source != nil && bt.Source.Refund != nil && bt.Source.Refund.Charge != nil && bt.Source.Refund.Charge.BalanceTransaction != nil {
		origStripeFee = bt.Source.Refund.Charge.BalanceTransaction.Fee
	}

	// Income source line
	trLines = append(trLines, TransactionPosting{
		Account: incomeSrc,
		// -1 * ((bt.Amount - accTaxAmt + origStripeFee)/100)
		Amount:   Zero().Neg(Zero().Quo(Zero().Add(Zero().Sub(Zero().SetInt64(bt.Amount), accTaxAmt), Zero().SetInt64(origStripeFee)), Zero().SetFloat64(100))),
		Currency: string(bt.Currency),
	})

	// Stripe fees line
	trLines = append(trLines, TransactionPosting{
		Account: r.viper.GetString("ledger_accounts.stripe_fees"),
		// bt.Fee / 100
		Amount:   Zero().Quo(Zero().SetInt64(bt.Fee), Zero().SetFloat64(100)),
		Currency: string(bt.Currency),
	})

	// Destination line
	trLines = append(trLines, TransactionPosting{
		Account: r.viper.GetString(bankAcctKey),
		// (bt.Net + origStripeFee)/100
		Amount:   Zero().Quo(Zero().Add(Zero().SetInt64(bt.Net), Zero().SetInt64(origStripeFee)), Zero().SetFloat64(100)),
		Currency: string(bt.Currency),
	})

	tr, err := NewLedgerTransaction(time.Unix(bt.Created, 0), "Stripe Customer Refund", trLines)
	if err != nil {
		return err
	}

	tr.AddComment(fmt.Sprintf("Correlates to Stripe payout %s from %s for amount %s", payout.ID, tr.formatDate(payout.ArrivalDate), tr.formatUnitAmount(payout.Amount, string(payout.Currency))))

	if origStripeFee > 0 {
		tr.AddKeyValComment("Original Stripe fee", tr.formatUnitAmount(origStripeFee, string(payout.Currency)))
	}

	fmt.Fprintln(r.outputWriter, tr.String())

	return nil
}
