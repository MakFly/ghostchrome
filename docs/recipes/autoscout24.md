# Recipe — autoscout24.fr

Two ways to query autoscout24: the built-in subcommand (which already wraps the fast-path + Chrome fallback) and the raw `fastfetch` for ad-hoc URLs.

## Built-in subcommand

```bash
# Search — HTTP fast-path by default, Chrome fallback automatic
ghostchrome autoscout24 search \
  --make audi --model a3 \
  --year-from 2020 --price-max 25000 \
  --pages 1 \
  --output ./results.jsonl
```

Output is JSONL, one listing per line, full structure (id, price, vehicle, location, images, evBanner, …).

| Flag | Effect |
|---|---|
| `--make`, `--model`, `--year-from`, `--year-to` | Filter |
| `--price-max`, `--km-max`, `--fuel`, `--gearbox` | Filter |
| `--country F\|D\|B\|I\|NL\|E\|A` | Country, default `F` |
| `--sort standard\|age\|price\|mileage\|year` | Sort order |
| `--pages N` | Number of pages to walk |
| `--query "tesla model 3"` | Free-text alternative |
| `--force-browser` / `--no-fast` | Skip fast-path, use Chrome only |
| `--wait-ms 2000` | Sleep between Chrome navigations |

## Detail page

```bash
ghostchrome autoscout24 detail \
  audi-a3-sportback-30-tfsi-hybrid-110-s-tronic-7design-luxe-essence-gris-3310988e-cae1-4546-8050-043c17c8d1c5 \
  > detail.json

# Or with a full URL:
ghostchrome autoscout24 detail https://www.autoscout24.fr/offres/...
```

## Raw `fastfetch`

When you want to control the URL yourself or chain through `jq`:

```bash
# SERP — first 20 listings
ghostchrome fastfetch --next-data \
  'https://www.autoscout24.fr/lst/audi/a3?fregfrom=2020&priceto=25000' \
  | jq '.props.pageProps.listings[] | {url, price: .price.priceFormatted, city: .location.city}'
```

Measured: **264 ms** wall, 20 listings extracted, 0 Chrome.

## Detail with `fastfetch`

```bash
ghostchrome fastfetch --next-data \
  'https://www.autoscout24.fr/offres/<slug-with-uuid>' \
  | jq '.props.pageProps.listingDetails | {make: .vehicle.make, model: .vehicle.model, price, owner}'
```

Measured: **227 ms** wall.

## Pagination

autoscout24 uses `?page=N` (1-indexed):

```bash
for p in 1 2 3 4 5; do
  ghostchrome fastfetch --next-data \
    "https://www.autoscout24.fr/lst/audi/a3?page=$p&fregfrom=2020" \
    | jq -c '.props.pageProps.listings[] | {url, price: .price.priceFormatted}'
done
```

Or via the subcommand which handles pagination + dedup:

```bash
ghostchrome autoscout24 search --make audi --model a3 --pages 5 --output audi.jsonl
```

## Anti-bot

autoscout24 ships the DataDome SDK on every page but does NOT challenge first-fetch from a typical IP/UA combination. ghostchrome's anti-bot detector is conservative and ignores the SDK presence on normal-sized pages.

If you do hit a challenge:

```bash
ghostchrome fastfetch --fallback-browser --stealth \
  'https://www.autoscout24.fr/lst/...'
```

Or run via the subcommand which already does this internally.

## Performance summary

| Query | Method | Wall time | Listings |
|---|---|---|---|
| `audi/a3?fregfrom=2020&priceto=25000` | fastfetch | 264 ms | 20 |
| Same query | autoscout24 search (auto fast-path) | 279 ms | 20 |
| Same query | autoscout24 search `--force-browser` | 3217 ms | 20 |
| Detail `/offres/<uuid>` | fastfetch | 227 ms | 1 (full Product) |

Speedup vs Chrome on the same URL: **~11.5×**, identical output.
