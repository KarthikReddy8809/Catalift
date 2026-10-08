# Sample launch: Navratri ethnic wear

A retest of the whole flow with fresh SKUs (GH- and ZR-, used by no other
sample). It also checks that every screen shows only this upload: after it,
older products, brands, listings and exports disappear from view while
staying in the database. Made-up products; the brands (Ghungroo, Zari Rang)
and SKUs are invented.

- `products.csv`: 12 rows. 10 are new and valid, 2 are refused on purpose.
- `photos/`: 10 drawn garments for 9 products, plus a back view of GH-5101.
- `fix-photos/`: 2 photos added later from the Fix drawer (ZR-5205, ZR-5206).

## Before you start

The API and the worker must run the latest code against a migrated database
(they refuse to start if a migration is missing):

```bash
cd /home/karthikreddy/Documents/Catalift/server && make migrate
cd /home/karthikreddy/Documents/Catalift && GOTOOLCHAIN=go1.26.8 SECURE_COOKIES=false make dev
```

Hard-refresh the browser (Ctrl+Shift+R). Use two windows (one private), one
signed in as `seller@example.com`, one as `reviewer@example.com`.

## As the seller

The sidebar shows Upload, Products, Brand voice and Received files.

1. **Upload** `products.csv`: 10 rows load and 2 are not loaded.

   | Row | SKU | Why it is not loaded | What to do |
   | --- | --- | --- | --- |
   | 12 | ZR-5206 | category "jacket" is not one Catalift handles | Fix: Category kurta, add `fix-photos/ZR-5206_front.jpg`, Validate |
   | 13 | DV-4101 | SKU already exists (it came in with the Diwali sample) | Fix, then **Discard this row** |

   If you never uploaded the Diwali sample, DV-4101 loads instead (11 loaded,
   1 not loaded) and shows "Needs photo"; discard nothing and skip that line.
2. **Photos**: choose the `photos/` folder. 10 attach, none unmatched, and
   ZR-5205 is left without a photo.
3. **Products**: only this upload shows, 12 rows over 2 pages, its two refused
   rows on top. No Diwali, Harvest or older product appears.
   - Search `zr-52` to see only the Zari Rang rows.
   - **Fix** ZR-5205: add `fix-photos/ZR-5205_front.jpg`, then **Validate**.
   - Fix row 12 and discard row 13 as in the table above.
4. **Brand voice**: only Ghungroo and Zari Rang are listed. Set:

   | Brand | Tone | Words to avoid |
   | --- | --- | --- |
   | Ghungroo | Bright and festive; name the colour first. | cheap, best ever |
   | Zari Rang | Rich and traditional; mention the zari work. | discount, sale |

5. **Products**: **Generate listings**. Both brands have a tone, so there is no
   neutral-voice question. 11 products, so 22 listings, one per product for
   each of the two channels. Every row should end "Ready for review".
6. Typing `/review` or `/channels` in the address bar brings you back to Products.

## As the reviewer

The sidebar shows Products, Review, Export and Channels.

1. **Products**: the same 11 products, read-only.
2. **Review**: only the 22 listings of this upload. Open one, edit a field or
   regenerate it with an instruction, tick the passing rows and **Approve**.
3. **Channels** (optional): the counts are for this upload only. **Edit rules**
   on a channel, for example add `festive` to its banned words, and watch the
   affected listings fail and lose their approval in Review.
4. **Export**: the approved counts cover this upload only. **Export N approved
   listings**, then under **Send to the seller** click **Send to seller**. The
   export changes to "Sent to seller" with the time and your email. Its CSVs
   hold only GH- and ZR- products.

## As the seller again

1. **Received files**: only the export just sent, one CSV per channel. Download
   them and check every row is a GH- or ZR- SKU that the reviewer approved.
2. Exports sent for earlier uploads do not show here any more.

## Check the words-to-avoid rule

1. As the seller, in **Brand voice**, add a word you can see in a Ghungroo
   listing (`Ghungroo` itself always works with the free local stand-in, which
   writes "Made by Ghungroo.") and **Save voice**.
2. As the reviewer, **Review** shows those listings failing with
   `uses "Ghungroo", a word the brand avoids.`; any that were approved need
   approving again.
3. Remove the word and save: they pass again.

Each SKU loads only once. To run this again, change the prefixes in the CSV
(for example GH-5301, ZR-5401) and rename the photos to match.

| SKU | Category | Colour | Pattern | Photo in |
| --- | --- | --- | --- | --- |
| GH-5101 | kurta | red | dots | photos (front and back) |
| GH-5102 | lehenga | pink | floral | photos |
| GH-5103 | kurti | yellow | stripes | photos |
| GH-5104 | dupatta | green | dots | photos |
| GH-5105 | saree | blue | floral | photos |
| ZR-5201 | sherwani | maroon | solid | photos |
| ZR-5202 | kurta | cyan | floral | photos |
| ZR-5203 | kurti | violet | dots | photos |
| ZR-5204 | saree | rust | stripes | photos |
| ZR-5205 | lehenga | magenta | dots | fix-photos |
| ZR-5206 | kurta (after fix) | teal | solid | fix-photos |
