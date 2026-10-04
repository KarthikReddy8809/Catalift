// Display rules for the money Catalift shows. Prices are INR minor units
// (paise, data model products.price_minor); AI cost is millionths of a US
// dollar (data model deviation 3). Fixtures and views both go through these,
// so a figure on a screen is never typed by eye.

const inr = new Intl.NumberFormat("en-IN", { style: "currency", currency: "INR" });

/** formatInrMinor shows paise as rupees with Indian digit grouping. */
export function formatInrMinor(paise: number): string {
  return inr.format(paise / 100);
}

/**
 * formatUsdMicro shows millionths of a dollar. Under one dollar it keeps
 * four decimals so a single call's cost is still visible.
 */
export function formatUsdMicro(micro: number): string {
  const usd = micro / 1_000_000;
  const digits = usd !== 0 && Math.abs(usd) < 1 ? 4 : 2;
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(usd);
}
