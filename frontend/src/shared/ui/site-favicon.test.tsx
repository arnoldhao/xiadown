import { describe, expect, test } from "bun:test";
import { Radar } from "lucide-react";
import { renderToStaticMarkup } from "react-dom/server";

import { SiteFavicon } from "./site-favicon";

describe("SiteFavicon", () => {
  test("renders a trimmed resolved favicon as a decorative image", () => {
    const markup = renderToStaticMarkup(
      <SiteFavicon source="  data:image/png;base64,icon  " />,
    );

    expect(markup).toContain('alt=""');
    expect(markup).toContain('src="data:image/png;base64,icon"');
  });

  test("renders the caller fallback when no favicon is available", () => {
    const markup = renderToStaticMarkup(
      <SiteFavicon source="  " fallback={<Radar data-testid="fallback" />} />,
    );

    expect(markup).toContain('data-testid="fallback"');
    expect(markup).not.toContain("<img");
  });

  test("uses the neutral site fallback by default", () => {
    const markup = renderToStaticMarkup(<SiteFavicon source={null} />);

    expect(markup).toContain("<svg");
    expect(markup).toContain('aria-hidden="true"');
  });
});
