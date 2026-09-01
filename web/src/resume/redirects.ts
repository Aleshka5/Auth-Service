/** Old URL → new path. Append a line whenever a node moves. */
export const REDIRECTS: Record<string, string> = {
  '/architecture': '/about-project/architecture',
  // '/architecture/:slug' is handled by LegacyArchitectureRedirect in App.tsx.
}

/** Takes a normalized path ('' for root) and returns the target path, or null. */
export function redirectFor(path: string): string | null {
  return REDIRECTS[`/${path}`] ?? null
}
