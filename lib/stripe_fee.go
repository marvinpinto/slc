package lib

import (
	"fmt"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v72"
)

func (r *StripeRunner) processStripeFee(bt *stripe.BalanceTransaction, payout *stripe.Payout) error {
	var trLines []TransactionPosting

	bankAcctKey := fmt.Sprintf("ledger_accounts.bank_account_%s", strings.ToLower(payout.Destination.ID))
	if !r.viper.IsSet(bankAcctKey) {
		r.logger.Warnf("No account map set for %s, using the default value of %s instead", bankAcctKey, "Assets:Bank")
	}
	r.viper.SetDefault(bankAcctKey, "Assets:Bank")

	// Stripe fees line
	trLines = append(trLines, TransactionPosting{
		Account: r.viper.GetString("ledger_accounts.stripe_fees"),
		// -1 * (bt.Amount / 100)
		Amount:   Zero().Neg(Zero().Quo(Zero().SetInt64(bt.Amount), Zero().SetFloat64(100))),
		Currency: string(bt.Currency),
	})

	// Destination line
	trLines = append(trLines, TransactionPosting{
		Account: r.viper.GetString(bankAcctKey),
		// bt.Amount / 100
		Amount:   Zero().Quo(Zero().SetInt64(bt.Amount), Zero().SetFloat64(100)),
		Currency: string(bt.Currency),
	})

	tr, err := NewLedgerTransaction(time.Unix(bt.Created, 0), "Stripe Account Fees", trLines)
	if err != nil {
		return err
	}

	tr.AddComment(fmt.Sprintf("Correlates to Stripe payout %s from %s for amount %s", payout.ID, tr.formatDate(payout.ArrivalDate), tr.formatUnitAmount(payout.Amount, string(payout.Currency))))

	fmt.Fprintln(r.outputWriter, tr.String())

	return nil
}
