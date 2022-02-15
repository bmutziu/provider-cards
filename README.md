# provider-cards

`provider-cards` is a [Crossplane](https://crossplane.io/) Provider for 
generating Decks of Playing Cards. It comes with the following resources:

- A `ProviderConfig` which is unique per-deck.
- A `Card` resource type that represents a single playing-card.
- A `Deck` resource type that represents a shuffled deck of 52 cards.
- A managed resource controller for each of the above Resources and maintains
  the state of the Deck in a Secret file in the crossplane namespace.

## Developing

1. Use this repository as a template to create a new one.
1. Find-and-replace `provider-cards` with your provider's name.
1. Run `make` to initialize the "build" Make submodule we use for CI/CD.
1. Run `make reviewable` to run code generation, linters, and tests.
1. Replace `Card` with your own managed resource implementation(s).

Refer to Crossplane's [CONTRIBUTING.md] file for more information on how the
Crossplane community prefers to work. The [Provider Development][provider-dev]
guide may also be of use.

[CONTRIBUTING.md]: https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md
[provider-dev]: https://github.com/crossplane/crossplane/blob/master/docs/contributing/provider_development_guide.md