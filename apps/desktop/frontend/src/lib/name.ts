export function sanitizeName(raw: string): string {
  return raw.trim().replace(/\s+/g, " ");
}
