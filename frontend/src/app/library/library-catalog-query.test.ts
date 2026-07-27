import { describe, expect, test } from "bun:test";

import { libraryCatalogCategory } from "./library-catalog-query";

describe("Library Catalog query projection", () => {
  test("maps plural workspace routes onto singular Catalog category values", () => {
    expect(libraryCatalogCategory("video")).toBe("video");
    expect(libraryCatalogCategory("audio")).toBe("audio");
    expect(libraryCatalogCategory("books")).toBe("book");
    expect(libraryCatalogCategory("images")).toBe("image");
    expect(libraryCatalogCategory("all")).toBe("all");
    expect(libraryCatalogCategory("ended")).toBe("all");
  });
});
