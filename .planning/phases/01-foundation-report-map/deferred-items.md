# Deferred Items — Phase 01

Out-of-scope discoveries logged during execution, per the executor's scope-boundary rule
(only auto-fix issues directly caused by the current task's changes).

## From plan 01-03

- **`internal/testutil/seed.go`'s `seedCategories` uses `storm_cyclone_damage`, not
  `storm_cyclone`.** Plan 01-01 (pre-service-layer) picked this string before
  `service.Categories` existed; the canonical slug locked by 01-03 is `storm_cyclone`. No
  Task 3 test decodes a seeded row through `service.Category`, so this mismatch doesn't
  currently break anything — `TestNearbyReportsUsesIndex` only counts rows and inspects the
  query plan, it never asserts on category values. Left unfixed per the scope-boundary rule
  (seed.go is plan 01-01's file, not touched by any 01-03 task). A future plan touching
  `testutil/seed.go` should align this string to `service.CategoryStormCyclone`.
