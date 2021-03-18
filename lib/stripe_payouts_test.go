package lib

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/client"
	"github.com/stripe/stripe-go/v72/form"

	log "github.com/sirupsen/logrus"
	afero "github.com/spf13/afero"
	viperlib "github.com/spf13/viper"
)

func TestStripePayouts(t *testing.T) {
	type test struct {
		name                      string
		skipTest                  bool
		inpPayoutList             string
		inpBalanceTransactionList string
		expOutput                 string
	}

	tests := []test{
		{
			name:                      "is able to handle a basic charge (no invoice)",
			skipTest:                  false,
			inpPayoutList:             "testdata/stripe/bank-payout.json",
			inpBalanceTransactionList: "testdata/stripe/balance-transaction.json",
			expOutput:                 "testdata/stripe/simple-report.ledger",
		},
		{
			name:                      "is able to handle a charges with customer info",
			skipTest:                  false,
			inpPayoutList:             "testdata/stripe/bank-payout.json",
			inpBalanceTransactionList: "testdata/stripe/charges/with-customer-info.json",
			expOutput:                 "testdata/stripe/charges/with-customer-info.ledger",
		},
		{
			name:                      "is able to handle multi currency payouts",
			skipTest:                  false,
			inpPayoutList:             "testdata/stripe/bank-payout.json",
			inpBalanceTransactionList: "testdata/stripe/charges/multi-currency.json",
			expOutput:                 "testdata/stripe/charges/multi-currency.ledger",
		},
		{
			name:                      "is able to handle invoices with tax line items",
			skipTest:                  false,
			inpPayoutList:             "testdata/stripe/bank-payout.json",
			inpBalanceTransactionList: "testdata/stripe/charges/taxed-items.json",
			expOutput:                 "testdata/stripe/charges/taxed-items.ledger",
		},
		{
			name:                      "is able to handle a basic refund",
			skipTest:                  false,
			inpPayoutList:             "testdata/stripe/bank-payout.json",
			inpBalanceTransactionList: "testdata/stripe/refunds/basic.json",
			expOutput:                 "testdata/stripe/refunds/basic.ledger",
		},
		// {
		// 	name:                      "is able to handle a refund with taxes",
		// 	skipTest:                  false,
		// 	inpPayoutList:             "testdata/stripe/bank-payout.json",
		// 	inpBalanceTransactionList: "testdata/stripe/refunds/with-taxes.json",
		// 	expOutput:                 "testdata/stripe/refund/with-taxes.ledger",
		// },
		{
			name:                      "is able to handle a lost dispute",
			skipTest:                  false,
			inpPayoutList:             "testdata/stripe/bank-payout.json",
			inpBalanceTransactionList: "testdata/stripe/disputes/lost.json",
			expOutput:                 "testdata/stripe/disputes/lost.ledger",
		},
		{
			name:                      "is able to handle stripe account fees",
			skipTest:                  false,
			inpPayoutList:             "testdata/stripe/bank-payout.json",
			inpBalanceTransactionList: "testdata/stripe/stripe-fee.json",
			expOutput:                 "testdata/stripe/stripe-fee.ledger",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipTest {
				t.Skip(fmt.Sprintf("Skipping test: %s", tc.name))
			}

			plFixture, err := ioutil.ReadFile(tc.inpPayoutList)
			if err != nil {
				t.Fatalf("Unable to read fixtures file %s", tc.inpPayoutList)
			}

			stripeBackend := &StripeMockBackend{}

			payoutArgs := new(form.Values)
			payoutArgs.Add("expand[0]", "data.destination")
			payoutArgs.Add("status", "paid")
			stripeBackend.
				On("CallRaw", "GET", "/v1/payouts", mock.Anything, payoutArgs, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					v := args.Get(5).(stripe.LastResponseSetter)
					SetStripeFixtureResponse(t, v, plFixture)
				}).
				Return(nil)

			btFixture, err := ioutil.ReadFile(tc.inpBalanceTransactionList)
			if err != nil {
				t.Fatalf("Unable to read fixtures file %s", tc.inpBalanceTransactionList)
			}

			btArgs := new(form.Values)
			btArgs.Add("expand[0]", "data.source.invoice")
			btArgs.Add("expand[1]", "data.source.charge")
			btArgs.Add("expand[2]", "data.source.charge.balance_transaction")
			btArgs.Add("payout", "po_1ITGPQCOCRzw0YkGEIImZLHC")
			stripeBackend.
				On("CallRaw", "GET", "/v1/balance_transactions", mock.Anything, btArgs, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					v := args.Get(5).(stripe.LastResponseSetter)
					SetStripeFixtureResponse(t, v, btFixture)
				}).
				Return(nil)

			sc := &client.API{}
			sc.Init("", &stripe.Backends{
				API: stripeBackend,
			})

			appFs := afero.NewMemMapFs()
			v := viperlib.New()
			v.SetFs(appFs)
			v.SetDefault("date_format_string", "2006-01-02")
			v.SetConfigName("slcconfig")
			v.AddConfigPath("/")
			afero.WriteFile(appFs, "/slcconfig.yml", []byte("---"), 0644)
			v.ReadInConfig()

			var logger = log.WithFields(log.Fields{"name": "slc-testing"})
			var output bytes.Buffer
			bar := &StubProgressBar{}
			runner := NewStripeRunner(sc, &output, v, logger, bar)

			expOutput, err := ioutil.ReadFile(tc.expOutput)
			if err != nil {
				t.Fatalf("Unable to read expected output file %s", tc.expOutput)
			}

			result := runner.GenerateStripeLedgerEntries()
			assert.Equal(t, nil, result)
			assert.Equal(t, string(expOutput), strings.Replace(output.String(), "\t", " ", -1))
		})
	}
}
