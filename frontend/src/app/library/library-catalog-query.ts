import type { LibraryWorkspaceRoute } from "./types";

/** Maps plural UI route names onto the singular Catalog category contract. */
export function libraryCatalogCategory(route: LibraryWorkspaceRoute): string {
  if (route === "video" || route === "audio") return route;
  if (route === "books") return "book";
  if (route === "images") return "image";
  return "all";
}
