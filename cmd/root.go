package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	slc "github.com/marvinpinto/slc/lib"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	cobra "github.com/spf13/cobra"
	viperlib "github.com/spf13/viper"
	stripe "github.com/stripe/stripe-go/v72"
	stripeClient "github.com/stripe/stripe-go/v72/client"
	mpb "github.com/vbauerster/mpb/v6"
	decor "github.com/vbauerster/mpb/v6/decor"
)

var (
	Version        = "latest"
	logger         *log.Entry
	cfgFile        string
	verbose        bool
	outputFile     string
	viper          *viperlib.Viper
	nonInteractive bool

	rootCmd = &cobra.Command{
		Use:     "slc",
		Short:   "A CLI client to convert your Stripe account payouts into ledger entries",
		Long:    "A CLI client to convert your Stripe account payouts into ledger entries",
		Example: "slc --config config.yml -o stripe-payouts.ledger",
		Version: Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSLC()
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		cobra.CheckErr(err)
	}
}

func init() {
	logger = log.WithFields(log.Fields{"ver": Version, "name": "slc"})
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.slc.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&outputFile, "output-file", "o", "", "where to write the ledger output (default is stdout)")
	rootCmd.PersistentFlags().BoolVar(&nonInteractive, "non-interactive", false, "enable non-interactive mode (no colors, progress bars, etc)")
}

func initConfig() {
	viper = viperlib.New()
	viper.SetDefault("date_format_string", "2006-01-02")

	if nonInteractive {
		log.SetFormatter(&log.TextFormatter{
			DisableColors: true,
		})
	}

	log.SetLevel(log.InfoLevel)
	if verbose {
		log.SetLevel(log.DebugLevel)
	}

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		cobra.CheckErr(err)
		viper.AddConfigPath(home)
		viper.SetConfigName(".slc")
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("SLC")

	if err := viper.ReadInConfig(); err != nil {
		cobra.CheckErr(err)
	}
	logger.Debugf("Using config file: %s", viper.ConfigFileUsed())
}

func runSLC() error {
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

	var ledgerDest io.Writer = os.Stdout
	if outputFile != "" {
		f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		ledgerDest = f
	}

	var p *mpb.Progress
	var bar slc.ProgressBar

	if nonInteractive {
		bar = &slc.StubProgressBar{}
	} else {
		p = mpb.New(
			mpb.WithWidth(60),
			mpb.WithRefreshRate(180*time.Millisecond),
		)

		bar = p.AddBar(100,
			mpb.PrependDecorators(
				decor.Name("stripe payout", decor.WCSyncSpaceR),
				decor.CountersNoUnit("%d / %d (est)", decor.WCSyncWidth),
			),
			mpb.AppendDecorators(decor.Percentage()),
		)
	}

	r := slc.NewRunner(sc, ledgerDest, viper, logger, bar)
	if err := r.GenerateLedgerEntries(); err != nil {
		logger.WithError(err).Error("Unable to download & process your Stripe payouts")
		return err
	}
	if !nonInteractive {
		p.Wait()
	}

	logger.Debug("Ledger CLI ledger entries successfully generated")
	return nil
}
