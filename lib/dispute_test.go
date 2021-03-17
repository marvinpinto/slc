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

func TestStripePayoutDisputes(t *testing.T) {
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
			inpPayoutList:             "testdata/bank-payout.json",
			inpBalanceTransactionList: "testdata/disputes/lost.json",
			expOutput:                 "testdata/disputes/lost.ledger",
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
			runner := NewRunner(sc, &output, v, logger, bar)

			expOutput, err := ioutil.ReadFile(tc.expOutput)
			if err != nil {
				t.Fatalf("Unable to read expected output file %s", tc.expOutput)
			}

			result := runner.GenerateLedgerEntries()
			assert.Equal(t, nil, result)
			assert.Equal(t, string(expOutput), strings.Replace(output.String(), "\t", " ", -1))
		})
	}
}
