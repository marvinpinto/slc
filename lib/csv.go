package lib

import (
	"encoding/csv"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"time"
	"unicode"
	"unicode/utf8"

	mapstructure "github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
	viperlib "github.com/spf13/viper"
)

type CSVRunner struct {
	outputWriter io.Writer
	viper        *viperlib.Viper
	logger       *log.Entry
	progressBar  ProgressBar
}

func NewCSVRunner(ow io.Writer, v *viperlib.Viper, l *log.Entry, pb ProgressBar) *CSVRunner {
	return &CSVRunner{
		outputWriter: ow,
		viper:        v,
		logger:       l,
		progressBar:  pb,
	}
}

type csvMappedAcctCfg struct {
	LedgerAcctName string `mapstructure:"ledger_account_name"`
	CsvDateFormat  string `mapstructure:"csv_date_format"`
	DateCol        int    `mapstructure:"date_col"`
	DescCol        int    `mapstructure:"desc_col"`
	MoneyCols      []int  `mapstructure:"money_cols"`
	NegateAmt      bool   `mapstructure:"negate_amount"`
	NoteCols       []int  `mapstructure:"note_cols"`
	Currency       string `mapstructure:"currency"`
	HeaderRow      int    `mapstructure:"header_row"`
}

type csvLedgerNameLookup struct {
	Search             string `mapstructure:"search"`
	AcctName           string `mapstructure:"account_name"`
	Description        string `mapstructure:"description"`
	DiscardTransaction bool   `mapstructure:"discard_transaction"`
}

func (r *CSVRunner) GenerateLedgerEntries(csvStream io.Reader, mappedAcct string) error {
	defer func() {
		if err := r.viper.WriteConfig(); err != nil {
			r.logger.WithError(err).Warn("Unable to update config file. This may result in duplicate transactions in the next run.")
		}
	}()

	var mappedCfg csvMappedAcctCfg
	csvMappedActKey := fmt.Sprintf("csv.account.%s", mappedAcct)

	// generate a stub if the "csv.account.<mappedAct>" value isn't set
	if !r.viper.IsSet(csvMappedActKey) {
		r.logger.Warnf("The '%s' configuration key for this CSV file has not been created, so I went ahead and created a generic configuration for you. Look through your config file to make sure it is correct and re-run this program again.", csvMappedActKey)
		mappedCfg := &csvMappedAcctCfg{
			LedgerAcctName: "Assets:Bank",
			CsvDateFormat:  "2-Jan-2006",
			DateCol:        1,
			DescCol:        2,
			MoneyCols:      []int{3},
			NegateAmt:      false,
			NoteCols:       []int{4, 5},
			Currency:       "eur",
			HeaderRow:      0,
		}

		var cfg map[string]interface{}
		err := mapstructure.Decode(mappedCfg, &cfg)
		if err != nil {
			r.logger.WithError(err).Errorf("Unable to decode mapped configuration key %s", csvMappedActKey)
			return nil
		}
		r.viper.Set(csvMappedActKey, cfg)
		r.progressBar.SetTotal(1, true)
		return nil
	}

	err := r.viper.UnmarshalKey(csvMappedActKey, &mappedCfg)
	if err != nil {
		r.logger.WithError(err).Errorf("Unable to decode configuration key %s", csvMappedActKey)
		return err
	}
	r.logger.Debugf("Decoded config key %s to val: %#v", csvMappedActKey, mappedCfg)

	var nameLookupList []csvLedgerNameLookup
	err = r.viper.UnmarshalKey("ledger_account_lookups", &nameLookupList)
	if err != nil {
		r.logger.WithError(err).Errorf("Unable to decode configuration key %s", "ledger_account_lookups")
		return err
	}
	r.logger.Debugf("Decoded lookup list key %s to val: %#v", "ledger_account_lookups", nameLookupList)

	var lineCtr int = 0
	data := csv.NewReader(csvStream)
	for {
		lineCtr++
		r.progressBar.Increment()
		record, err := data.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			r.logger.WithError(err).Error("Unable to process CSV file")
			return err
		}

		if err := r.processCSVRecord(record, lineCtr, csvMappedActKey, &mappedCfg, &nameLookupList); err != nil {
			return err
		}
	}

	// Write back the lookup list with any new found values
	var cfg []map[string]interface{}
	err = mapstructure.Decode(nameLookupList, &cfg)
	if err != nil {
		r.logger.WithError(err).Errorf("Unable to decode mapped configuration key %s", "ledger_account_lookups")
		return nil
	}
	r.viper.Set("ledger_account_lookups", cfg)

	r.progressBar.SetTotal(int64(lineCtr), true)
	r.logger.Debug("CSV processing successful")
	return nil
}

func (r *CSVRunner) processCSVRecord(record []string, lineNumber int, mappedKey string, cfg *csvMappedAcctCfg, lookupList *[]csvLedgerNameLookup) error {
	r.logger.Debugf("Processing CSV record: %#v Supplied config: %#v", record, cfg)

	if cfg.HeaderRow > 0 && lineNumber == cfg.HeaderRow {
		r.logger.Debugf("Skipping record marked as header row: %#v", record)
		return nil
	}

	// Cursory out of bounds checking
	oobs := []int{cfg.DateCol, cfg.DescCol}
	oobs = append(oobs, cfg.NoteCols...)
	oobs = append(oobs, cfg.MoneyCols...)
	if !doesBasicOOBCheckPass(oobs, len(record)) {
		return fmt.Errorf("There are currently %d columns in the CSV record '%v'. One of more of the columns specified in the config key '%s' are out of range.", len(record), record, mappedKey)
	}

	if cfg.DateCol < 1 {
		return fmt.Errorf("Invalid date column '%v'", cfg.DateCol)
	}
	date, err := time.Parse(cfg.CsvDateFormat, record[cfg.DateCol-1])
	if err != nil {
		r.logger.WithError(err).Errorf("Unable to parse the date value from column %d. Full CSV record: %v", cfg.DateCol, record)
		return err
	}

	moneyValue, err := r.coerceMoneyValue(record, cfg.MoneyCols)
	if err != nil {
		r.logger.WithError(err).Errorf("Unable to parse the money value from columns %v. Full CSV record: %v", cfg.MoneyCols, record)
		return err
	}
	r.logger.Debugf("Money coercion result: %v Full CSV record: %#v", moneyValue, record)

	primaryAcctName := cfg.LedgerAcctName
	if primaryAcctName == "" {
		primaryAcctName = "Assets:Bank"
	}

	if cfg.DescCol < 1 {
		return fmt.Errorf("Invalid description column '%v'", cfg.DescCol)
	}
	description := record[cfg.DescCol-1]

	currency := cfg.Currency
	if currency == "" {
		currency = "eur"
	}

	var acctLookup *csvLedgerNameLookup
	for _, val := range *lookupList {
		matches, err := regexp.MatchString(val.Search, description)
		if err != nil {
			return err
		}

		if matches {
			acctLookup = &val
			break
		}
	}
	if acctLookup == nil {
		acctLookup = &csvLedgerNameLookup{
			Search:             description,
			AcctName:           "Expenses:Unknown",
			Description:        description,
			DiscardTransaction: false,
		}
		r.logger.Debugf("Updating lookup list '%s' with newly found entry %#v", "ledger_account_lookups", acctLookup)
		*lookupList = append(*lookupList, *acctLookup)
	}

	if acctLookup.DiscardTransaction {
		r.logger.Debugf("Discarding record %#v as per lookup config %#v", record, acctLookup)
		return nil
	}

	transactionLines := []TransactionPosting{
		{
			Account:  primaryAcctName,
			Amount:   moneyValue,
			Currency: currency,
		},
		{
			Account:  acctLookup.AcctName,
			Amount:   Zero().Neg(moneyValue),
			Currency: currency,
		},
	}

	if cfg.NegateAmt {
		transactionLines = []TransactionPosting{
			{
				Account:  primaryAcctName,
				Amount:   Zero().Neg(moneyValue),
				Currency: currency,
			},
			{
				Account:  acctLookup.AcctName,
				Amount:   moneyValue,
				Currency: currency,
			},
		}
	}

	tr, err := NewLedgerTransaction(date, acctLookup.Description, transactionLines)
	if err != nil {
		return err
	}

	for _, noteCol := range cfg.NoteCols {
		if noteCol > 0 {
			tr.AddComment(record[noteCol-1])
		}
	}

	tr.SetDateFormat(r.viper.GetString("date_format_string"))
	fmt.Fprintln(r.outputWriter, tr.String())

	return nil
}

func doesBasicOOBCheckPass(values []int, recordLength int) bool {
	for _, v := range values {
		if v < 0 || v > recordLength {
			return false
		}
	}
	return true
}

func (r *CSVRunner) coerceMoneyValue(record []string, moneyCols []int) (*big.Float, error) {
	if len(moneyCols) < 1 || len(moneyCols) > 2 {
		return nil, fmt.Errorf("You should have only 1 or 2 designated 'money_cols'")
	}

	// Consider the first column a debit and the second column a credit. The end
	// result can always be inversed by supplying the the 'negate_amount'
	// argument.

	r.logger.Debugf("Using configured money columns %v to parse record %#v", moneyCols, record)

	if moneyCols[0] < 1 {
		return nil, fmt.Errorf("Invalid money column '%v'", moneyCols[0])
	}
	rawDebitVal := record[moneyCols[0]-1]
	r.logger.Debugf("Raw debit value: %v", rawDebitVal)

	debit := Zero()
	var err error
	if len(moneyCols) == 1 {
		// Do not automatically negate values in single money column scenarios
		debit, err = r.parseRawMoneyValue(rawDebitVal, false)
	} else {
		debit, err = r.parseRawMoneyValue(rawDebitVal, true)
	}
	if err != nil {
		return nil, err
	}
	r.logger.Debugf("Parsed debit value: %v", debit)

	credit := Zero()
	if len(moneyCols) == 2 {
		if moneyCols[1] < 1 {
			return nil, fmt.Errorf("Invalid money column '%v'", moneyCols[1])
		}
		rawCreditVal := record[moneyCols[1]-1]
		r.logger.Debugf("Raw credit value: %v", rawCreditVal)
		credit, err = r.parseRawMoneyValue(rawCreditVal, false)
		r.logger.Debugf("Parsed credit value: %v", credit)
		if err != nil {
			return nil, err
		}
	}

	// Return the sum of debit & credit
	return Zero().Add(debit, credit), nil
}

func (r *CSVRunner) parseRawMoneyValue(rawval string, isDebit bool) (*big.Float, error) {
	// Treat empty strings as a 0
	if len(rawval) < 1 {
		return Zero(), nil
	}

	if !utf8.ValidString(rawval) {
		return nil, fmt.Errorf("Value '%v' does not appear to be a valid money representation", rawval)
	}

	// Tease out a leading - or + sign (implying positive or negative), if the
	// string begins with such a value
	var isNegated bool = false
	var negSign rune = '-'
	var posSign rune = '+'
	ru, size := utf8.DecodeRuneInString(rawval)
	if ru == negSign || ru == posSign {
		rawval = rawval[size:]
		if ru == negSign {
			isNegated = true
		}
	}

	// Debit columns, i.e. on the left, are negated by default
	if isDebit {
		isNegated = true
	}

	// Tease out everything except decimal digits (effectively strips currency
	// symbols, commas, etc)
	var runeRes []rune
	for _, v := range []rune(rawval) {
		if unicode.IsDigit(v) || v == '.' {
			runeRes = append(runeRes, v)
		}
	}

	// Now that we have something hopefully resembling a number, try and parse it
	// into a Float
	f, _, err := Zero().Parse(string(runeRes), 10)
	if err != nil {
		return nil, err
	}

	// Finally, negate the value if it was originally specified as a negative
	// number
	if isNegated {
		return f.Neg(f), nil
	}

	return f, nil
}

func Zero() *big.Float {
	r := big.NewFloat(0.0)
	r.SetPrec(64)
	return r
}
