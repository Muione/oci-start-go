# Task 12 Report: Frontend Instances — Support instanceId query param auto-selection

## Status: DONE

## Commits
- `7d8f4c9` — feat(frontend): support instanceId query param for auto-selection in Instances

## Changes
- Added `useRoute` import from `vue-router`
- Added `const route = useRoute()` alongside the existing router
- Replaced `onMounted(load)` with an async handler that awaits `load()`, then reads `route.query.instanceId` and calls `showDetail` if a matching instance is found in `allInstances`

## Notes
- The task spec referenced an `instances` ref, but the actual codebase uses `allInstances` (full list) and `rows` (paginated view). Used `allInstances` for the lookup since it contains every loaded instance.

## Concerns
None.
