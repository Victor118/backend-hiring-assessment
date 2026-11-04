# Delivery

This document sums up what was done for each task, the choices made and the alternatives left aside. The behaviour of the API itself is documented in the [README](README.md) and in the [OpenAPI spec](ads/internal/cmd/ads-api/specs/openapi.yaml).

Every commit follows Conventional Commits and leaves the test suite green. The work was done test first.

```shell
go test ./...
```

> The schema is created by the Postgres init scripts, which only run on an empty volume. To apply the new migrations (`003`, `004`) on an existing stack: `docker compose down -v && docker compose up -d`.

## 1. GetAdByID bug

The use case called the DAO with `0` instead of the requested ID, so `GET /ads/{id}` always answered `404`.

The tests did not catch it because the mock expected `mock.Anything` as the ID. They now expect the exact ID, which made them fail before the fix.

The commented-out legacy implementation was removed: it loaded every ad and filtered them in memory, a full table scan on a read endpoint under heavy load. Removing it loses nothing: it no longer compiled, and git history keeps it.

## 2. Delete an ad

`DELETE /ads/{id}` answers `204`, `400` when the ID is not a number, `401` without authentication, `403` when the caller does not own the ad, and `404` when it does not exist.

- **Only the owner can delete an ad.** The instructions do not ask for it, but without it any authenticated user could delete any ad. The rule lives in the domain (`Ad.AuthorizeDeletion`).
- The ad is loaded first, to tell a missing ad from a foreign one. The `DELETE` itself does not check the number of affected rows: if a concurrent request deleted the ad in between, the call still succeeds, which matches the idempotency of `DELETE`.

## 3. Owner username

`GET /ads/{id}` adds the `username` of the owner, fetched through the provided `users` client, behind a domain port.

The provided `UserClient` interface (`users/pkg`) does not match the concrete client (`users/pkg/api`): the ID type and the `User` type differ, so `api.Client` does not implement it. Since `users/` is read-only and the domain port already gives the abstraction, the adapter uses `api.Client` directly and is tested against an `httptest` server.

- **The username never makes the request fail.** It is secondary to the ad: when it cannot be resolved, the ad is still returned and the field is omitted. A failing `users` api is logged at `ERROR`, an unknown owner at `WARN`.
  This is a product assumption to confirm with the product owner and the web and mobile developers: the clients must handle a missing username. The alternatives would make a read endpoint under heavy load as available as the `users` api: failing the whole request, or retrying with a backoff, which would also add latency to every read and load on a failing `users` api. A circuit breaker would be the next step.
- **Usernames are cached in memory**, because this endpoint is under heavy load and would otherwise call the `users` api on every read. The cache is an LRU with a TTL (`hashicorp/golang-lru/v2/expirable`), wrapped around the adapter as a decorator. Only successful lookups are cached.
- **Trade-off:** a renamed user may keep their former username for up to the TTL (5 minutes by default), and each instance has its own cache. Both are acceptable for a display field.
- The call is bounded by a timeout (2 s by default). Size, TTL and timeout are configurable, and non-positive values fall back to the default, since `0` means "unbounded" or "no timeout" to the libraries.

## 4. `price_cents` renamed to a float `price`

I would not ship it as asked, and would come back to the stakeholders with an alternative.

**Why not a float.** A float cannot represent most decimal amounts exactly:

```go
a, b := 0.1, 0.2
fmt.Println(a + b)        // 0.30000000000000004
fmt.Println(a+b == 0.3)   // false
```

The same holds for every client (JavaScript, Swift, Kotlin) and for a `double precision` column in Postgres. Prices would drift through every computation: totals, the `min_price_cents` / `max_price_cents` filters, equality checks. Integer cents are exact, fast and unambiguous, which is why they are the common practice for money.

**Why not a rename.** Renaming a field of a user-facing API is a breaking change. Mobile apps already installed do not update with the API: every version in the stores would break on the day of the release.

**What I would do.**
1. Ask the product owner what the actual need is. Most likely, clients want a price they can display without dividing by 100.
2. Keep `price_cents` as the source of truth, in storage and in the request bodies.
3. Only if the clients really cannot format the cents themselves: add an output field next to `price_cents`, computed from the cents without going through a float, for instance a decimal string `"price": "120.00"`, or an object `{"amount": "120.00", "currency": "EUR"}` if several currencies are foreseeable. This is the choice to make with the web and mobile developers. Adding a field breaks no client.

If the stakeholders still want a JSON number, the minimum is the same transition: both fields side by side, and the number only ever computed from the cents, never stored or summed.

## 5. Bulk import

**Cause.** The handler looped over the creation of single ads, each one committed on its own. A failure midway left the first ads in the database, and retrying the import created them a second time.

**Fix, in two parts.**
- **Atomicity.** The import goes through its own use case, which creates the whole batch in a single transaction. One invalid ad rejects the whole batch, and an empty batch is rejected.
- **Idempotency.** Atomicity alone does not cover a request that succeeded but whose response was lost (a timeout on the client side): the retry would still create duplicates. `POST /ads/import` now requires an `Idempotency-Key` header, a UUID generated by the client for each import.
  Making the header required is a breaking change for existing clients, which section 4 argues against. I chose it because the duplicates are a bug reported by users, and an optional key would leave the bug in place. In a real team, I would roll it out with the web and mobile developers: during a transition period, requests without a key would still be accepted (atomic, but not idempotent).
  - The key is stored in `import_requests`, in the same transaction as the ads, with a SHA-256 fingerprint of the batch and the IDs of the created ads.
  - A retry with the same key and the same ads answers with the IDs of the first import, and creates nothing.
  - The same key with other ads is rejected with a `422` (Unprocessable Content: the request is well-formed, but conflicts with what the key already stands for), so that a reused key never silently drops an import.
  - The primary key `(user_id, idempotency_key)` scopes the keys to their owner and serializes concurrent requests with the same key: the second one waits for the first to commit, then replays it.
  - A new key with the same content creates the ads again: this is intended, the key is the client's statement that this is a new import.

## 6. Publication quota

A user may publish at most 5 ads through `POST /ads` over a rolling 24 hours, after which the API answers `429` (Too Many Requests).

**What is counted.** I read the quota as an anti-spam rule on the act of publishing, so it counts publications, not the ads currently online:
- the bulk import neither counts towards the quota nor is limited by it, as required;
- deleting an ad does not give a slot back, otherwise deleting and publishing again would get around the quota.

This is a product assumption to confirm with the product owner. The alternative, counting the ads of the user created over the last 24 hours, would only need a query on `ads`, but every import would then count and every deletion would free a slot.

**Design.**
- Each publication is recorded in a journal table, `ad_publications (user_id, published_at)`, with no foreign key to `ads`: it records an event, which outlives the ad.
- The rule lives in the domain, in a `Publisher` aggregate identified by the user, which holds its number of recent publications and authorizes the next one. The quota is not an invariant of an ad, but of the user publishing it.
- The quota and the window are domain constants. The use case passes the window to the repository, so that the whole rule reads in the use case.

**Concurrency.** Reading the count, then inserting, is a race: ten simultaneous requests would all read 4 and all succeed. The use case takes a transaction-level advisory lock per user (`pg_advisory_xact_lock`) right after opening the transaction, then loads the publisher, creates the ad and records the publication. The lock is released on commit or rollback, and only serializes the requests of the same user.

The lock is a separate port method (`LockPublisher`), called before `LoadPublisher`, rather than hidden in the loading. The intent stays visible in the use case, and the tests check the order of the calls.

Alternative considered: `SELECT … FOR UPDATE` on a row per user. The ads database has no users table, so there is no row to lock, and two first publications of a new user would both pass. It would become the natural option with a local publishers table (see "What I would do next"). The port would not change: only the adapter would.

The unit tests check that the use case takes the lock before counting; the actual guarantee against concurrent requests comes from the end-to-end check below, which an automated integration test should replace.

**Verified end to end:** 10 concurrent `POST /ads` of the same user give 5 × `201` (Created) and 5 × `429` (Too Many Requests); a deletion does not free a slot; an import still works once the quota is reached; another user is not affected.

## Other improvements

- **Domain layer.** I noticed the issue while writing the tests for the delete task: `ErrNotFound`, a business fact ("this ad does not exist"), was defined in the DAO and went all the way up to the HTTP handler. The structure looked hexagonal, but the use cases depended on the persistence package. Business errors now live in the domain, and the use cases depend on ports declared in the domain, which the DAO implements. This is the hexagonal dependency rule, and also the Go idiom: an interface belongs to the package that uses it, not to the one that implements it. The creation of an ad goes through `AdDraft`, which validates it: a title that is not empty once trimmed, a price that is not negative. Invalid ads answer `400` instead of `500`.

  I kept the existing package names (`dao`, `http`) to keep the diff readable. On a new project, I would group them under `adapter/primary` (http) and `adapter/secondary` (dao, identity).
- **Tests.** An inverted assertion made the suite red on the initial commit, it was fixed first. The mock expectations now check the exact arguments.
- **OpenAPI spec** brought up to date: delete, import, `username`, error responses, validation rules.
- **Devcontainer** with Go 1.26 and Postgres, and the `users` api wired into the Compose stack.

## What I would do next

**Testing**
- Integration tests against a real Postgres (testcontainers): the SQL is only tested through `sqlmock`, and the concurrency of the quota was only checked by hand.

**API**
- Error bodies: errors currently only have a status code. A JSON body saying what went wrong (which field is invalid, why the import was rejected) would let the clients show a precise message.
- A `Retry-After` header on the `429` (Too Many Requests). It would need the date of the oldest publication of the window, not only the count.
- A maximum batch size for the import, with a dedicated error.
- The `Authorization` header is not validated as a UUID: an invalid value ends in a `500` at the database. `handlerCreateAd` still uses the deprecated `middleware.UserIDFromHeader`.

**Search (`GET /ads`)**
- No pagination: without filters, `GET /ads` returns the whole table, on a read endpoint under heavy load. I would add cursor (keyset) pagination rather than `OFFSET`, which gets slower as the page number grows.
- `title ILIKE '%…%'` cannot use a B-tree index, because of the leading wildcard: every title search scans the whole table. A trigram index (`pg_trgm`) would help; at scale, this belongs in a search engine.
- `user_id::text = $1` casts the column, so an index on `user_id` could not be used (there is none today). Casting the parameter instead (`user_id = $1::uuid`) and adding the index would fix it.

**Domain model**
- `Ad` exposes its fields because the DAO scans directly into it: the persistence shapes the domain. The domain could keep its fields private and expose a snapshot (or a restore function) that the adapters read and write.

**Architecture**
- A local `publishers` table in the ads service, kept in sync with events from the users service (user created, nickname changed). It would remove the synchronous call to the users api on every read, and give one row per user to lock with `SELECT … FOR UPDATE` for the quota. The trade-off is eventual consistency: a renamed user shows the old nickname until the event arrives.
- Cache failed username lookups for a few seconds, to stop hitting a failing users api on every read, before going to a full circuit breaker.

**Data**
- `CHECK` constraints in the database, mirroring the domain rules (`price_cents >= 0`, non-empty title).
- Multi-row inserts for the import, instead of one insert per ad.
- A purge job for expired import keys and old publications (the index on `import_requests.created_at` is already there).
