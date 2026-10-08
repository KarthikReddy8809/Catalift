# Sample launch: monsoon ethnic wear

Tests the whole flow, including the Products tab's search, pagination,
the Fix drawer and discarding rejected rows. Made-up products; the brands
(Saanjh Vastra, Tanka Kala) and SKUs are invented.

- `products.csv`: 18 rows in every category Catalift accepts. 14 are valid
  and 4 are broken on purpose. `price` is in rupees.
- `photos/`: 13 drawn garments. That is one per product for 11 products,
  a back view for SV-9001, and one photo (SV-9999) that matches no row.
- `fix-photos/`: 6 photos you add later from the Fix drawer: the 3 products
  left without a photo and the 3 rejected rows you can repair.

## What to expect

1. **Upload** `products.csv`: 14 rows load and 4 are not loaded:

   | Row | SKU | Why it is not loaded | Fix in the drawer |
   | --- | --- | --- | --- |
   | 16 | SV-9008 | category "jeans" is not one Catalift handles | Category: kurta |
   | 17 | SV-9009 | price must be a positive number of rupees | Price: 1599 |
   | 18 | TK-9508 | brand is missing | Brand: Tanka Kala |
   | 19 | SV-9001 | SKU SV-9001 is repeated in this file (first on row 2) | Discard this row |

2. **Photos**: choose the `photos/` folder. 12 attach, SV-9999 is unmatched,
   and 3 products are left without a photo: SV-9006, TK-9503, TK-9506.
3. **Products tab**: 18 rows, 14 products and 4 marked "Not loaded", over 2 pages.
   - Search `tk-95` to see only the Tanka Kala rows. Search a product's ID
     (the ID column) to find exactly that product.
   - Click **Fix** on SV-9006, choose `fix-photos/SV-9006_front.jpg`, then **Validate**.
     The product saves and the AI reads it again (one vision call).
     Do the same for TK-9503 and TK-9506.
   - Click **Fix** on row 16, set Category to kurta, add `fix-photos/SV-9008_front.jpg`,
     then **Validate**: it becomes a product and the AI reads it. Then do rows 17 and 18
     with the values in the table above and their photos.
   - Click **Fix** on row 19 and choose **Discard this row**.
     **Discard all** would drop every row still marked "Not loaded".
4. **Generate listings**: both brands are new and have no voice note, so tick
   "Use a neutral voice", then Generate. With all fixes done that is 17 products,
   so 34 listings (one per product for each of the two channels).
5. **Review**: sign in as the reviewer, open listings, edit or regenerate a field,
   tick the passing rows, then use **Approve**.
6. **Export**: download one CSV per channel.

Each SKU loads only once. To run it again, change the SKU prefixes (for example
SV-9101) in the CSV and rename the photos to match.

| SKU | Category | Colour | Pattern | Photo in |
| --- | --- | --- | --- | --- |
| SV-9001 | kurta | navy | solid | photos (front and back) |
| SV-9002 | kurti | pink | floral | photos |
| SV-9003 | saree | purple | stripes | photos |
| SV-9004 | sherwani | gold | solid | photos |
| SV-9005 | dupatta | orange | dots | photos |
| SV-9006 | lehenga | red | floral | fix-photos |
| SV-9007 | salwar suit | green | floral | photos |
| TK-9501 | kurta | maroon | stripes | photos |
| TK-9502 | kurti | teal | dots | photos |
| TK-9503 | saree | teal | dots | fix-photos |
| TK-9504 | kurta | grey | solid | photos |
| TK-9505 | dupatta | mustard | floral | photos |
| TK-9506 | sherwani | ivory | solid | fix-photos |
| TK-9507 | kurta | lime | stripes | photos |
| SV-9008 | kurta (after fix) | indigo | solid | fix-photos |
| SV-9009 | kurta | yellow | dots | fix-photos |
| TK-9508 | kurta | wine | floral | fix-photos |
