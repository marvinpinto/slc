package lib

import (
	"fmt"
	"time"

	stripe "github.com/stripe/stripe-go/v72"
)

func (r *StripeRunner) processStripeDispute(bt *stripe.BalanceTransaction, payout *stripe.Payout, lookupList *ledgerAccountLookup) error {
	var trLines []TransactionPosting

	bankAcctInfo, err := lookupList.getOrAddItem(payout.Destination.ID, "Assets:Bank")
	if err != nil {
		return err
	}

	incomeSrcKey := STRIPE_INCOME_SRC_LOOKUP_KEY
	if bt.Source != nil && bt.Source.Charge != nil && bt.Source.Charge.Customer != nil {
		incomeSrcKey = fmt.Sprintf("%s_%s", incomeSrcKey, bt.Source.Charge.Customer.ID)
	}
	incomeAcctInfo, err := lookupList.getOrAddItem(incomeSrcKey, "Income:Stripe")
	if err != nil {
		return err
	}

	stripeFeesAcctInfo, err := lookupList.getOrAddItem(STRIPE_FEES_LOOKUP_KEY, "Expenses:Stripe Fees")
	if err != nil {
		return err
	}

	// Income source line
	trLines = append(trLines, TransactionPosting{
		Account: incomeAcctInfo.AcctName,
		// -1 * bt.Amount/100
		Amount:   Zero().Neg(Zero().Quo(Zero().SetInt64(bt.Amount), Zero().SetFloat64(100))),
		Currency: string(bt.Currency),
	})

	// Stripe fees line
	trLines = append(trLines, TransactionPosting{
		Account: stripeFeesAcctInfo.AcctName,
		// bt.Fee / 100
		Amount:   Zero().Quo(Zero().SetInt64(bt.Fee), Zero().SetFloat64(100)),
		Currency: string(bt.Currency),
	})

	// Destination line
	trLines = append(trLines, TransactionPosting{
		Account: bankAcctInfo.AcctName,
		// bt.Net / 100
		Amount:   Zero().Quo(Zero().SetInt64(bt.Net), Zero().SetFloat64(100)),
		Currency: string(bt.Currency),
	})

	tr, err := NewLedgerTransaction(time.Unix(bt.Created, 0), "Stripe Dispute Charge", trLines)
	if err != nil {
		return err
	}

	tr.AddComment(fmt.Sprintf("Correlates to Stripe payout %s from %s for amount %s", payout.ID, tr.formatDate(payout.ArrivalDate), tr.formatUnitAmount(payout.Amount, string(payout.Currency))))

	tr.SetDateFormat(r.viper.GetString("date_format_string"))
	fmt.Fprintln(r.outputWriter, tr.String())

	return nil
}
