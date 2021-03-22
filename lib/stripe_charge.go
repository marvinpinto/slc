package lib

import (
	"fmt"
	"time"

	stripe "github.com/stripe/stripe-go/v72"
)

func (r *StripeRunner) processStripeCharge(bt *stripe.BalanceTransaction, payout *stripe.Payout, lookupList *ledgerAccountLookup) error {
	var trLines []TransactionPosting

	bankAcctInfo, err := lookupList.getOrAddItem(payout.Destination.ID, "Assets:Bank")
	if err != nil {
		return err
	}

	accTaxAmt := Zero()
	if bt.Source != nil && bt.Source.Charge != nil && bt.Source.Charge.Invoice != nil {
		for _, taxAmt := range bt.Source.Charge.Invoice.TotalTaxAmounts {
			taxAcctInfo, err := lookupList.getOrAddItem(taxAmt.TaxRate.ID, "Liabilities:SalesTax")
			if err != nil {
				return err
			}

			normalizedTaxAmt := Zero().SetInt64(taxAmt.Amount)
			if bt.Currency != bt.Source.Charge.Currency {
				// normalizedTaxAmt *= exchange rate
				normalizedTaxAmt.Mul(normalizedTaxAmt, Zero().SetFloat64(bt.ExchangeRate))
			}
			accTaxAmt.Add(accTaxAmt, normalizedTaxAmt)

			// Tax liability line
			trLines = append(trLines, TransactionPosting{
				Account: taxAcctInfo.AcctName,
				// -1 * (normalizedTaxAmt / 100)
				Amount:   Zero().Neg(Zero().Quo(normalizedTaxAmt, Zero().SetFloat64(100))),
				Currency: string(bt.Currency),
			})
		}
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
		// -1 * ((bt.Amount - accTaxAmt)/100)
		Amount:   Zero().Neg(Zero().Quo(Zero().Sub(Zero().SetInt64(bt.Amount), accTaxAmt), Zero().SetFloat64(100))),
		Currency: string(bt.Currency),
	})

	// Stripe fees line
	trLines = append(trLines, TransactionPosting{
		Account: stripeFeesAcctInfo.AcctName,
		// bt.Fee / 100
		Amount:   Zero().Quo(Zero().SetInt64(bt.Fee), Zero().SetFloat64(100)),
		Currency: string(bt.Currency),
	})

	// Income destination line
	trLines = append(trLines, TransactionPosting{
		Account: bankAcctInfo.AcctName,
		// bt.Net / 100
		Amount:   Zero().Quo(Zero().SetInt64(bt.Net), Zero().SetFloat64(100)),
		Currency: string(bt.Currency),
	})

	tr, err := NewLedgerTransaction(time.Unix(bt.Created, 0), "Stripe Payout", trLines)
	if err != nil {
		return err
	}

	tr.AddComment(fmt.Sprintf("Correlates to Stripe payout %s from %s for amount %s", payout.ID, tr.formatDate(payout.ArrivalDate), tr.formatUnitAmount(payout.Amount, string(payout.Currency))))

	// Ledger transaction comments for customer metadata
	addCustMetadata := r.viper.GetBool("stripe.add_customer_metadata")
	if addCustMetadata && bt.Source != nil && bt.Source.Charge != nil && bt.Source.Charge.BillingDetails != nil && bt.Source.Charge.BillingDetails.Address != nil {
		tr.AddKeyValComment("CustomerCity", bt.Source.Charge.BillingDetails.Address.City)
		tr.AddKeyValComment("CustomerState", bt.Source.Charge.BillingDetails.Address.State)
		tr.AddKeyValComment("CustomerCountry", bt.Source.Charge.BillingDetails.Address.Country)
		tr.AddKeyValComment("CustomerPostalCode", bt.Source.Charge.BillingDetails.Address.PostalCode)
	}

	tr.SetDateFormat(r.viper.GetString("date_format_string"))
	fmt.Fprintln(r.outputWriter, tr.String())

	return nil
}
