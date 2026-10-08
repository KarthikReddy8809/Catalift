# Sample launch: spring ethnic wear

Exercises the category check, the product statuses and the folder picker.
Made-up products; brands and SKUs are invented.

- `products.csv`: 10 rows across kurta, kurti, salwar suit, sherwani and lehenga, plus one jeans row.
  `price` is in rupees.
- `photos/`: 10 drawn garments in two subfolders (`phool-haat/`, `rang-ghar/`), in JPEG, PNG and
  WebP, plus `README-photographer.txt`, which is not a photo.

## What to expect

1. Sign in as the **seller** (reviewers no longer see Upload). Upload `products.csv`:
   9 of 10 rows load. Row 11, RG-8005, is rejected: `category "jeans" is not one Catalift handles`.
2. In Photos, **Choose folder** and pick `photos`: "10 photos ready from photos · 1 other files left out".
3. Upload: 10 photos attach. PH-7001 and RG-8002 get two each. RG-8003 is listed under
   "Products still without a photo".
4. Products: RG-8003 shows **Needs photo**; the other 8 show **Uploaded**.
   Both brands are new, so tick "Use a neutral voice" and Generate listings. The 8 products move to
   **Enriching**, then **Ready for review**: 8 AI calls, 16 listings. RG-8003 is skipped.
5. Sign in as the **reviewer**: Review shows 8 product rows. Approve, then Export.

Each SKU loads only once. To run it again, change the SKUs and rename the photos to match.

| SKU | Category | Colour | Pattern | Sleeve | Photos |
| --- | --- | --- | --- | --- | --- |
| PH-7001 | kurta | pink | floral | three-quarter | front, back (JPEG) |
| PH-7002 | kurti | teal | dots | short | PNG |
| PH-7003 | kurta | navy | stripes | full | WebP |
| PH-7004 | salwar suit | ochre | floral | full | JPEG |
| PH-7005 | kurti | ivory | solid | short | JPEG |
| RG-8001 | sherwani | brown | solid | full | JPEG |
| RG-8002 | kurta | light green | stripes | three-quarter | front, side (PNG) |
| RG-8003 | kurta | | | | none (shows Needs photo) |
| RG-8004 | lehenga | crimson | floral | short | WebP |
| RG-8005 | jeans | | | | rejected at upload |
