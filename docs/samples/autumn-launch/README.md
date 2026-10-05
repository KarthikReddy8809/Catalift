# Sample launch: autumn kurtas

Made-up products for trying the whole flow locally. Brands and SKUs are invented.

- `products.csv`: 10 product rows after the header; `price` is in rupees.
- `photos/`: drawn kurta silhouettes, one or two per SKU, plus one file that matches no SKU.

## What to expect

Upload `products.csv` on the Upload page:

- 7 of 10 rows become products: IL-1001 to IL-1004 (Indigo Loom) and SR-2001 to SR-2003 (Saffron Row).
- 3 rows are rejected, each with its reason:
  - row 9, SR-2004: `price must be a positive number of rupees`
  - row 10: `sku is missing`
  - row 11, IL-1001: `SKU IL-1001 is repeated in this file (first on row 2)`

Then select every file in `photos/` and upload them:

- 8 photos attach. IL-1001 gets two (front and back).
- `holiday-banner.jpg` is listed as not matched to any SKU.
- No product is left without a photo.

On Products, neither brand has a voice note yet. Tick "Use a neutral voice" and click Generate listings.
That gives 7 products and 14 listings, one per product for each of the two channels.

Each SKU can be loaded only once. To run the flow again, change the SKUs (for example IL-1101) in
the CSV and rename the photos to match.

| SKU | Colour | Pattern | Sleeve |
| --- | --- | --- | --- |
| IL-1001 | navy | solid | full |
| IL-1002 | maroon | stripes | three-quarter |
| IL-1003 | green | dots | full |
| IL-1004 | off-white | solid | short |
| SR-2001 | orange | floral | three-quarter |
| SR-2002 | black | stripes | full |
| SR-2003 | pink | dots | short |
