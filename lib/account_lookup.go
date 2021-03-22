package lib

import (
	"regexp"

	mapstructure "github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
	viperlib "github.com/spf13/viper"
)

type lookupItem struct {
	Search             string `mapstructure:"search"`
	AcctName           string `mapstructure:"account_name"`
	Description        string `mapstructure:"description"`
	DiscardTransaction bool   `mapstructure:"discard_transaction"`
}

type ledgerAccountLookup struct {
	list   []lookupItem
	viper  *viperlib.Viper
	logger *log.Entry
}

func initializeLookupList(l *log.Entry, v *viperlib.Viper) (*ledgerAccountLookup, error) {
	var list []lookupItem

	if err := v.UnmarshalKey("ledger_account_lookups", &list); err != nil {
		l.WithError(err).Errorf("Unable to decode configuration key %s", "ledger_account_lookups")
		return nil, err
	}
	l.Debugf("Decoded lookup list key %s to val: %#v", "ledger_account_lookups", list)

	return &ledgerAccountLookup{
		list:   list,
		viper:  v,
		logger: l,
	}, nil
}

func (l *ledgerAccountLookup) getOrAddItem(searchStr string, defaultAcctName string) (*lookupItem, error) {
	for _, val := range l.list {
		matches, err := regexp.MatchString(val.Search, searchStr)
		if err != nil {
			return nil, err
		}

		if matches {
			return &val, nil
		}
	}

	newItem := &lookupItem{
		Search:             searchStr,
		AcctName:           defaultAcctName,
		Description:        searchStr,
		DiscardTransaction: false,
	}
	l.logger.Debugf("Updating lookup list '%s' with new entry %#v", "ledger_account_lookups", newItem)
	l.list = append(l.list, *newItem)

	return newItem, nil
}

func (l *ledgerAccountLookup) persistData() error {
	var cfg []map[string]interface{}

	if err := mapstructure.Decode(l.list, &cfg); err != nil {
		l.logger.WithError(err).Errorf("Unable to encode mapped configuration key %s", "ledger_account_lookups")
		return err
	}

	l.viper.Set("ledger_account_lookups", cfg)
	return nil
}
