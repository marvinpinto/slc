package cmd

import (
	"io"
	"os"
	"path/filepath"
	"time"

	slc "github.com/marvinpinto/slc/lib"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	cobra "github.com/spf13/cobra"
	viperlib "github.com/spf13/viper"
	mpb "github.com/vbauerster/mpb/v6"
	decor "github.com/vbauerster/mpb/v6/decor"
)

var (
	Version          = "latest"
	logger           *log.Entry
	cfgFile          string
	verbose          bool
	outputFile       string
	viper            *viperlib.Viper
	nonInteractive   bool
	ledgerOutputDest io.Writer
	progress         *mpb.Progress
	progressBar      slc.ProgressBar
	of               *os.File

	rootCmd = &cobra.Command{
		Use:               "slc",
		Short:             "A CLI client to generate Ledger accounting entries",
		Long:              "A CLI client to generate Ledger accounting entries - works with Stripe API as well as generic CSV files.",
		PersistentPreRun:  appSetup,
		PersistentPostRun: appTeardown,
		Version:           Version,
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		cobra.CheckErr(err)
	}
}

func init() {
	logger = log.WithFields(log.Fields{"ver": Version, "name": "slc"})
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.slc.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&outputFile, "output-file", "o", "", "where to write the ledger output (default is stdout)")
	rootCmd.PersistentFlags().BoolVar(&nonInteractive, "non-interactive", false, "enable non-interactive mode (no colors, progress bars, etc)")
}

func appSetup(cmd *cobra.Command, args []string) {
	viper = viperlib.New()

	if outputFile == "" || verbose {
		nonInteractive = true
	}

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
		viper.SetConfigType("yaml")
		viper.SetConfigFile(filepath.Join(home, ".slc.yaml"))
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("SLC")
	viper.ReadInConfig()
	logger.Debugf("Using config file: %s", viper.ConfigFileUsed())

	ledgerOutputDest = os.Stdout
	if outputFile != "" {
		of, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		cobra.CheckErr(err)

		// Note that this file gets closed in the teardown function
		ledgerOutputDest = of
	}

	if nonInteractive {
		progressBar = &slc.StubProgressBar{}
	} else {
		progress = mpb.New(
			mpb.WithWidth(60),
			mpb.WithRefreshRate(180*time.Millisecond),
		)

		var decorName decor.Decorator
		var decorCtr decor.Decorator
		if cmd.Name() == "stripe" {
			decorName = decor.Name("Processing stripe payouts:", decor.WCSyncSpaceR)
			decorCtr = decor.OnComplete(decor.Current(0, "# %d", decor.WCSyncWidth), "complete!")
		} else if cmd.Name() == "csv" {
			decorName = decor.Name("Processing CSV records:", decor.WCSyncSpaceR)
			decorCtr = decor.OnComplete(decor.Current(0, "# %d", decor.WCSyncWidth), "complete!")
		}

		progressBar = progress.AddSpinner(100,
			mpb.SpinnerOnLeft,
			mpb.PrependDecorators(decorName, decorCtr),
		)
	}
}

func appTeardown(cmd *cobra.Command, args []string) {
	if !nonInteractive {
		progress.Wait()
	}

	// Close the output file, if stdout wasn't used
	if outputFile != "" {
		of.Close()
	}
}
