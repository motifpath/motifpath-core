# motifpath-core

Core domain logic and shared libraries. No framework dependencies — this package is consumed by motifpath-web and any other services.

## Key rules

- No HTTP, no DB drivers, no framework code in `src/domain/`; pure business logic only
- Services in `src/services/` may use adapters but must depend on interfaces, not concrete implementations
- Every public export must have a corresponding test
- Prefer explicit errors over thrown exceptions; use a Result type pattern

## Testing

- Unit tests live next to source: `src/foo.ts` → `src/foo.test.ts`
- Integration tests in `tests/integration/`
- `npm test` runs unit tests only; `npm run test:all` runs everything

## How to help

- Suggest domain model changes as separate commits from service changes
- Flag any dependency that would couple this package to a specific framework
- Keep the public API surface small; prefer internal over exported
