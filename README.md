# Way of working

This repository is read-only, click on "Use this template" => "Create a new repository" and work on your copy just as you would on a live one. \
Once you are satisfied with your work, please send us your repository url.

# Hiring instructions

Please refer to [these instructions](INSTRUCTIONS.md).

See [DELIVERY.md](DELIVERY.md) for what was done for each task, and why.

# Ads API

A small Go HTTP API around a single domain: *ads* (classified ads).

The API needs an up-and-running postgres database, which can be launched via the provided Docker Compose file.

## Requirements

- Go 1.26 or later
- Docker with the Compose plugin

## Launching the stack

```bash
docker compose up -d
```

This starts PostgreSQL with the schema and seed data already applied, the `ads` api (port 8080), and the `users` api (port 8081), which the `ads` api calls to resolve the username of an ad's owner.

## Model

There are 2 entities in this sample:
- ad (title, price, etc...)
- user (id and nickname only)

## API

| Method | Path          | Auth | Description                                                                       |
|--------|---------------|------|-----------------------------------------------------------------------------------|
| `POST` | `/ads`        | yes | Create an ad                                                                      |
| `GET` | `/ads/{id}`   | no | Get a single ad, with its owner's `username`                                      |
| `GET` | `/ads`        | no | List ads (filterable by `title`, `user_id`, `min_price_cents`, `max_price_cents`) |
| `POST` | `/ads/import` | yes | Bulk ad import                                                                    |
| `DELETE` | `/ads/{id}` | yes | Delete an ad (owner only)                                                         |

```bash
# Health check
curl -s -v http://localhost:8080/health

# List ads (with optional filters)
curl -s 'http://localhost:8080/ads?title=bike&min_price_cents=1000'

# Get a single ad
curl -s http://localhost:8080/ads/1

# Create an ad (requires Authorization header)
curl -s -X POST http://localhost:8080/ads \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Mountain bike","price_cents":12000,"photo_url":"https://example.com/photos/bike.jpg"}'

# Import ads in bulk (requires Authorization and Idempotency-Key headers)
curl -s -X POST http://localhost:8080/ads/import \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001' \
  -H 'Idempotency-Key: 3f6c1a52-8e0b-4d6e-9a43-2b7f5c9d1e80' \
  -H 'Content-Type: application/json' \
  -d '{"ads":[{"title":"Mountain bike","price_cents":12000,"photo_url":"https://example.com/photos/bike.jpg"},{"title":"Road bike","price_cents":30000,"photo_url":"https://example.com/photos/road.jpg"}]}'

# Delete an ad (requires Authorization header, only the owner can delete it)
curl -s -X DELETE http://localhost:8080/ads/3 \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001'
```

### Owner username

`GET /ads/{id}` adds the `username` of the ad's owner, fetched from the `users` api:

```json
{"id":1,"title":"Leather sofa #1","price_cents":25000,"photo_url":"https://example.com/photos/ad-1.jpg","user_id":"00000000-0000-0000-0000-000000000002","created_at":"2024-06-01T02:00:00Z","username":"bob"}
```

The username is secondary to the ad, so it never makes the request fail. When it cannot be resolved, the ad is still returned with a `200` and the `username` field is **omitted**:
- the `users` api is down, slow or failing: logged at `ERROR`;
- the owner does not exist in the `users` api (an orphan ad): logged at `WARN`.

Usernames are cached in memory, in each instance of the `ads` api, so that this heavily read endpoint does not call the `users` api on every request. Only successful lookups are cached: a failure is retried on the next request. A renamed user may keep their former username for up to `IDENTITY_CACHE_TTL`.

The call to the `users` api is configured with:

| Variable              | Default                 | Description                                         |
|-----------------------|-------------------------|-----------------------------------------------------|
| `IDENTITY_URL`        | `http://localhost:8081` | Base URL of the `users` api                         |
| `IDENTITY_TIMEOUT`    | `2s`                    | Maximum duration of a call to the `users` api       |
| `IDENTITY_CACHE_SIZE` | `10000`                 | Maximum number of usernames kept in memory          |
| `IDENTITY_CACHE_TTL`  | `5m`                    | How long a username is kept before being refreshed  |

Invalid or non-positive values fall back to the default.

### Ad validation

`POST /ads` and `POST /ads/import` reject an invalid ad with a `400`:
- the title must not be empty (it is trimmed of surrounding spaces);
- the price must not be negative (`0` is a free ad).

### Publication quota

A user may publish at most 5 ads through `POST /ads` over a rolling 24 hours. Beyond that, `POST /ads` answers `429 Too Many Requests` until the oldest of these publications is more than 24 hours old.

The quota counts publications, not the ads currently online:
- ads created by the bulk import do not count, and the import is never limited by the quota;
- deleting an ad does not free a slot, so that deleting and publishing again cannot get around the quota.

Concurrent requests of the same user are serialized, so that they cannot exceed the quota together. The other users are not blocked.

### Bulk import

`POST /ads/import` creates every ad of the batch or none of them: they are inserted in a single transaction, and one invalid ad rejects the whole batch. An empty batch is rejected with a `400`.

It answers with the IDs of the created ads, in the order of the batch:

```json
{"ids":[107,108]}
```

The `Idempotency-Key` header is required, and must be a UUID generated by the client for each import. It makes the import safe to retry, after a timeout for instance:

| Request                             | Response                                                    |
|-------------------------------------|-------------------------------------------------------------|
| a new key                           | `201`, the ads are created                                  |
| the same key and the same ads       | `201` with the IDs of the first import, nothing is created  |
| the same key and other ads          | `422`, the key was reused for another import                |
| no key, or a key that is not a UUID | `400`                                                       |

Keys are scoped to their owner, and concurrent requests with the same key create the ads only once.

## Authentication

To pass authentication information to a call, simply add the `Authorization` header with the user ID in plain text.

It will be extracted by the service.

```
Authorization: 00000000-0000-0000-0000-000000000001
```

## Tests

To launch all the tests:

```shell
go test ./...
```
