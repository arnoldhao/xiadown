import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { LibraryPaginationFooter } from "./LibraryPaginationFooter";

const labels = {
  itemCount: (count: number) => `${count} items`,
  pageRange: (start: number, end: number, total: number) => `${start}-${end} of ${total}`,
  perPage: (pageSize: number) => `${pageSize} per page`,
  perPageUnit: "items/page",
  pageOf: (page: number, pageCount: number) => `Page ${page} of ${pageCount}`,
  previousPage: "Previous page",
  nextPage: "Next page",
};

describe("LibraryPaginationFooter", () => {
  test("renders range, page size, direct page numbers and navigation state", () => {
    const markup = renderToStaticMarkup(
      <LibraryPaginationFooter
        page={3}
        pageSize={24}
        total={240}
        labels={labels}
        onPageChange={() => {}}
        onPageSizeChange={() => {}}
      />,
    );

    expect(markup).toContain("49-72 of 240");
    expect(markup).toContain('<option value="24" selected="">24</option>');
    expect(markup).toContain("items/page");
    expect(markup).toContain('aria-label="24 per page"');
    expect(markup).toContain("Page 3 of 10");
    expect(markup).toContain('aria-current="page"');
    expect(markup).toContain("…");
    expect(markup).toContain('aria-label="Previous page"');
    expect(markup).toContain('aria-label="Next page"');
  });

  test("uses a stable zero range and disables both directions for an empty result", () => {
    const markup = renderToStaticMarkup(
      <LibraryPaginationFooter
        page={8}
        pageSize={48}
        total={0}
        labels={labels}
        onPageChange={() => {}}
        onPageSizeChange={() => {}}
      />,
    );

    expect(markup).toContain("0-0 of 0");
    expect(markup).toContain("Page 1 of 1");
    expect(markup.match(/disabled=""/g)?.length).toBe(2);
  });
});
