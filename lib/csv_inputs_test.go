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

func TestCSVInputs(t *testing.T) {
	type test struct {
		name          string
		skipTest      bool
		inpCSVMapCfg  *csvMappedAcctCfg
		inpLookupList *[]lookupItem
		inpCSVData    string
		expOutput     string
		expError      error
	}

	tests := []test{
		{
			name:     "handles two money columns",
			skipTest: false,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "1/2/2006",
				DateCol:        1,
				DescCol:        2,
				MoneyCols:      []int{4, 5},
				NegateAmt:      false,
				NoteCols:       []int{3},
				Currency:       "cad",
			},
			inpLookupList: &[]lookupItem{},
			inpCSVData:    "testdata/csv/two-money-columns.csv",
			expOutput:     "testdata/csv/two-money-columns.ledger",
			expError:      nil,
		},
		{
			name:     "handles single money column with header",
			skipTest: false,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "2006/1/2",
				DateCol:        1,
				DescCol:        2,
				MoneyCols:      []int{3},
				NegateAmt:      false,
				NoteCols:       []int{},
				Currency:       "eur",
				HeaderRow:      1,
			},
			inpLookupList: &[]lookupItem{},
			inpCSVData:    "testdata/csv/single-money-column-w-header.csv",
			expOutput:     "testdata/csv/single-money-column-w-header.ledger",
			expError:      nil,
		},
		{
			name:     "handles different date format with notes",
			skipTest: false,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "20060102",
				DateCol:        2,
				DescCol:        3,
				MoneyCols:      []int{4},
				NegateAmt:      false,
				NoteCols:       []int{1},
				Currency:       "usd",
				HeaderRow:      0,
			},
			inpLookupList: &[]lookupItem{},
			inpCSVData:    "testdata/csv/full-date-example.csv",
			expOutput:     "testdata/csv/full-date-example.ledger",
			expError:      nil,
		},
		{
			name:     "handles two money columns part 2",
			skipTest: false,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "1/2/2006",
				DateCol:        1,
				DescCol:        3,
				MoneyCols:      []int{4, 5},
				NegateAmt:      false,
				NoteCols:       []int{2},
				Currency:       "eur",
			},
			inpLookupList: &[]lookupItem{},
			inpCSVData:    "testdata/csv/two-money-columns-2.csv",
			expOutput:     "testdata/csv/two-money-columns-2.ledger",
			expError:      nil,
		},
		{
			name:     "handles two money columns part 3",
			skipTest: false,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "02 Jan 2006",
				DateCol:        1,
				DescCol:        3,
				MoneyCols:      []int{4, 5},
				NegateAmt:      false,
				NoteCols:       []int{3},
				Currency:       "gbp",
			},
			inpLookupList: &[]lookupItem{},
			inpCSVData:    "testdata/csv/two-money-columns-3.csv",
			expOutput:     "testdata/csv/two-money-columns-3.ledger",
			expError:      nil,
		},
		{
			name:     "handles two money columns part 4",
			skipTest: false,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "02.01.2006",
				DateCol:        1,
				DescCol:        2,
				MoneyCols:      []int{4, 5},
				NegateAmt:      false,
				NoteCols:       []int{3},
				Currency:       "usd",
			},
			inpLookupList: &[]lookupItem{},
			inpCSVData:    "testdata/csv/two-money-columns-4.csv",
			expOutput:     "testdata/csv/two-money-columns-4.ledger",
			expError:      nil,
		},
		{
			name:     "handles inversed CC statements",
			skipTest: false,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Liabilities:Mastercard",
				CsvDateFormat:  "2006/01/02",
				DateCol:        1,
				DescCol:        6,
				MoneyCols:      []int{7},
				NegateAmt:      true,
				NoteCols:       []int{4, 5},
				Currency:       "cad",
			},
			inpLookupList: &[]lookupItem{},
			inpCSVData:    "testdata/csv/inversed-cc.csv",
			expOutput:     "testdata/csv/inversed-cc.ledger",
			expError:      nil,
		},
		{
			name:     "discards transactions",
			skipTest: false,
			inpCSVMapCfg: &csvMappedAcctCfg{
				LedgerAcctName: "Assets:Bank123",
				CsvDateFormat:  "02.01.2006",
				DateCol:        1,
				DescCol:        2,
				MoneyCols:      []int{4, 5},
				NegateAmt:      false,
				NoteCols:       []int{3},
				Currency:       "usd",
			},
			inpLookupList: &[]lookupItem{
				{
					Search:             "(?i)check.*0000000",
					AcctName:           "Expenses:Discarded",
					Description:        "Duplicate Expenses",
					DiscardTransaction: true,
				},
			},
			inpCSVData: "testdata/csv/two-money-columns-4.csv",
			expOutput:  "testdata/csv/discards-transactions.ledger",
			expError:   nil,
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
			v.Set(csvMappingKeyFullName, tc.inpCSVMapCfg)

			v.Set("ledger_account_lookups", tc.inpLookupList)

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
		})
	}
}
