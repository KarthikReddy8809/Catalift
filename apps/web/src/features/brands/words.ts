/** parseWords reads words to avoid typed one per line, or separated by commas. */
export function parseWords(text: string): string[] {
  return text
    .split(/[\n,]/)
    .map((w) => w.trim())
    .filter(Boolean);
}
