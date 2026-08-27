# Adding a Pack Crack recipe

This is the authoring contract for adding another set from an official Wizards of the Coast product-breakdown article. It is written so a developer or an LLM can start with the article URL, make the recipe auditable, and fail closed when local card data does not match the modeled pools.

The simulator does not scrape an article at runtime. Wizards articles are editorial documents whose table shapes and wording vary. A recipe is a reviewed translation of disclosed facts into code, not a guess generated during a request.

## Inputs

Collect these before editing:

- The official `https://magic.wizards.com/en/news/...` article URL.
- Primary set code, any Commander/supplemental set codes, and release date.
- Every supported physical product and its exact card count.
- The ordered slots for each product.
- Every published slot probability, finish, language restriction, and conditional replacement.
- Collector-number ranges or exact collector numbers for each treatment.
- Statements whose rate is not disclosed, including “less than 1%,” numbered print runs, and “extremely rare.”

Keep research notes that distinguish these three categories:

1. Exact published rates.
2. A published structure with rounded, conditional, or aggregate rates.
3. Undisclosed rates that cannot be derived.

Never promote category 2 or 3 to an exact rate.

## Files and extension points

- `internal/web/pack_opening.go`
  - Add one named `packOpeningSource...` constant.
  - Add the set and product recipes to `packOpeningSetConfigs`.
  - Add only reviewed products to `packOpeningLaunchProducts`, with an explicit accuracy tier, summary, and limitations. A set becomes visible automatically when it has at least one launch product; there is no second set-level allowlist.
- `internal/web/pack_opening_recipe_framework.go`
  - Add collector-number treatment pools to `packOpeningCollectorPoolRules`.
  - Use numeric `Numbers` spans for plain numeric collector numbers.
  - Use canonical lowercase `ExactNumbers` for identifiers such as `12a`. Non-numeric identifiers are first-class and are not coerced.
  - Put all buckets for the same number range on one rule. Cross-rule numeric overlap is rejected so one printing cannot be accidentally counted twice.
- `internal/web/pack_opening_test.go`
  - Add focused pool membership, exact published-rate, and omitted-headliner tests.
- `internal/web/pack_opening_recipe_framework_test.go`
  - The shared static validator covers source provenance, publication metadata, slot shape, weights, finishes, pool sizes, and collector-number selectors.

## Translate the article

### 1. Model slots before probabilities

Write down each physical card slot in order. Repeated slots should use `repeatPackSlot` or `repeatWeightedPackSlot`. `CardCount` must equal the number of generated slots; validation rejects a mismatch.

Every slot must have exactly one card source:

```go
packSlot{Label: "Rare or Mythic", Bucket: "set:abc:default:rare"}

packSlot{
    Label: "Rare or Mythic",
    Weighted: wb(
        "set:abc:default:rare", 875,
        "set:abc:default:mythic", 125,
    ),
}
```

Use `FinishWeighted: wf(...)` only when the article describes a finish choice inside the same card slot. Supported finish names are `nonfoil`, `foil`, and `etched`.

### 2. Convert only disclosed rates

Weights are relative integers within one `wb` or `wf` call. Choose a denominator that preserves the source precision: percentages can use 100 or 1,000; hundredths of a percent usually need 10,000. Do not round a small rate to zero. The helpers panic on malformed pairs, empty names, non-integer weights, or non-positive weights, and static validation rejects duplicate branches.

For a conditional statement, calculate the unconditional rate only when all required factors are disclosed. Record the derivation in the test or nearby recipe comment. If the article gives an aggregate remainder and identifies every eligible treatment, that remainder may be distributed according to the disclosed card counts; say so in `Limitations`. If it merely says “less than 1%,” “extremely rare,” or gives a numbered print run without the product print run, omit that branch and disclose the omission. Never invent a denominator.

Use these accuracy tiers truthfully:

- `packOpeningAccuracySourced`: slot structure and modeled rates are directly supported by the official article.
- `packOpeningAccuracyStructure`: product structure is sourced, but one or more rates/correlations are simplified or unavailable.
- `packOpeningAccuracyBasicRarity`: a clearly labeled fallback rarity simulation, not product collation.

Every launched product must explicitly provide `Accuracy`, `AccuracySummary`, and at least one `Limitations` entry. Validation does not permit an empty accuracy value to silently become sourced.

### 3. Define narrow card pools

Inspect the locally synced Scryfall printings before naming a pool. Collector number, set code, rarity, finish, booster flag, treatment metadata, promo type, release date, and language all matter. Do not use a broad `set:abc:rare` bucket for a showcase-only slot.

Collector-number example:

```go
"abc": {
    {
        Numbers: []packOpeningCollectorNumberSpan{
            {First: 1, Last: 180},
            {First: 187, Last: 188},
        },
        ExactNumbers: []string{"181a"},
        Targets: []packOpeningCollectorPoolTarget{
            {Bucket: "set:abc:main", WithRarity: true},
        },
    },
},
```

Use separate spans to express exclusions. Do not create overlapping rules; add multiple `Targets` to one rule when a printing intentionally belongs to an aggregate pool and a treatment pool. `OnlyRarities` can restrict an aggregate target to, for example, `[]string{"uncommon"}`. `TypeContains: "land"` is available when a numeric range must also be protected by type.

Use `ExpectedPools` when the article/gallery and local data establish an exact eligible-card count. The product will be hidden if a later data sync adds or removes a printing, preventing silent probability drift. Use `RequiredPools` only for a genuine lower bound; it deliberately allows extra cards.

If a product includes non-English cards, update the language exception in `loadPackOpeningCandidates` and add a test for the exact set, language, and collector numbers. Never admit an entire non-English set to solve a five-card language treatment. Document any local bulk-data shortfall in `Limitations`.

### 4. Publish product by product

Adding a recipe to `packOpeningSetConfigs` does not expose it. Add a product to `packOpeningLaunchProducts` only after its pools and probability tests pass. A Play Booster can launch while its Collector Booster remains configured but hidden.

Use the official article for `SourceURL`. Static validation requires HTTPS, the exact `magic.wizards.com` hostname, and a news path. Product artwork is not required: the sealed wrapper is generated from the set symbol, set name, and booster type so every new recipe inherits the same visual system automatically.

## Required tests

At minimum, add tests that prove:

- `CardCount` equals the generated slot count.
- Collector-number boundaries, exclusions, and suffixed exact numbers map to only the intended pools.
- Every published weighted branch exists; missing branches fail closed instead of being renormalized away.
- Any probability called exact matches its published rate in a deterministic Monte Carlo test with a justified tolerance, or through direct weight assertions when that is clearer.
- Undisclosed headliners or treatments are absent from both slots and required/expected pools.
- Finish selection produces the corresponding displayed price.
- Language exceptions admit only the intended printings.

Run the fast checks:

```sh
gofmt -w internal/web/pack_opening.go internal/web/pack_opening_recipe_framework.go internal/web/pack_opening_test.go internal/web/pack_opening_recipe_framework_test.go
go test ./internal/web
go test ./...
go vet ./...
git diff --check
```

Then run the local-data preflight against the same database the app uses:

```sh
PACK_OPENING_PREFLIGHT_DATABASE_URL='postgres://manatomb:manatomb@localhost:5432/manatomb?sslmode=disable' \
  go test ./internal/web -run PackOpeningDatabasePreflight -v
```

The preflight verifies every launched slot against current local card data and opens 100 deterministic packs per launched product. Treat a missing branch, insufficient pool, duplicate printing/finish, or changed exact pool size as a launch blocker. It does not reproduce factory print sheets or prove correlations Wizards does not publish; those remain explicit limitations.

## Definition of done

- Official source is linked; the generated wrapper resolves the set symbol from the configured set code.
- Slot order and count match the physical product.
- Rates are traceable to disclosed facts; derivations and approximations are recorded.
- Treatment pools use exact set/range/finish/language boundaries.
- Known exact pool sizes fail closed through `ExpectedPools`.
- Unsupported or unknowable branches are omitted and disclosed.
- The static validator, focused tests, full suite, vet, diff check, and database preflight pass.
- Only audited products are added to `packOpeningLaunchProducts`.
