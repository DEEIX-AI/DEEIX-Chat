# Expiring credits

Expiring credits are optional balance grants with an independent expiration time.
They use the existing USD account balance and balance transaction ledger.
The feature is available in usage billing and for overage in period billing.

## Administrator grants

Use the user editor to enter a credit amount and its duration. Leaving the amount
blank skips the grant. Durations may be fractional, from 0.1 through 3650 days;
the server stores whole seconds.

The existing administrator authentication and billingGating feature gate protect:

`POST /api/v1/admin/billing/accounts/{user_id}/expiring-credits`

```json
{
  "creditUSD": 20,
  "durationDays": 0.1,
  "idempotencyKey": "campaign-user-42-001",
  "description": "Trial credit"
}
```

Reuse the same key when retrying a request whose outcome is unknown. The same
user, amount, duration, administrator and description return the original grant
without another deposit. Changing these fields while reusing the key returns a
conflict. The response includes the grant and the current account balance.

## Redemption codes

Usage-mode codes accept the optional `balanceDurationDays` field. Omission or zero
keeps the existing permanent-credit behavior. The lifetime starts at redemption;
the code's `expiresAt` separately controls when the code itself can be redeemed.
Subscription codes continue to use integer `durationDays`. Mixing subscription
duration and balance duration is rejected.

Responses and redemption snapshots expose `balanceDurationSeconds`. Existing
permanent codes remain permanent. Previous custom balance snapshots with a
fractional `duration_days` value remain readable.

## Accounting and expiration

- Account, grant batch and ledger changes commit in one database transaction.
  The lock order is account first, then credit batches.
  Capture one accounting timestamp after acquiring the account lock and reuse it
  for expiration and consumption, including when a request waits across expiry.
- Usage consumes active batches in expiration order, then permanent balance.
- A grant first repays existing debt. Only its unspent remainder can expire.
- Administrative balance reductions reduce active batches so expiration cannot
  deduct the same money again.
- Balance-sensitive reads, grants, top-ups, redemption, reservation and settlement
  expire due credits before calculating the available balance. A cancellable
  background sweep also runs on startup and every minute.
  Account lists check due users in one batch before reclaiming affected balances.
- Reservations remain budget checks; they do not extend a grant's lifetime.
  Usage settling after expiration is billed against the balance then available.
  Real costs can produce a negative balance, preserving existing settlement rules.

The shared schema migration adds `billing_expiring_credits` and
`billing_redemption_codes.balance_duration_seconds`. Existing account balances
are not converted into temporary grants. Existing custom grants retain their
amounts and expiration times; a missing grant-duration field defaults to zero.

Back up the database before deploying this schema change. To roll back the
application, restore the matching pre-upgrade database rather than running older
code against the migrated schema.

## Validation

SQLite regression coverage includes consumption order, redemption lifetime,
idempotent grants, conflicts, expiration and ledger rollback, concurrent sweeps,
debt repayment, delayed payment callbacks, reservation rechecks and settlement
after expiration. Account-list coverage also bounds expiration lookups as the
number of users grows, and subscription snapshots keep their duration separate
from balance lifetimes. PostgreSQL concurrency and lock-wait tests run when
`DEEIX_TEST_DATABASE_DSN` points to a dedicated test database.
