# Task 6-11 Report

## Status: DONE

## Commits
- `e3bb3bf` fix(frontend): fix TenantDetail loaded flags, error display, subscription days, instance nav, costTotal, quota filter

## Changes Summary

### Task 6: Loaded flags to prevent empty-list re-fetch
- Added 6 `*Loaded` refs (instLoaded, userLoaded, socialLoaded, costLoaded, domainsLoaded, settingsLoaded)
- Replaced `onTabChange` to use loaded flags instead of empty-list checks
- Set loaded flags in `finally` blocks of loadInstances, loadUsers, loadSocial, loadDomains, loadCost

### Task 7: Surface OCI API errors with retry
- Added 4 error refs (mfaError, notifError, auditError, quotaError)
- Updated loadMfaStatus, loadNotifRecipients, loadAudit, loadAuditCustom, loadQuota to capture error messages
- Added 4 `el-alert` components in settings tab template with retry buttons (MFA, notifications, quota, audit log)

### Task 8: Fix subscription days calculation
- Replaced loadSubscriptionDays with cascading fallback: subscription timeStart -> OCI API -> tenant createdAt -> em-dash
- onTabChange for costs tab chains loadSubscription().then(() => loadSubscriptionDays()) so subscriptionData is populated first

### Task 9: Fix instance click navigation
- Changed router.push from `/instances/${row.id}` (DB PK path param) to `{ path: '/instances', query: { instanceId: row.instanceId } }` (OCI OCID query param)

### Task 10: Convert costTotal to computed
- Added `computed` to Vue import
- Changed `const costTotal = ref(0)` to `computed(() => costData.value.reduce(...))`
- Removed manual `costTotal.value = 0` and `costTotal.value = ...` assignments from loadCost

### Task 11: Filter quota services to those with data
- Replaced loadQuotaServices to probe each service with pageSize=1 via Promise.allSettled, keeping only services with actual quota items
- Added auto-select of first available service if current selection is gone
- Added `el-empty` placeholder when no quota services have data

## Concerns
None. All changes applied cleanly to a single file with no conflicts.
