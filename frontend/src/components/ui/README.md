# UI Base Components

This directory keeps the low-level Radix/shadcn-compatible APIs used by
`frontend/src/shared/ui`. Product code imports the shared wrappers, never these
base modules directly.

The base components publish stable `app-base-*` classes and state attributes.
Their anatomy and fallback appearance live in Dream CSS; do not reintroduce
Tailwind appearance recipes here. App-specific behavior, variants, and material
roles belong in `frontend/src/shared/ui`.
