package lib

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/client"
	"github.com/stripe/stripe-go/v72/form"

	log "github.com/sirupsen/logrus"
	afero "github.com/spf13/afero"
	viperlib "github.com/spf13/viper"
)

func TestMain(m *testing.M) {
	flag.Parse()
	log.SetOutput(ioutil.Discard)
	log.SetLevel(log.FatalLevel)

	if testing.Verbose() {
		log.SetOutput(os.Stdout)
		log.SetLevel(log.DebugLevel)
	}

	os.Exit(m.Run())
}

func TestGenerateStripeLedgerEntries(t *testing.T) {
	type test struct {
		name                      string
		skipTest                  bool
		inpIsQueryCursorPresent   bool
		inpPayoutListApiCallErr   bool
		inpBTListApiCallErr       bool
		inpPayoutList             string
		inpBalanceTransactionList string
		expOutput                 string
		expError                  error
		expSavedCursor            string
	}

	tests := []test{
		{
			name:                      "does not use the query cursor when not present",
			skipTest:                  false,
			inpIsQueryCursorPresent:   false,
			inpPayoutListApiCallErr:   false,
			inpBTListApiCallErr:       false,
			inpPayoutList:             "testdata/stripe/empty-payload.json",
			inpBalanceTransactionList: "testdata/stripe/balance-transaction.json",
			expOutput:                 "testdata/stripe/empty-response.ledger",
			expError:                  nil,
			expSavedCursor:            "",
		},
		{
			name:                      "uses the query cursor when present",
			skipTest:                  false,
			inpIsQueryCursorPresent:   true,
			inpPayoutListApiCallErr:   false,
			inpBTListApiCallErr:       false,
			inpPayoutList:             "testdata/stripe/empty-payload.json",
			inpBalanceTransactionList: "testdata/stripe/balance-transaction.json",
			expOutput:                 "testdata/stripe/empty-response.ledger",
			expError:                  nil,
			expSavedCursor:            "cursor123",
		},
		{
			name:                      "gracefully handles stripe payout list API errors",
			skipTest:                  false,
			inpIsQueryCursorPresent:   false,
			inpPayoutListApiCallErr:   true,
			inpBTListApiCallErr:       false,
			inpPayoutList:             "testdata/stripe/empty-payload.json",
			inpBalanceTransactionList: "testdata/stripe/balance-transaction.json",
			expOutput:                 "testdata/stripe/empty-response.ledger",
			expError:                  errors.New("payout API testing error"),
			expSavedCursor:            "",
		},
		{
			name:                      "ignores stripe payouts to cards",
			skipTest:                  false,
			inpIsQueryCursorPresent:   false,
			inpPayoutListApiCallErr:   false,
			inpBTListApiCallErr:       false,
			inpPayoutList:             "testdata/stripe/card-payout.json",
			inpBalanceTransactionList: "testdata/stripe/balance-transaction.json",
			expOutput:                 "testdata/stripe/empty-response.ledger",
			expError:                  nil,
			expSavedCursor:            "po_1ITGPQCOCRzw0YkGEIImZLHC",
		},
		{
			name:                      "gracefully handles stripe balance transaction list API errors",
			skipTest:                  false,
			inpIsQueryCursorPresent:   false,
			inpPayoutListApiCallErr:   false,
			inpBTListApiCallErr:       true,
			inpPayoutList:             "testdata/stripe/bank-payout.json",
			inpBalanceTransactionList: "testdata/stripe/balance-transaction.json",
			expOutput:                 "testdata/stripe/empty-response.ledger",
			expError:                  errors.New("balance transaction API testing error"),
			expSavedCursor:            "",
		},
		{
			name:                      "is able to produce a basic ledger report",
			skipTest:                  false,
			inpIsQueryCursorPresent:   false,
			inpPayoutListApiCallErr:   false,
			inpBTListApiCallErr:       false,
			inpPayoutList:             "testdata/stripe/bank-payout.json",
			inpBalanceTransactionList: "testdata/stripe/balance-transaction.json",
			expOutput:                 "testdata/stripe/simple-report.ledger",
			expError:                  nil,
			expSavedCursor:            "",
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
			if tc.inpIsQueryCursorPresent {
				payoutArgs.Add("starting_after", "cursor123")
			}
			if tc.inpPayoutListApiCallErr {
				stripeBackend.
					On("CallRaw", "GET", "/v1/payouts", mock.Anything, payoutArgs, mock.Anything, mock.Anything).
					Return(fmt.Errorf("payout API testing error"))
			} else {
				stripeBackend.
					On("CallRaw", "GET", "/v1/payouts", mock.Anything, payoutArgs, mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						v := args.Get(5).(stripe.LastResponseSetter)
						SetStripeFixtureResponse(t, v, plFixture)
					}).
					Return(nil)
			}

			btFixture, err := ioutil.ReadFile(tc.inpBalanceTransactionList)
			if err != nil {
				t.Fatalf("Unable to read fixtures file %s", tc.inpBalanceTransactionList)
			}

			btArgs := new(form.Values)
			btArgs.Add("expand[0]", "data.source.invoice")
			btArgs.Add("expand[1]", "data.source.charge")
			btArgs.Add("expand[2]", "data.source.charge.balance_transaction")
			btArgs.Add("payout", "po_1ITGPQCOCRzw0YkGEIImZLHC")
			if tc.inpBTListApiCallErr {
				stripeBackend.
					On("CallRaw", "GET", "/v1/balance_transactions", mock.Anything, btArgs, mock.Anything, mock.Anything).
					Return(fmt.Errorf("balance transaction API testing error"))
			} else {
				stripeBackend.
					On("CallRaw", "GET", "/v1/balance_transactions", mock.Anything, btArgs, mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						v := args.Get(5).(stripe.LastResponseSetter)
						SetStripeFixtureResponse(t, v, btFixture)
					}).
					Return(nil)
			}

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
			if tc.inpIsQueryCursorPresent {
				afero.WriteFile(appFs, "/slcconfig.yml", []byte("---\nstripe:\n  most_recently_processed_payout: cursor123"), 0644)
			}
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
			assert.Equal(t, tc.expError, result)
			assert.Equal(t, string(expOutput), strings.Replace(output.String(), "\t", " ", -1))

			if tc.expSavedCursor != "" {
				resp, _ := afero.FileContainsBytes(appFs, "/slcconfig.yml", []byte(fmt.Sprintf("most_recently_processed_payout: %s", tc.expSavedCursor)))
				assert.True(t, resp, "pagination cursor is saved back to the config file")
			}
		})
	}
}
