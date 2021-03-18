package cmd

import (
	"fmt"

	slc "github.com/marvinpinto/slc/lib"
	log "github.com/sirupsen/logrus"
	cobra "github.com/spf13/cobra"
	stripe "github.com/stripe/stripe-go/v72"
	stripeClient "github.com/stripe/stripe-go/v72/client"
)

func init() {
	rootCmd.AddCommand(stripeCmd)
}

var stripeCmd = &cobra.Command{
	Use:     "stripe",
	Short:   "Generate Ledger entries directly from your Stripe account payouts",
	Example: "slc stripe --config config.yml -o stripe-payouts.ledger",
	Args:    cobra.NoArgs,
	RunE:    runStripeCmd,
}

func runStripeCmd(cmd *cobra.Command, args []string) error {
	stripeAPIKey := viper.GetString("stripe_api_key")
	if stripeAPIKey == "" {
		return fmt.Errorf("Missing stripe_api_key. You need to set a value for the config key stripe_api_key - example: export SLC_STRIPE_API_KEY=sk_test_123")
	}

	// Create a new logrus instance to use for the Stripe client. This is
	// primarily to reduce "info" level noise (from the Stripe client).
	slogger := log.New()
	slogger.SetFormatter(logger.Logger.Formatter)
	slogger.SetLevel(log.ErrorLevel)
	if verbose {
		slogger.SetLevel(log.InfoLevel)
	}

	logger.Debug("Initializing Stripe API client")
	stripe.SetAppInfo(&stripe.AppInfo{
		Name:    "slc",
		URL:     "https://github.com/marvinpinto/slc",
		Version: Version,
	})
	stripe.DefaultLeveledLogger = slogger
	config := &stripe.BackendConfig{
		MaxNetworkRetries: stripe.Int64(5),
		EnableTelemetry:   stripe.Bool(false),
	}
	sc := &stripeClient.API{}
	sc.Init(stripeAPIKey, &stripe.Backends{
		API: stripe.GetBackendWithConfig(stripe.APIBackend, config),
	})

	r := slc.NewStripeRunner(sc, ledgerOutputDest, viper, logger, progressBar)
	if err := r.GenerateStripeLedgerEntries(); err != nil {
		logger.WithError(err).Error("Unable to download & process your Stripe payouts")
		return err
	}

	logger.Debug("Ledger CLI ledger entries successfully generated")
	return nil
}
