// Package testdb gives each integration test its own throwaway database on
// the local Postgres (make db), migrated from db/migrations, and drops it
// afterwards. Services open their own transactions, so a per-test database
// keeps tests isolated where a rolled-back transaction cannot. The helpers
// build only with -tags=integration.
package testdb
