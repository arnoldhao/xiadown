import { describe, expect, test } from "bun:test";

import { resolveListenNativeVideoPrimaryHole } from "@/app/main/listen/native-video-underlay";

describe("native video Primary hole geometry", () => {
  test("converts viewport geometry into Primary-local coordinates", () => {
    expect(
      resolveListenNativeVideoPrimaryHole(
        { left: 332, top: 96, width: 640, height: 360, radius: 18 },
        { left: 280, top: 44, width: 1040, height: 720 },
      ),
    ).toEqual({
      left: 52,
      top: 52,
      width: 640,
      height: 360,
      radius: 18,
    });
  });

  test("clips the local hole and its radius to the Primary bounds", () => {
    expect(
      resolveListenNativeVideoPrimaryHole(
        { left: 80, top: 20, width: 260, height: 180, radius: 100 },
        { left: 100, top: 40, width: 200, height: 120 },
      ),
    ).toEqual({
      left: 0,
      top: 0,
      width: 200,
      height: 120,
      radius: 60,
    });
  });
});
