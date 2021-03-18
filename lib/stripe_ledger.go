package lib

import (
	"fmt"
	"io"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	viperlib "github.com/spf13/viper"
	stripe "github.com/stripe/stripe-go/v72"
	stripeClient "github.com/stripe/stripe-go/v72/client"
)

type StripeRunner struct {
	stripeClient *stripeClient.API
	outputWriter io.Writer
	viper        *viperlib.Viper
	logger       *log.Entry
	progressBar  ProgressBar
}

func NewStripeRunner(sc *stripeClient.API, ow io.Writer, v *viperlib.Viper, l *log.Entry, pb ProgressBar) *StripeRunner {
	return &StripeRunner{
		stripeClient: sc,
		outputWriter: ow,
		viper:        v,
		logger:       l,
		progressBar:  pb,
	}
}

func (r *StripeRunner) GenerateStripeLedgerEntries() error {
	params := &stripe.PayoutListParams{}
	params.Filters.AddFilter("status", "", "paid")
	params.AddExpand("data.destination")

	cursor := r.viper.GetString("stripe.most_recently_processed_payout")
	if cursor != "" {
		params.Filters.AddFilter("starting_after", "", cursor)
	}

	var numPayouts int64 = 0

	var mostRecentPayoutDate int64 = 0
	i := r.stripeClient.Payouts.List(params)
	for i.Next() {
		numPayouts += 1
		r.progressBar.Increment()

		p := i.Payout()
		if p.Created > mostRecentPayoutDate {
			r.logger.Debugf("Saving payout ID %s as the most recently seen payout", p.ID)
			r.viper.Set("stripe.most_recently_processed_payout", p.ID)
		}

		if err := r.processStripePayout(p); err != nil {
			return err
		}
	}

	if err := i.Err(); err != nil {
		r.logger.WithError(err).Error("Unable to retrieve payout list from Stripe")
		return err
	}

	if err := r.viper.WriteConfig(); err != nil {
		r.logger.WithError(err).Warn("Unable to update config file. This may result in duplicate transactions in the next run.")
	}

	r.progressBar.SetTotal(numPayouts, true)
	r.logger.Infof("Successfully processed %d Stripe payouts", numPayouts)
	return nil
}

func (r *StripeRunner) processStripePayout(payout *stripe.Payout) error {
	payoutAmt := float64(payout.Amount) / 100.0
	r.logger.Debugf("Processing stripe payout %s for %s %.2f, issued at %s (paid out to %s %s)", payout.ID, payout.Currency, payoutAmt, time.Unix(payout.Created, 0), payout.Destination.Type, payout.Destination.ID)

	if payout.Type == "card" {
		r.logger.Warnf("This application does not yet support Stripe payouts to cards (vs bank accounts). If you would like to see this supported, open an issue at https://github.com/marvinpinto/slc/issues. Ignoring payout %s for now.", payout.ID)
		return nil
	}

	r.logger.Debugf("Retrieving a list of all the balance transactions associated with payout %s", payout.ID)
	params := &stripe.BalanceTransactionListParams{}
	params.Filters.AddFilter("payout", "", payout.ID)
	params.AddExpand("data.source.invoice")
	params.AddExpand("data.source.charge")
	params.AddExpand("data.source.charge.balance_transaction")
	i := r.stripeClient.BalanceTransaction.List(params)
	for i.Next() {
		bt := i.BalanceTransaction()
		if err := r.processStripeBalanceTransaction(bt, payout); err != nil {
			return err
		}
	}

	if err := i.Err(); err != nil {
		r.logger.WithError(err).Errorf("Unable to retrieve the balance transactions for payout %s", payout.ID)
		return err
	}

	return nil
}

func (r *StripeRunner) processStripeBalanceTransaction(bt *stripe.BalanceTransaction, payout *stripe.Payout) error {
	// Note: This application only deals with a subset of the possible balance
	// transactions - primarily associated with payments related reporting
	// categories. See
	// https://stripe.com/docs/reports/reporting-categories#group-charge_and_payment_related

	r.logger.Debugf("Processing stripe balance transaction %s. Details: %s", bt.ID, debugObject(bt))
	if bt.ReportingCategory == "payout" {
		r.logger.Debugf("Ignoring balance transaction %s as this %s will already be covered in another category", bt.ID, bt.ReportingCategory)
		return nil
	}

	r.viper.SetDefault("stripe.add_customer_metadata", true)
	r.viper.SetDefault("ledger_accounts.income", "Income:Stripe")
	r.viper.SetDefault("ledger_accounts.stripe_fees", "Expenses:Stripe Fees")

	// TODO: also handle the dispute_reversal, & refund_failure categories
	switch bt.ReportingCategory {
	case "charge":
		return r.processStripeCharge(bt, payout)
	case "dispute":
		return r.processStripeDispute(bt, payout)
	case "refund":
		return r.processStripeRefund(bt, payout)
	case "fee":
		return r.processStripeFee(bt, payout)
	default:
		r.logger.Warnf("This application primarily supports balance transactions associated with payments, and does not support the %s type at the moment. See https://stripe.com/docs/reports/reporting-categories#group-charge_and_payment_related for more information.", bt.ReportingCategory)
		return nil
	}
}

func (r *StripeRunner) getFormattedDate(date int64) string {
	loc := time.FixedZone("UTC", 0)
	return time.Unix(date, 0).In(loc).Format(r.viper.GetString("date_format_string"))
}

func (r *StripeRunner) getFormattedAmount(amount int64, currency stripe.Currency, negation bool) string {
	if negation {
		amount = amount * -1
	}
	return fmt.Sprintf("%.2f %s", float64(amount)/100.0, strings.ToUpper(string(currency)))
}
