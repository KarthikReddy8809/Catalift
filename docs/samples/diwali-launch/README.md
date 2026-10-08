# Sample launch: Diwali ethnic wear

A small, clean set with SKU prefixes no other sample uses (DV- and PN-), so every
row is new to Catalift. Use it to check that the latest upload shows at the top of
Products, then to run the full flow. Made-up products; the brands (Dhaga Vastra,
Pankh) and SKUs are invented.

- `products.csv`: 10 products, 5 per brand, all valid. `price` is in rupees.
- `photos/`: 10 drawn garments, one per SKU.

## Check the latest upload shows first

1. Hard-refresh the app first (Ctrl+Shift+R), so the browser runs the newest code.
2. Sign in as the seller and **Upload** `products.csv`. The summary must read
   **10 of 10 rows loaded, 0 not loaded**. If it says "SKU already exists", these
   SKUs were uploaded before: change the prefixes (for example DV-4301, PN-4401) and
   rename the photos to match.
3. Choose the `photos/` folder: 10 attach, none unmatched, no product left without a photo.
4. Click **Products** in the sidebar (not the link on the upload screen). The first
   10 rows, page 1, are DV-4101 to DV-4105 then PN-4201 to PN-4205, in CSV order.
   Older uploads follow from page 2.
5. Sign out, sign in again, open **Products**: the same 10 rows are still on top.
6. Search `pn-42` to see only the Pankh rows.

## Then the rest of the flow

1. **Brand voice** (seller): set a tone for both brands, for example
   Dhaga Vastra "Rich and festive; name the craft first." and
   Pankh "Light and modern; keep it short.", with words to avoid such as `cheap, sale`.
2. **Products** (seller): **Generate listings**: 10 products, so 20 listings.
3. **Review** (reviewer): tick the passing rows and **Approve**.
4. **Export** (reviewer): **Export N approved listings**, then **Send to seller**.
5. **Received files** (seller): download one CSV per channel.

| SKU | Category | Colour | Pattern |
| --- | --- | --- | --- |
| DV-4101 | kurta | red | floral |
| DV-4102 | kurti | mustard | dots |
| DV-4103 | saree | purple | floral |
| DV-4104 | sherwani | cream | solid |
| DV-4105 | dupatta | pink | dots |
| PN-4201 | kurta | teal | stripes |
| PN-4202 | lehenga | orange | floral |
| PN-4203 | kurti | blue | solid |
| PN-4204 | saree | green | stripes |
| PN-4205 | salwar suit | maroon | dots |
