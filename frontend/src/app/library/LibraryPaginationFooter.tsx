import { ChevronLeft, ChevronRight } from "lucide-react";

import { Button } from "@/shared/ui/button";
import { Select } from "@/shared/ui/select";

import {
  LIBRARY_PAGE_SIZE_OPTIONS,
  libraryPageCount,
  libraryPageRange,
  libraryPageTokens,
} from "./library-pagination";

export interface LibraryPaginationLabels {
  itemCount: (count: number) => string;
  pageRange: (start: number, end: number, total: number) => string;
  perPage: (pageSize: number) => string;
  perPageUnit: string;
  pageOf: (page: number, pageCount: number) => string;
  previousPage: string;
  nextPage: string;
}

export interface LibraryPaginationFooterProps {
  page: number;
  pageSize: number;
  total: number;
  labels: LibraryPaginationLabels;
  disabled?: boolean;
  pageSizeOptions?: readonly number[];
  /** Lets WorkspacePageFooter own the single semantic footer landmark. */
  embedded?: boolean;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
}

export function LibraryPaginationFooter(props: LibraryPaginationFooterProps) {
  const count = libraryPageCount(props.total, props.pageSize);
  const page = Math.min(count, Math.max(1, props.page));
  const range = libraryPageRange(page, props.pageSize, props.total);
  const tokens = libraryPageTokens(page, count);
  const pageSizeOptions = props.pageSizeOptions ?? LIBRARY_PAGE_SIZE_OPTIONS;

  const contents = (
    <>
      <span className="app-library-pagination__range">
        {props.labels.pageRange(range.start, range.end, props.total)}
      </span>
      <div className="app-library-pagination__controls">
        <span className="app-library-pagination__size-control">
          <Select
            value={String(props.pageSize)}
            disabled={props.disabled}
            aria-label={props.labels.perPage(props.pageSize)}
            className="app-library-pagination__size"
            onChange={(event) => props.onPageSizeChange(Number(event.currentTarget.value))}
          >
            {pageSizeOptions.map((value) => (
              <option key={value} value={value}>{value}</option>
            ))}
          </Select>
          <span aria-hidden="true">{props.labels.perPageUnit}</span>
        </span>
        <span className="app-library-pagination__page-label">
          {props.labels.pageOf(page, count)}
        </span>
        <nav className="app-library-pagination__pages" aria-label={props.labels.pageOf(page, count)}>
          <Button
            type="button"
            variant="ghost"
            size="compactIcon"
            aria-label={props.labels.previousPage}
            title={props.labels.previousPage}
            disabled={props.disabled || page <= 1}
            onClick={() => props.onPageChange(page - 1)}
          >
            <ChevronLeft className="h-4 w-4" aria-hidden="true" />
          </Button>
          <span className="app-library-pagination__number-list">
            {tokens.map((token, index) => token === "ellipsis" ? (
              <span key={`ellipsis-${index}`} className="app-library-pagination__ellipsis" aria-hidden="true">…</span>
            ) : (
              <Button
                key={token}
                type="button"
                variant={token === page ? "secondary" : "ghost"}
                size="compactIcon"
                aria-label={props.labels.pageOf(token, count)}
                aria-current={token === page ? "page" : undefined}
                disabled={props.disabled}
                onClick={() => props.onPageChange(token)}
              >
                {token}
              </Button>
            ))}
          </span>
          <Button
            type="button"
            variant="ghost"
            size="compactIcon"
            aria-label={props.labels.nextPage}
            title={props.labels.nextPage}
            disabled={props.disabled || page >= count}
            onClick={() => props.onPageChange(page + 1)}
          >
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          </Button>
        </nav>
      </div>
    </>
  );

  if (props.embedded) return contents;

  return (
    <footer
      className="app-library-page__footer app-library-pagination"
      aria-label={props.labels.itemCount(props.total)}
      aria-live="polite"
    >
      {contents}
    </footer>
  );
}
