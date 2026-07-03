# Task A — Notification Recipient Per-Row Delete

## Status
Done. Build green, committed.

## Commits
- `f8ea469` feat(frontend): add per-row delete for notification recipients
  - `frontend/src/views/TenantDetail.vue` (+18)

## Changes
1. **`deleteRecipient(email)`** added immediately after `updateNotifRecipients`. Confirms via `ElMessageBox.confirm`, computes `remaining` by filtering out the target email, `POST /tenants/:id/notification-recipients/update` with the remaining list, reloads via `loadNotifRecipients()`. `cancel` from the confirm dialog is swallowed; other errors surface via `ElMessage.error`. Shares the existing `notifSaving` flag so the "更新接收人" button also disables during a delete.
2. **"操作" column** added to the recipients `<el-table>` (width 80, centered): a `danger` text button with the `Delete` icon, calling `deleteRecipient(row.email)`.

## Verification
- `npm run build` succeeded (built in 11.75s, no errors). Pre-existing chunk-size warning only.
- `Delete` icon and `ElMessageBox` were already imported — no import changes needed.

## Concerns / Notes
- Delete is implemented as "compute remaining list → full update", reusing the existing `update` endpoint (no new backend route). Same semantics as the bulk "更新接收人" button, just scoped to removal. If the OCI/notification service treats `update` as replace-all (which it already does for the bulk button), this is consistent; if it ever becomes additive, the delete path would need a dedicated `DELETE` route. No change in behavior vs. the existing flow.
- No confirmation on empty `remaining` (deleting the last recipient) — the call sends `{emails: []}`, which clears the list. Matches the behavior of clearing the input and clicking "更新接收人". If the backend rejects an empty list, this would surface as `ElMessage.error`; acceptable and matches existing bulk semantics.
- Did not add a test: this is a thin UI handler delegating to an existing request path; per project test discipline the repo layer / OCI layer is where coverage is owed, not this Vue handler.
