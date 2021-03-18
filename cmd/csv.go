package cmd

import (
	"fmt"
	"os"

	slc "github.com/marvinpinto/slc/lib"
	cobra "github.com/spf13/cobra"
)

var (
	mappingFlag string
	inpCSVFile  string
)

func init() {
	csvCmd.Flags().StringVar(&mappingFlag, "mapping", "", "Name of the CSV account settings key (required)")
	csvCmd.Flags().StringVarP(&inpCSVFile, "csv-input", "i", "", "CSV file to parse (required)")
	csvCmd.MarkFlagRequired("mapping")
	csvCmd.MarkFlagRequired("csv-input")
	rootCmd.AddCommand(csvCmd)
}

var csvCmd = &cobra.Command{
	Use:     "csv",
	Short:   "Create Ledger entries from your CSV files (bank, credit card, etc)",
	Example: `slc csv -o transactions.ledger --mapping "amro-mastercard" -i amro.csv`,
	Args:    cobra.NoArgs,
	RunE:    runCSVCmd,
}

func runCSVCmd(cmd *cobra.Command, args []string) error {
	csvData, err := os.Open(inpCSVFile)
	if err != nil {
		logger.WithError(err).Errorf("Unable to open CSV file %s", inpCSVFile)
		return err
	}
	defer csvData.Close()

	if mappingFlag == "" {
		return fmt.Errorf("The --mapping argument cannot be empty")
	}

	r := slc.NewCSVRunner(ledgerOutputDest, viper, logger, progressBar)
	if err := r.GenerateLedgerEntries(csvData, mappingFlag); err != nil {
		logger.WithError(err).Error("Unable to process your CSV file for ledger entries")
		return err
	}

	logger.Debug("Ledger CLI ledger entries successfully generated")
	return nil
}
