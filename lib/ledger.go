package lib

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

type LedgerTransaction struct {
	date        time.Time
	dateFormat  string
	isCleared   bool
	description string
	comments    []string
	lines       []TransactionPosting
}

type TransactionPosting struct {
	Account  string
	Amount   *big.Float
	Currency string
}

func NewLedgerTransaction(date time.Time, desc string, lines []TransactionPosting) (*LedgerTransaction, error) {
	sum := Zero()
	for _, line := range lines {
		sum.Add(sum, line.Amount)
	}
	if !approxEquals(sum, Zero()) {
		return nil, fmt.Errorf("The items in this ledger transaction appear to be unbalanced. The amounts in these ledger posting should balance out to 0, but results in %.2f instead. Lines: %v", sum, lines)
	}

	return &LedgerTransaction{
		date:        date,
		dateFormat:  "2006-01-02",
		isCleared:   true,
		description: desc,
		comments:    []string{},
		lines:       lines,
	}, nil
}

func (l *LedgerTransaction) SetDateFormat(format string) {
	if format != "" {
		l.dateFormat = format
	}
}

func (l *LedgerTransaction) sanitizeDescription() {
	rgx := regexp.MustCompile(`\s+`)
	s := rgx.ReplaceAllString(l.description, " ")
	l.description = strings.TrimSpace(s)
}

func (l *LedgerTransaction) AddComment(comment string) {
	comment = strings.TrimSpace(comment)
	if comment != "" {
		l.comments = append(l.comments, comment)
	}
}

func (l *LedgerTransaction) AddKeyValComment(key string, val string) {
	key = strings.TrimSpace(key)
	val = strings.TrimSpace(val)
	if key != "" && val != "" {
		l.comments = append(l.comments, fmt.Sprintf("%s: %s", key, val))
	}
}

func (l *LedgerTransaction) String() string {
	var res strings.Builder

	// transaction header line: e.g. 2020-12-27 * Stripe Payout
	l.sanitizeDescription()
	clearedValue := "*"
	if !l.isCleared {
		clearedValue = "!"
	}
	res.WriteString(fmt.Sprintf(
		"%s %s %s\n",
		l.date.Format(l.dateFormat),
		clearedValue,
		l.description,
	))

	// comment lines: e.g. ; Correlates to Stripe payout...
	for _, comment := range l.comments {
		res.WriteString(fmt.Sprintf(
			"%4s; %s\n",
			"", // indent
			comment,
		))
	}

	// Find the length of the longest account name. This will be used as the
	// printf padding value.
	var acctStrLen int = 0
	for _, line := range l.lines {
		if len(line.Account) > acctStrLen {
			acctStrLen = len(line.Account)
		}
	}

	// transaction lines: e.g. Liabilities:SalesTax  -2.82 USD
	for _, line := range l.lines {
		res.WriteString(fmt.Sprintf(
			"%4s%-*s    %.2f %s\n",
			"", // indent
			acctStrLen,
			line.Account,
			line.Amount,
			strings.ToUpper(line.Currency),
		))
	}

	return res.String()
}

func (l *LedgerTransaction) formatDate(date int64) string {
	return time.Unix(date, 0).Format(l.dateFormat)
}

func (l *LedgerTransaction) formatUnitAmount(amount int64, currency string) string {
	// amount / 100
	res := Zero().Quo(Zero().SetInt64(amount), Zero().SetFloat64(100))
	return fmt.Sprintf("%.2f %s", res, strings.ToUpper(currency))
}

func Zero() *big.Float {
	r := big.NewFloat(0.0)
	r.SetPrec(64)
	return r
}

func approxEquals(a *big.Float, b *big.Float) bool {
	tolerance := Zero().SetFloat64(1e-8)

	// delta = abs(a - b)
	delta := Zero().Abs(Zero().Sub(a, b))

	// delta < tolerance ? true : false
	if delta.Cmp(tolerance) < 0 {
		return true
	}
	return false
}
