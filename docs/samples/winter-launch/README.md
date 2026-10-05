# Sample launch: winter kurtas

A clean run through the whole flow: every row and every photo is valid.
Made-up products; brands and SKUs are invented. For a run that shows row
errors and an unmatched photo, use `../autumn-launch/` instead.

- `products.csv`: 8 products, 4 from Indigo Loom and 4 from Monsoon Bazaar; `price` is in rupees.
- `photos/`: 9 drawn kurtas, one per SKU plus a back view for WL-3001.

## What to expect

1. Upload `products.csv`: 8 of 8 rows become products, no errors.
2. Select all 9 files in `photos/`: 9 attach, none unmatched, no product left without a photo.
3. Products: Monsoon Bazaar is a new brand with no voice note, so tick "Use a neutral voice", then Generate listings.
   That gives 16 listings, one per product for each of the two channels.
4. Review: open a listing, edit or rewrite a field, tick the passing rows, Approve.
5. Export: download one CSV per channel.

Each SKU loads only once. To run it again, change the SKUs (for example WL-3101) and rename the photos to match.

| SKU | Colour | Pattern | Sleeve |
| --- | --- | --- | --- |
| WL-3001 | navy | solid | full |
| WL-3002 | maroon | floral | three-quarter |
| WL-3003 | green | stripes | full |
| WL-3004 | grey | dots | short |
| MB-4001 | mustard | stripes | three-quarter |
| MB-4002 | purple | floral | full |
| MB-4003 | teal | solid | short |
| MB-4004 | red | dots | three-quarter |
