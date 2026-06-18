---
"yesoreyeram-pocketbase-datasource": minor
---

Add a Grafana data source plugin for PocketBase. TypeScript/React frontend with a
Go backend that talks to the PocketBase REST Records API. Supports records and
count query types over collections, type-aware structured filters (nested AND/OR
groups) compiled into a PocketBase filter expression, a raw filter escape hatch,
multi-field sort, field selection, page-based pagination, three authentication
modes (superuser, user, token) with token caching and transparent
re-authentication, and data-plane-compliant frames. The local docker-compose
stack runs a real self-hosted PocketBase seeded with sample collections.
