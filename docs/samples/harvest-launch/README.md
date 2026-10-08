# Sample launch: harvest ethnic wear

A walk through the whole flow with both roles: the seller uploads, fixes rows
and sets each brand's voice; the reviewer approves, exports and sends the files;
the seller collects them under Received files. Made-up products; the brands
(Haldi Lane, Kesar Vann) and SKUs are invented.

- `products.csv`: 14 rows, 12 valid and 2 broken on purpose. `price` is in rupees.
- `photos/`: 12 drawn garments, one per product for 11 products plus a back view for HL-2101.
- `fix-photos/`: 3 photos added later from the Fix drawer: HL-2106 (loaded with no photo)
  and the two rows you repair.

## Before you start

The API runs migration 00007 and the latest code:

```bash
GOTOOLCHAIN=go1.26.8 make migrate
GOTOOLCHAIN=go1.26.8 SECURE_COOKIES=false make dev
```

Sign in with the demo accounts from `make seed-demo`: `seller@example.com` and
`reviewer@example.com`, with the password you set as `DEMO_PASSWORD`. Use two
browser windows (one private) to keep both signed in.

## As the seller

The sidebar shows Upload, Products, Brand voice and Received files.

1. **Upload** `products.csv`: 12 rows load and 2 are not loaded.

   | Row | SKU | Why it is not loaded | Fix |
   | --- | --- | --- | --- |
   | 14 | KV-2207 | category "shirt" is not one Catalift handles | Category: kurta |
   | 15 | HL-2107 | price must be a positive number of rupees | Price: 1399 |

2. **Photos**: choose the `photos/` folder. 12 attach, none unmatched, and HL-2106 is left
   without a photo.
3. **Brand voice**: set both brands before generating.

   | Brand | Tone | Words to avoid |
   | --- | --- | --- |
   | Haldi Lane | Warm, festive and concise; name the colour first. | cheap, best ever, guaranteed |
   | Kesar Vann | Calm and elegant; mention the craft. | discount, sale |

   Each **Save voice** confirms with a green "Saved".
4. **Products**: 14 rows over 2 pages.
   - Search `kv-` to see only Kesar Vann.
   - **Fix** HL-2106: add `fix-photos/HL-2106_front.jpg`, then **Validate**. The AI reads it.
   - **Fix** row 14: Category kurta, add `fix-photos/KV-2207_front.jpg`, **Validate**.
   - **Fix** row 15: Price 1399, add `fix-photos/HL-2107_front.jpg`, **Validate**.
5. **Generate listings**. Both brands now have a tone, so no neutral-voice question.
   That is 14 products, so 28 listings, one per product for each of the two channels.
6. Try `/review` or `/channels` in the address bar: you land on Products.

## As the reviewer

The sidebar shows Products, Review, Export and Channels.

1. **Review**: open a listing, edit a field, or regenerate one with an instruction.
   Tick the passing rows and **Approve**.
2. **Channels** (optional): **Edit rules** on a channel, for example add a banned word.
   Listings are checked again and any approved one that now fails loses its approval.
3. **Export**: **Export N approved listings** writes one CSV per channel.
4. Under **Send to the seller**, the new export shows "Not sent". Click **Send to seller**:
   it changes to "Sent to seller" with the time and your email.

## As the seller again

1. **Received files** lists the export the reviewer sent, with one download per channel.
   Download each CSV and check that only approved listings are in it.
2. An export the reviewer has not sent never appears here.

## Check the words-to-avoid rule

1. As the seller, in **Brand voice**, add a word you can see in a Haldi Lane listing
   (its own name, "Haldi", always works with the free local stand-in, which writes
   "Made by Haldi Lane.") and **Save voice**.
2. As the reviewer, **Review** shows those listings as failing with
   `Title uses "Haldi", a word the brand avoids.` (or the bullet or description it is in), and any that were approved need approving again.
3. Remove the word and save: the listings pass again.

Each SKU loads only once. To run it again, change the prefixes in the CSV (for example
HL-2201) and rename the photos to match.

| SKU | Category | Colour | Pattern | Photo in |
| --- | --- | --- | --- | --- |
| HL-2101 | kurta | yellow | floral | photos (front and back) |
| HL-2102 | kurti | pink | dots | photos |
| HL-2103 | saree | teal | stripes | photos |
| HL-2104 | dupatta | violet | floral | photos |
| HL-2105 | sherwani | brown | solid | photos |
| HL-2106 | kurta | teal | floral | fix-photos |
| HL-2107 | kurta | ochre | stripes | fix-photos |
| KV-2201 | kurta | green | stripes | photos |
| KV-2202 | lehenga | red | floral | photos |
| KV-2203 | salwar suit | blue | dots | photos |
| KV-2204 | kurti | orange | solid | photos |
| KV-2205 | saree | maroon | dots | photos |
| KV-2206 | kurta | slate | stripes | photos |
| KV-2207 | kurta (after fix) | purple | solid | fix-photos |
