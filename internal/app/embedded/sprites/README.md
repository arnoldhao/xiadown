Place builtin sprites here.

Recommended layout:

- `internal/app/embedded/sprites/<sprite-id>/sprite.png`
- `internal/app/embedded/sprites/<sprite-id>/manifest.json`

Builtin sprites are ingested through the same pipeline as imported sprites:

- PNG / ZIP are supported
- the source sheet is validated as an `8×8` square grid with no aspect-ratio tolerance
- the source PNG must stay within `1024×1024` to `5120×5120`
- XiaDown stores and displays the source sheet as `sprite.png`
- exported ZIP archives always contain `sprite.png` and `manifest.json`

Row order is fixed from top to bottom:

1. `Greeting`
2. `Snoring`
3. `Upset`
4. `Celebrate`
5. `Seeking`
6. `Embarrassed`
7. `Working`
8. `ListeningToMusic`

## Generation Guide

For AI-generated builtin sprites, prefer generating 64 separate frames and
stitching them locally. Do not generate the final `8×8` sheet directly: image
models often draw visually correct grids that are not pixel-aligned, which makes
runtime cell slicing unreliable.

Recommended workflow:

1. Choose one reference image for identity only. For `gege`, the reference is
   `frontend/public/tray.png`.
2. Write the 64-frame plan before generating images. Each row must contain 8
   sequential, visibly different frames for one looped behavior.
3. Generate each frame as one isolated square image, not as a sheet or row.
   Use a flat removable chroma-key background such as `#ff00ff`, no shadow, no
   grid, no border, no text labels, and enough padding around the character.
4. Post-process each frame locally:
   - remove the chroma-key background to alpha
   - find the visible-pixel bounding box
   - scale frames with one consistent scale per row
   - center each frame inside a `512×512` transparent cell
5. Stitch the normalized frames row-major into a `4096×4096` sheet and save it
   as `sprite.png`.
6. Validate the final sheet before committing:
   - PNG is exactly square and within the accepted `1024×1024` to `5120×5120`
     range
   - sheet is divisible into an `8×8` grid with no remainder
   - all 64 cells contain visible pixels
   - each cell leaves a safe transparent margin, preferably at least `40px`
   - all frames keep the same character identity, outfit, body type, and style
   - adjacent frames show visible but continuous motion, with no duplicate,
     mirrored, or near-identical filler frames

Behavior definitions:

- `Greeting`: friendly greeting, light wave or nod, loopable.
- `Snoring`: sleeping or dozing; only closed-eye breathing and slight body
  rise/fall, no yawning.
- `Upset`: one consistent negative emotion such as annoyed, sad, shocked, or
  angry.
- `Celebrate`: excited celebration with clear, rhythmic motion.
- `Seeking`: confused searching, looking around, or trying to find something.
- `Embarrassed`: shy or awkward motion, restrained and cute.
- `Working`: one work mode for the whole row, such as writing, computer work,
  or crafting; do not mix modes in one row.
- `ListeningToMusic`: immersed listening, light swaying, nodding, and rhythm.
