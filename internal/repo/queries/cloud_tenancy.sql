-- Cloud tenancy queries (Phase 9). cloud_tenancy stores account cost and
-- custom name per tenancy.

-- name: FindCloudTenancyByName :one
SELECT id, tenancy_name, cloud_type, type, def_name, account_cost, create_time, update_time
FROM cloud_tenancy WHERE tenancy_name = ?;

-- name: UpsertCloudTenancy :exec
INSERT INTO cloud_tenancy (tenancy_name, cloud_type, type, def_name, account_cost, create_time, update_time)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(tenancy_name) DO UPDATE SET
    def_name = excluded.def_name,
    account_cost = excluded.account_cost,
    update_time = excluded.update_time;

-- name: UpdateCloudTenancyCost :exec
UPDATE cloud_tenancy SET account_cost = ?, update_time = ? WHERE tenancy_name = ?;
