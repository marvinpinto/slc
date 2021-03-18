package lib

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	afero "github.com/spf13/afero"
	viperlib "github.com/spf13/viper"
)

func TestCSVFlow(t *testing.T) {
	type test struct {
		name                   string
		skipTest               bool
		inpIsMappingKeyPresent bool
		inpCSVMapCfg           *csvMappedAcctCfg
		inpCSVLookupList       *[]csvLedgerNameLookup
		inpCSVData             string
		expOutput              string
		expCSVMapCfg           *csvMappedAcctCfg
		expCSVLookupList       *[]csvLedgerNameLookup
		expError               error
	}

	tests := []test{
		{
			name:                   "generates a stock CSV mapping if not set in config",
			skipTest:               false,
			inpIsMappingKeyPresent: false,
			inpCSVData:             "testdata/csv/bad-csv-record.csv",
			expOutput:              "testdata/stripe/empty-response.ledger",
			expCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank",
				CsvDateFormat:  "2-Jan-2006",
				DateCol:        1,
				DescCol:        2,
				MoneyCols:      []int{3},
				NegateAmt:      false,
				NoteCols:       []int{4, 5},
				Currency:       "eur",
				HeaderRow:      0,
			},
			expError: nil,
		},
		{
			name:                   "returns an error if provided an invalid csv file",
			skipTest:               false,
			inpIsMappingKeyPresent: true,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "2-Jan-2006",
				DateCol:        1,
				DescCol:        2,
				MoneyCols:      []int{3},
				NegateAmt:      false,
				NoteCols:       []int{5, 6},
				Currency:       "eur",
			},
			inpCSVData: "testdata/csv/bad-csv-record.csv",
			expOutput:  "testdata/stripe/empty-response.ledger",
			expCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "2-Jan-2006",
				DateCol:        1,
				DescCol:        2,
				MoneyCols:      []int{3},
				NegateAmt:      false,
				NoteCols:       []int{5, 6},
				Currency:       "eur",
			},
			expError: fmt.Errorf("expect an error here"),
		},
		{
			name:                   "returns an error if unable to parse the date value",
			skipTest:               false,
			inpIsMappingKeyPresent: true,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "2-Jan-2006",
				DateCol:        1,
				DescCol:        2,
				MoneyCols:      []int{3},
				NegateAmt:      false,
				NoteCols:       []int{5, 6},
				Currency:       "eur",
			},
			inpCSVData: "testdata/csv/basic-record.csv",
			expOutput:  "testdata/stripe/empty-response.ledger",
			expCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "2-Jan-2006",
				DateCol:        1,
				DescCol:        2,
				MoneyCols:      []int{3},
				NegateAmt:      false,
				NoteCols:       []int{5, 6},
				Currency:       "eur",
			},
			expError: fmt.Errorf("expect an error here"),
		},
		{
			name:                   "returns an error if unable to parse the money value",
			skipTest:               false,
			inpIsMappingKeyPresent: true,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "2-Jan-2006",
				DateCol:        2,
				DescCol:        3,
				MoneyCols:      []int{0},
				NegateAmt:      false,
				NoteCols:       []int{},
				Currency:       "eur",
			},
			inpCSVData: "testdata/csv/basic-record.csv",
			expOutput:  "testdata/stripe/empty-response.ledger",
			expCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "2-Jan-2006",
				DateCol:        2,
				DescCol:        3,
				MoneyCols:      []int{0},
				NegateAmt:      false,
				NoteCols:       []int{},
				Currency:       "eur",
			},
			expError: fmt.Errorf("expect an error here"),
		},
		{
			name:                   "returns an error if unable to parse a user supplied config regex",
			skipTest:               false,
			inpIsMappingKeyPresent: true,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "2-Jan-2006",
				DateCol:        2,
				DescCol:        3,
				MoneyCols:      []int{5},
				NegateAmt:      false,
				NoteCols:       []int{},
				Currency:       "eur",
			},
			inpCSVLookupList: &[]csvLedgerNameLookup{
				{
					Search:   "[0-9]++",
					AcctName: "Assets:NewBankAccount",
				},
			},
			inpCSVData: "testdata/csv/basic-record.csv",
			expOutput:  "testdata/stripe/empty-response.ledger",
			expCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "2-Jan-2006",
				DateCol:        2,
				DescCol:        3,
				MoneyCols:      []int{5},
				NegateAmt:      false,
				NoteCols:       []int{},
				Currency:       "eur",
			},
			expCSVLookupList: &[]csvLedgerNameLookup{
				{
					Search:   "[0-9]++",
					AcctName: "Assets:NewBankAccount",
				},
			},
			expError: fmt.Errorf("expect an error here"),
		},
		{
			name:                   "is able to produce a basic ledger report",
			skipTest:               false,
			inpIsMappingKeyPresent: true,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "2-Jan-2006",
				DateCol:        2,
				DescCol:        3,
				MoneyCols:      []int{5, 6},
				NegateAmt:      false,
				NoteCols:       []int{},
				Currency:       "eur",
			},
			inpCSVLookupList: &[]csvLedgerNameLookup{},
			inpCSVData:       "testdata/csv/basic-record.csv",
			expOutput:        "testdata/csv/basic-record.ledger",
			expCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "2-Jan-2006",
				DateCol:        2,
				DescCol:        3,
				MoneyCols:      []int{5, 6},
				NegateAmt:      false,
				NoteCols:       []int{},
				Currency:       "eur",
			},
			expCSVLookupList: &[]csvLedgerNameLookup{
				{
					Search:      "Withdrawal Transfer to       acct123                                             ",
					AcctName:    "Expenses:Unknown",
					Description: "Withdrawal Transfer to       acct123                                             ",
				},
				{
					Search:      "External Deposit             Miscellaneous Payments       STRIPE ABCD123K8E  ",
					AcctName:    "Expenses:Unknown",
					Description: "External Deposit             Miscellaneous Payments       STRIPE ABCD123K8E  ",
				},
				{
					Search:      "Maintenance Service Charge                                                            ",
					AcctName:    "Expenses:Unknown",
					Description: "Maintenance Service Charge                                                            ",
				},
				{
					Search:      "External Deposit             Miscellaneous Payments       STRIPE ABCD123J2Q  ",
					AcctName:    "Expenses:Unknown",
					Description: "External Deposit             Miscellaneous Payments       STRIPE ABCD123J2Q  ",
				},
			},
			expError: nil,
		},
		{
			name:                   "is able to produce a basic ledger report with account lookups",
			skipTest:               false,
			inpIsMappingKeyPresent: true,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:BankOfMontreal",
				CsvDateFormat:  "02-Jan-2006",
				DateCol:        2,
				DescCol:        3,
				MoneyCols:      []int{5, 6},
				NegateAmt:      false,
				NoteCols:       []int{7},
				Currency:       "CAD",
			},
			inpCSVLookupList: &[]csvLedgerNameLookup{
				{
					Search:      "(?i)withdrawal.*to.*acct123",
					AcctName:    "Assets:Scotiabank",
					Description: "Cash Transfer",
				},
				{
					Search:      "(?i)external deposit .*stripe",
					AcctName:    "Income:Stripe",
					Description: "Revenue",
				},
			},
			inpCSVData: "testdata/csv/basic-record.csv",
			expOutput:  "testdata/csv/basic-record-with-lookups.ledger",
			expCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:BankOfMontreal",
				CsvDateFormat:  "02-Jan-2006",
				DateCol:        2,
				DescCol:        3,
				MoneyCols:      []int{5, 6},
				NegateAmt:      false,
				NoteCols:       []int{7},
				Currency:       "CAD",
			},
			expCSVLookupList: &[]csvLedgerNameLookup{
				{
					Search:      "(?i)withdrawal.*to.*acct123",
					AcctName:    "Assets:Scotiabank",
					Description: "Cash Transfer",
				},
				{
					Search:      "(?i)external deposit .*stripe",
					AcctName:    "Income:Stripe",
					Description: "Revenue",
				},
				{
					Search:      "Maintenance Service Charge                                                            ",
					AcctName:    "Expenses:Unknown",
					Description: "Maintenance Service Charge                                                            ",
				},
			},
			expError: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			csvMappingKeyName := "test_cc_account"
			csvMappingKeyFullName := fmt.Sprintf("csv.account.%s", csvMappingKeyName)

			if tc.skipTest {
				t.Skip(fmt.Sprintf("Skipping test: %s", tc.name))
			}

			csvFixture, err := os.Open(tc.inpCSVData)
			if err != nil {
				t.Fatalf("Unable to read fixtures file %s", tc.inpCSVData)
			}
			defer csvFixture.Close()

			appFs := afero.NewMemMapFs()
			v := viperlib.New()
			v.SetFs(appFs)
			v.SetDefault("date_format_string", "2006-01-02")
			v.SetConfigName("slcconfig")
			v.AddConfigPath("/")
			afero.WriteFile(appFs, "/slcconfig.yml", []byte("---"), 0644)
			v.ReadInConfig()

			if tc.inpIsMappingKeyPresent {
				v.Set(csvMappingKeyFullName, tc.inpCSVMapCfg)
			}

			if tc.inpCSVLookupList != nil {
				v.Set("ledger_account_lookups", tc.inpCSVLookupList)
			}

			var logger = log.WithFields(log.Fields{"name": "slc-testing"})
			var output bytes.Buffer
			bar := &StubProgressBar{}
			runner := NewCSVRunner(&output, v, logger, bar)

			expOutput, err := ioutil.ReadFile(tc.expOutput)
			if err != nil {
				t.Fatalf("Unable to read expected output file %s", tc.expOutput)
			}

			result := runner.GenerateLedgerEntries(csvFixture, csvMappingKeyName)
			if tc.expError != nil {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
			assert.Equal(t, string(expOutput), output.String())

			var mappedCfg csvMappedAcctCfg
			err = v.UnmarshalKey(csvMappingKeyFullName, &mappedCfg)
			if err != nil {
				t.Fatalf("Unable to unmarshal config key %s", csvMappingKeyFullName)
			}
			assert.Equal(t, tc.expCSVMapCfg, &mappedCfg)

			if tc.expCSVLookupList != nil {
				var nameLookupList []csvLedgerNameLookup
				err = v.UnmarshalKey("ledger_account_lookups", &nameLookupList)
				if err != nil {
					t.Fatalf("Unable to unmarshal config key %s", "ledger_account_lookups")
				}
				assert.Equal(t, tc.expCSVLookupList, &nameLookupList)
			}
		})
	}
}
