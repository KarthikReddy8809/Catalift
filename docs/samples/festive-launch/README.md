# Sample launch: festive kurtas

A clean run that also exercises the folder picker. Made-up products; brands and SKUs are invented.

- `products.csv`: 10 products, 5 from Kesar Kala and 5 from Nila Threads; `price` is in rupees.
- `photos/`: 12 drawn kurtas in two subfolders (`kesar-kala/`, `nila-threads/`), in JPEG, PNG and
  WebP, plus `shoot-notes.txt`, which is not a photo.

## What to expect

1. Upload `products.csv`: 10 of 10 rows become products, no errors.
2. In Photos, click **Choose folder** and pick the `photos` folder (or drag it onto the box).
   The picker reads both subfolders: "12 photos ready from photos · 1 other files left out".
3. Upload: 12 photos attach, none unmatched, no product left without a photo. The SKU table shows
   KK-5001 and NT-6003 with two photos each.
4. Products: both brands are new and have no voice note, so tick "Use a neutral voice", then
   Generate listings. That gives 20 listings, one per product for each of the two channels.
5. Review, approve, export as usual.

Each SKU loads only once. To run it again, change the SKUs and rename the photos to match.

| SKU | Colour | Pattern | Sleeve | Photos |
| --- | --- | --- | --- | --- |
| KK-5001 | rust | floral | full | front, back (JPEG) |
| KK-5002 | teal | stripes | three-quarter | PNG |
| KK-5003 | mustard | dots | short | WebP |
| KK-5004 | maroon | solid | full | JPEG |
| KK-5005 | green | floral | three-quarter | PNG |
| NT-6001 | royal blue | solid | full | JPEG |
| NT-6002 | magenta | dots | three-quarter | WebP |
| NT-6003 | black | stripes | full | two (JPEG) |
| NT-6004 | cream | floral | short | PNG |
| NT-6005 | violet | solid | three-quarter | JPEG |
