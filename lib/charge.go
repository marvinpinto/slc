package lib

import (
	"fmt"
	"math"
	"strings"

	stripe "github.com/stripe/stripe-go/v72"
)

func (r *Runner) processStripeCharge(bt *stripe.BalanceTransaction, payout *stripe.Payout) error {
	var ledgerEntry strings.Builder

	ledgerEntry.WriteString(fmt.Sprintf("%s * Stripe Payout\n", r.getFormattedDate(bt.Created)))
	ledgerEntry.WriteString(fmt.Sprintf("\t; Correlates to Stripe payout %s from %s for amount %s\n", payout.ID, r.getFormattedDate(payout.ArrivalDate), r.getFormattedAmount(payout.Amount, payout.Currency)))

	r.viper.SetDefault("misc.add_customer_metadata", true)
	addCustMetadata := r.viper.GetBool("misc.add_customer_metadata")

	if addCustMetadata && bt.Source != nil && bt.Source.Charge != nil && bt.Source.Charge.BillingDetails != nil && bt.Source.Charge.BillingDetails.Address != nil {
		if bt.Source.Charge.BillingDetails.Address.City != "" {
			ledgerEntry.WriteString(fmt.Sprintf("\t; CustomerCity: %s\n", bt.Source.Charge.BillingDetails.Address.City))
		}

		if bt.Source.Charge.BillingDetails.Address.State != "" {
			ledgerEntry.WriteString(fmt.Sprintf("\t; CustomerState: %s\n", bt.Source.Charge.BillingDetails.Address.State))
		}

		if bt.Source.Charge.BillingDetails.Address.Country != "" {
			ledgerEntry.WriteString(fmt.Sprintf("\t; CustomerCountry: %s\n", bt.Source.Charge.BillingDetails.Address.Country))
		}

		if bt.Source.Charge.BillingDetails.Address.PostalCode != "" {
			ledgerEntry.WriteString(fmt.Sprintf("\t; CustomerPostalCode: %s\n", bt.Source.Charge.BillingDetails.Address.PostalCode))
		}
	}

	r.viper.SetDefault("ledger_accounts.income", "Income:Stripe")
	r.viper.SetDefault("ledger_accounts.stripe_fees", "Expenses:Stripe Fees")

	bankAcctKey := fmt.Sprintf("ledger_accounts.bank_account_%s", strings.ToLower(payout.Destination.ID))
	if !r.viper.IsSet(bankAcctKey) {
		r.logger.Warnf("No account map set for %s, using the default value of %s instead", bankAcctKey, "Assets:Bank")
	}
	r.viper.SetDefault(bankAcctKey, "Assets:Bank")

	var accTaxAmt int64
	if bt.Source != nil && bt.Source.Charge != nil && bt.Source.Charge.Invoice != nil {
		for _, taxAmt := range bt.Source.Charge.Invoice.TotalTaxAmounts {
			taxRateAcctKey := fmt.Sprintf("ledger_accounts.tax_account_%s", strings.ToLower(taxAmt.TaxRate.ID))
			if !r.viper.IsSet(taxRateAcctKey) {
				r.logger.Warnf("No account map set for %s, using the default value of %s instead", taxRateAcctKey, "Liabilities:SalesTax")
			}
			r.viper.SetDefault(taxRateAcctKey, "Liabilities:SalesTax")

			normalizedTaxAmt := taxAmt.Amount
			if bt.Currency != bt.Source.Charge.Currency {
				normalizedTaxAmt = int64(math.Round(float64(taxAmt.Amount) * bt.ExchangeRate))
				accTaxAmt += normalizedTaxAmt
			} else {
				accTaxAmt += normalizedTaxAmt
			}

			ledgerEntry.WriteString(fmt.Sprintf("\t%s\t\t-%s\n", r.viper.GetString(taxRateAcctKey), r.getFormattedAmount(normalizedTaxAmt, bt.Currency)))
		}
	}

	incomeSrc := r.viper.Get("ledger_accounts.income")
	if bt.Source != nil && bt.Source.Charge != nil && bt.Source.Charge.Customer != nil {
		incomeSrc = fmt.Sprintf("%s:Customer-%s", r.viper.Get("ledger_accounts.income"), bt.Source.Charge.Customer.ID)
	}

	ledgerEntry.WriteString(fmt.Sprintf("\t%s\t\t-%s\n", incomeSrc, r.getFormattedAmount(bt.Amount-accTaxAmt, bt.Currency)))
	ledgerEntry.WriteString(fmt.Sprintf("\t%s\t\t%s\n", r.viper.GetString("ledger_accounts.stripe_fees"), r.getFormattedAmount(bt.Fee, bt.Currency)))
	ledgerEntry.WriteString(fmt.Sprintf("\t%s\t\t%s\n", r.viper.GetString(bankAcctKey), r.getFormattedAmount(bt.Net, bt.Currency)))
	fmt.Fprintln(r.outputWriter, ledgerEntry.String())

	return nil
}
