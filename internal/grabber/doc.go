// Package grabber is the 抢机 (instance-grabbing) engine (SPEC S8, S16).
// It implements the dual-pool scheduling core: a parent scheduler pool runs
// lightweight dedup+dispatch logic, and a child OCI API pool executes the
// long-running LaunchInstance calls with 80s non-blocking timeout protection.
// Single-flight deduplication on (tenancy+region+architecture) keys ensures
// only one launch attempt per target per round. Idempotent launch is achieved
// via the DB open_boot_lock table + OCI opcRetryToken.
//
// The engine is triggered by the scheduler package via CheckAndExecuteTasksOnce
// every 6 seconds (parity with CreateInstanceJob). Success and failure are
// handled asynchronously: successes trigger the notification + delayed backup
// chain (onGrabSuccess), failures increment error counters and alert via Telegram
// (onGrabFailure). All exit paths release the single-flight key.
package grabber
