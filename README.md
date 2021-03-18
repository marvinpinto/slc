# SLC

`slc` is a CLI application to generate Ledger accounting entries. It works with generic CSV files as well as the Stripe API.

``` bash
$ slc --help
A CLI client to generate Ledger accounting entries - works with Stripe API as well as generic CSV files.

Usage:
  slc [command]

Available Commands:
  csv         Create Ledger entries from your CSV files (bank, credit card, etc)
  help        Help about any command
  stripe      Generate Ledger entries directly from your Stripe account payouts

Flags:
      --config string        config file (default is $HOME/.slc.yaml)
  -h, --help                 help for slc
      --non-interactive      enable non-interactive mode (no colors, progress bars, etc)
  -o, --output-file string   where to write the ledger output (default is stdout)
  -v, --verbose              enable verbose output
      --version              version for slc

Use "slc [command] --help" for more information about a command.
```

## Contents

- [Installation](#installation)
- [CSV Files](#csv-files)
- [Stripe API](#stripe-api)
- [General Configuration](#general-configuration)
- [Questions](#questions)

## Installation

Download the [latest release](https://github.com/marvinpinto/slc/releases/tag/latest) as a pre-compiled binary and save it somewhere in your PATH. Releases are automatically generated for Linux, macOS, and Windows.

<details><summary>Install slc on Linux</summary>

``` bash
curl -L -o slc "https://github.com/marvinpinto/slc/releases/download/latest/slc_linux_amd64"
sudo install -o root -g root -m 0755 slc /usr/local/bin/slc
slc --version
```

</details>

<details><summary>Install slc on macOS</summary>

``` bash
curl -L -o slc "https://github.com/marvinpinto/slc/releases/download/latest/slc_darwin_amd64"
chmod +x ./slc
sudo mv ./slc /usr/local/bin/slc
sudo chown root: /usr/local/bin/slc
slc --version
```

</details>

<details><summary>Install slc on Windows</summary>

``` bash
curl -L -o slc.exe "https://github.com/marvinpinto/slc/releases/download/latest/slc_windows_amd64.exe"
```

Add the `slc.exe` binary somewhere to your PATH, and then verify it:
``` bash
slc --version
```

</details>

## CSV Files

This application works with a wide range of CSV input files from most financial institutions. If you come across a CSV format that is not supported, open an issue (see instructions below) and we'll see what can be done.

If you're running this program against a new CSV file for the very first time, supply a new name for the **mapping** parameter and let the app generate a stub config for you.

``` bash
slc csv --config ./config.yml -o output.ledger --mapping "amro-mastercard" -i amro.csv
```

In the config file you specified (`./config.yml` in this example) you should now see a new key added, something similar to:

``` yaml
csv:
  account:
    amro-mastercard:
      ledger_account_name: "Assets:Bank"
      csv_date_format: "2-Jan-2006"
      date_col: 1
      desc_col: 2
      money_cols:
        - 3
      negate_amount: false
      note_cols:
        - 4
        - 5
      currency: "eur"
      header_row: 0
```

You should now be able to tweak the values in the config to match your CSV format and re-run the program again.

#### Configuration Details

``` yaml
csv:
  account:
    # Each new "mapping" you create will generate a new sub-key here. This
    # example uses the "amro-mastercard" mapping we previously created.
    amro-mastercard:
      # This is the name of the ledger account that will be used as the
      # "primary" account.
      ledger_account_name: "Assets:Bank"

      # Setting used to parse the date format of the CSV file. The layout must
      # reference the following date exactly:
      #
      # Mon Jan 2 15:04:05 -0700 MST 2006
      #
      # examples:
      # - "2-Jan-2006" for dates that look like "25-Dec-1999"
      # - "2006-01-02" for dates that look like "2021-02-27"
      # and so on.
      #
      # More details available at: https://golang.org/pkg/time/#Time.Format
      csv_date_format: "2-Jan-2006"

      # The column in your CSV file which contains the date. All columns start
      # at 1.
      date_col: 1

      # A column that references a description. This description will be used
      # against the "search" parameter in "ledger_account_lookups" list to
      # subtitute names, as well as discard transactions. See the General
      # Configuration section for details.
      desc_col: 2

      # A list of columns that reference the money amount values. Maximum 2
      # columns are allowed in this list.
      money_cols:
        - 3

      # Use this to "reverse" transactions, depending on how your financial
      # institution formats their CSV files.
      negate_amount: false

      # These columns will be added as transaction comments to each ledger
      # entry.
      note_cols:
        - 4
        - 5

      # The currency or commodity value ledger entries will be reported in.
      currency: "eur"

      # The line/row number of the "header" value to ignore in a CSV file. A
      # value of 0 here implies "do not ignore any rows".
      header_row: 0
```

## Stripe API

The `stripe` subcommand reconciles your Stripe payouts into Ledger entries, taking into account each charge/invoice associated with a payout and also accounting for any collected sales tax.

It focuses primarily focuses on [trasactions associated with accepting payments](https://stripe.com/docs/reports/reporting-categories#group-charge_and_payment_related) - e.g.: charges, refunds, and disputes.

For charges (and related invoices), it goes through and generates Ledger entries for each charge associated with a Stripe payout. This automatic reconciliation will probably not work if you have [automatic payouts](https://stripe.com/docs/payouts#manual-payouts) disabled in your Stripe account.

``` ledger
2021-02-27 * Stripe Payout
	; Correlates to Stripe payout po_1ITGPQCOCRzw0YkGEIImZLHC from 2021-03-09 for amount 23.06 USD
	; CustomerCity: Toronto
	; CustomerState: ON
	; CustomerCountry: CA
	; CustomerPostalCode: M8D9D3
	Liabilities:SalesTax		-2.77 USD
	Income:Stripe:Customer-cus_HueMTwXzJ6NWw2		-21.29 USD
	Expenses:Stripe Fees		1.00 USD
	Assets:Bank		23.06 USD
```

#### Initial Setup

You will need your [Stripe API Key](https://stripe.com/docs/keys) to get started. Create an environment varlable called `SLC_STRIPE_API_KEY` containing your key.

``` bash
export SLC_STRIPE_API_KEY=sk_....
```

The program will automatically read your API key from that environment variable and generate your Stripe ledger entries.

```bash
slc stripe --config ./config.yml -o stripe.ledger
```

#### Configuration Details

```yaml
# This is the map containing all your Ledger account names. Whatever values you
# use here will be the values used in your generated Ledger entries.
ledger_accounts:
  income: Income:Stripe
  stripe_fees: Expenses:Stripe Fees
  # .. and more

stripe:
  # Optionally add your customer's location metadata to your Ledger entries. See
  # the questions section of the README for details.
  add_customer_metadata: true

  # This key is used to store the Stripe pagination cursor in order to
  # avoid duplicates.
  most_recently_processed_payout: po_abcd1234
```

## General Configuration

``` yaml
# This is the string used to format ledger dates. For example, the string
# 2006-01-02 will generate ledger header lines with dates that look like:
#
# 2021-02-27 * Stripe Payout
#
# The layout must reference the following date exactly:
#
# Mon Jan 2 15:04:05 -0700 MST 2006
#
# More details available at: https://golang.org/pkg/time/#Time.Format
date_format_string: "2006-01-02"

# This is the lookup list the program will use to match account names with
# actions or substitutions.
ledger_account_lookups:

  # The "search" field uses the supplied value as a regular expression to match
  # against descriptions (for CSVs).
  #
  # You can also use the "(?i)" prefix to perform a case-insensitive match. For
  # example, replacing the below with "(?i)febo.*bv" achieves the same result.
  - search: "FEBO.*BV"

    # The "account_name" field is used as the replacement account in your
    # ledger entries.
    account_name: "Expenses:Lunch"

    # Value used as the ledger transaction description.
    description: "Quick work lunch"

    # Sometimes you end up with duplicate transactions from different accounts,
    # for example when paying a credit card bill. You can use this boolean value
    # to discard one of the transactions.
    discard_transaction: false
```

## Questions

#### Can I change the order in which transactions are displayed?

This application processes each transaction entry in the order it is received - implying that it is dependent on how your CSV files are ordered, or how the Stripe API responds to requests (usually newest first).

As a post-processing step, you can format and sort your output ledger file however you wish. Here is an example sorting by date:

``` bash
ledger --no-pager --date-format "%Y-%m-%d" -f stripe.ledger --sort d print > temp.ledger
mv temp.ledger stripe.ledger
rm -f temp.ledger
```

#### How is Stripe revenue calculated, is sales tax taken into account?

If you use the [Stripe Tax Rates](https://stripe.com/docs/billing/taxes/tax-rates) feature and if one (or more) charges associated with a payout are from your customers, the total tax rate is calcualated using the data from the charge. This even takes into account currency conversions - where you charge a customer in X currency but are paid out in Y currency.

This potentially makes tax remittance much easier as you can track exactly how much you are liable for.

#### Where does the Stripe data for the customer metadata fields come from?

These metadata fields are looked up when the program comes across a charge (from a payout). They mostly apply to credit card charges and only if you already collect this information as part of your flow.

In terms of Ledger, this is useful information to track & query Sales Tax Nexus for jurisdictions where you have not yet registered and might be liable.

#### I use different account names in my Ledger entries, can I change these default names?

The `ledger_account_lookups` key in the config file was made specifically for this purpose. You can substitute or discard any transactions you wish. 

#### I found a bug, how do I report it?

Report all bugs in the [Issues](https://github.com/marvinpinto/slc/issues) section of the repo. When reporting bugs, it would be very helpful to attach CSV files or Stripe payloads as well as Ledger actual & expected output. Bonus points for submitting a PR with failing tests!

Do note that we are all volunteers maintaining this application. As such if a bug becomes too hard to reproduce, it will likely be de-prioritized.

#### This is awesome! Could you add feature X?

Building on the "volunteers" idea from the previous question, the answer to this may be yes or no. It will really depend on what the feature is.

If you decide you'd like to implement feature X yourself, please open an issue to discuss it before submitting a PR. This may not be a feature we're willing to maintain and so the answer may still be no. We've tried to simplifiy the build and so you should be able to compile and use this feature for yourself if it is not accepted here.

#### How do builds & releases work?

All [binaries](https://github.com/marvinpinto/slc/releases) for this program are built automatically using GitHub Actions. Every commit that lands on `main` triggers an automatic build & a tagged release called `latest`.

Every tag triggers an automatic build with a corresponding tagged release. If you don't wish to live on the bleeding edge, use one of these stable tagged releases.
