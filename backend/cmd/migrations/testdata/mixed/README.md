# Fixture for runner discovery tests.

Used by `cmd/migrations/main_test.go::TestDiscoverMigrations_DropsDownSuffix`
and friends. Files in this directory must NOT be discovered because they:

- lack the leading numeric prefix (`README.md`), or
- are in a subdirectory (`subdir/100_sub.sql`), or
- end in `.down.sql` (`006_drop.down.sql`).
