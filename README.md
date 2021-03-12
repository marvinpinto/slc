Automatically generate Ledger entries from transactions in your Stripe account.

``` bash
$ slc --help
A CLI client to convert your Stripe account payouts into ledger entries

Usage:
  slc [flags]

Examples:
slc --config config.yml -o stripe-payouts.ledger

Flags:
      --config string        config file (default is $HOME/.slc.yaml)
  -h, --help                 help for slc
      --non-interactive      enable non-interactive mode (no colors, progress bars, etc)
  -o, --output-file string   where to write the ledger output (default is stdout)
  -v, --verbose              enable verbose output
      --version              version for slc
```

This CLI program uses the Stripe API to generate [Ledger](https://www.ledger-cli.org) entries from transactions in your Stripe account. It primarily focuses on [trasactions associated with accepting payments](https://stripe.com/docs/reports/reporting-categories#group-charge_and_payment_related) - e.g.: charges, refunds, and disputes.

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

## Getting Started

Download a pre-compiled binary from the [releases](https://github.com/marvinpinto/slc/releases) page and save it as `slc` somewhere in your PATH. You will also need your [Stripe API Key](https://stripe.com/docs/keys).

Create an empty config file for the program. Among other settings, it will use this config file to save the ID of the last processed payout, so as not to produce duplicate entries.

``` bash
touch config.yml
```

Create the `SLC_STRIPE_API_KEY` environment variable for your Stripe API key.

``` bash
export SLC_STRIPE_API_KEY=sk_....
```

Finally, run `slc` for the first time. You should see all your ledger entries populated in `stripe.ledger`.

``` bash
slc --config ./config.yml -o stripe.ledger
```

## Config File

``` yaml
# This is the string used to format ledger dates. For example, the string
# 2006-01-02 will generate ledger header lines with dates that look like:
#
# 2021-02-27 * Stripe Payout
#
# Read https://golang.org/pkg/time/#Time.Format very carefully for instructions
# on how to customize this value.
date_format_string: "2006-01-02"

# This is the map containing all your Ledger account names. Whatever values you
# use here will be the values used in your generated Ledger entries.
ledger_accounts:
  income: Income:Stripe
  stripe_fees: Expenses:Stripe Fees
  # .. and more

misc:
  # Optionally add your customer's location metadata to your Ledger entries. See
  # the questions section of the README for details.
  add_customer_metadata: true

# This key is used to store the Stripe pagination cursor in order to avoid
# duplicates.
most_recently_processed_payout: po_abcd1234
```

## Questions

#### Why are the Ledger transaction ordered newest to oldest, can I change that?

The Stripe API provides `payout` items ordered from newest to oldest - there does not appear to be a way to retrieve items starting from the very first (and paginating forward). This program saves the ID of the newest `payout` it sees and uses this ID as the starting cursor for next time. This helps in preventing duplicates Ledger entries.

As a post-processing step, you can format and sort your output ledger file however you wish. Here is an example sorting by date:

``` bash
ledger --no-pager --date-format "%Y-%m-%d" -f stripe.ledger --sort d print > temp.ledger
mv temp.ledger stripe.ledger
rm -f temp.ledger
```

#### How is revenue calculated, is sales tax taken into account?

If you use the [Stripe Tax Rates](https://stripe.com/docs/billing/taxes/tax-rates) feature and if one (or more) charges associated with a payout are from your customers, the total tax rate is calcualated using the data from the charge. This even takes into account currency conversions - where you charge a customer in X currency but are paid out in Y currency.

This potentially makes tax remittance much easier as you can track exactly how much you are liable for.

#### Where does the data for the customer metadata fields come from?

These metadata fields are looked up when the program comes across a charge (from a payout). They mostly apply to credit card charges and only if you already collect this information as part of your flow.

In terms of Ledger, this is useful information to track & query Sales Tax Nexus for jurisdictions where you have not yet registered and might be liable.

#### I use different account names in my Ledger entries, can I change these default names?

Every account name the program uses is mapped under the `ledger_accounts` key in the config file. This includes any bank accounts and tax rates it comes across. If you're not sure how to format the lookup, run the program and it will generate a config file with all the accounts it's come across.

It also emits a warning if it comes across a new account you haven't yet mapped (with the key), so you can add it yourself.

#### I found a bug, how do I report it?

Report all bugs in the [Issues](https://github.com/marvinpinto/slc/issues) section of the repo. When reporting bugs, it would be very helpful to attach Stripe payloads as well as Ledger actual & expected output. Bonus points for submitting a PR with failing tests!

Keep in mind that we are all volunteers here so if a bug becomes too hard to reproduce it will probably be de-prioritized.

#### This is awesome! Could you add feature X?

Building on the "volunteers" idea from the previous question, the answer to this may be yes or no. It will really depend on what the feature is.

If you decide you'd like to implement feature X yourself, please open an issue to discuss it before submitting a PR. This may not be a feature we're willing to maintain and so the answer may still be no. We've tried to simplifiy the build and so you should be able to compile and this feature for yourself!

#### How do builds & releases work?

All [binaries](https://github.com/marvinpinto/slc/releases) for this program are built automatically using GitHub Actions. Every commit that lands on `main` triggers an automatic build & a tagged release called `latest`.

Every tag triggers an automatic build with a corresponding tagged release. If you don't wish to live on the bleeding edge, use one of these stable tagged releases.
