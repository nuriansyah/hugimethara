/** Derives a 1–40 char address from a name (matches the API's slug rules). */
export function slugify(name: string, max = 40): string {
  return name
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, max)
    .replace(/-+$/, "");
}

export const SHORT_SLUG = /^[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?$/;
export const SHORT_SLUG_MESSAGE = "1–40 lowercase letters, digits or hyphens; no leading or trailing hyphen.";
