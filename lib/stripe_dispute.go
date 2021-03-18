package lib

import (
	"fmt"
	"strings"

	stripe "github.com/stripe/stripe-go/v72"
)

func (r *StripeRunner) processStripeDispute(bt *stripe.BalanceTransaction, payout *stripe.Payout) error {
	var ledgerEntry strings.Builder

	ledgerEntry.WriteString(fmt.Sprintf("%s * Stripe Dispute Charge\n", r.getFormattedDate(bt.Created)))
	ledgerEntry.WriteString(fmt.Sprintf("\t; Correlates to Stripe payout %s from %s for amount %s\n", payout.ID, r.getFormattedDate(payout.ArrivalDate), r.getFormattedAmount(payout.Amount, payout.Currency, false)))

	bankAcctKey := fmt.Sprintf("ledger_accounts.bank_account_%s", strings.ToLower(payout.Destination.ID))
	if !r.viper.IsSet(bankAcctKey) {
		r.logger.Warnf("No account map set for %s, using the default value of %s instead", bankAcctKey, "Assets:Bank")
	}
	r.viper.SetDefault(bankAcctKey, "Assets:Bank")

	incomeSrc := r.viper.Get("ledger_accounts.income")
	if bt.Source != nil && bt.Source.Charge != nil && bt.Source.Charge.Customer != nil {
		incomeSrc = fmt.Sprintf("%s:Customer-%s", r.viper.Get("ledger_accounts.income"), bt.Source.Charge.Customer.ID)
	}

	ledgerEntry.WriteString(fmt.Sprintf("\t%s\t\t%s\n", incomeSrc, r.getFormattedAmount(bt.Amount, bt.Currency, true)))
	ledgerEntry.WriteString(fmt.Sprintf("\t%s\t\t%s\n", r.viper.GetString("ledger_accounts.stripe_fees"), r.getFormattedAmount(bt.Fee, bt.Currency, false)))
	ledgerEntry.WriteString(fmt.Sprintf("\t%s\t\t%s\n", r.viper.GetString(bankAcctKey), r.getFormattedAmount(bt.Net, bt.Currency, false)))
	fmt.Fprintln(r.outputWriter, ledgerEntry.String())

	return nil
}
