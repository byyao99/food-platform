// Money helpers. The API speaks integer cents; the UI shows/edits dollars.

/** Format integer cents as a currency string, e.g. 18000 -> "$180.00". */
export function formatCents(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`
}

/** Convert a dollar amount entered by the user to integer cents. */
export function toCents(dollars: number): number {
  return Math.round(dollars * 100)
}

/** Convert integer cents to a dollar number for editing in a form field. */
export function fromCents(cents: number): number {
  return cents / 100
}
