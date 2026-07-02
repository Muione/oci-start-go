-- Clear stale register_time values that were set to the tenant creation time
-- (or sync time) instead of the real OCI subscription timeStart. With the
-- code fix, register_time is only set from a real subscription timeStart on
-- successful OCI sync; null here makes GetSubscriptionDetail fall back to a
-- live OCI call (correct subscription start) until the next sync.
UPDATE register_detail SET register_time = NULL;
