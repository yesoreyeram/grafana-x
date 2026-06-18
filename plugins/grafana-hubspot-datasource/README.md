# Grafana HubSpot Data Source

A Grafana data source plugin for [HubSpot](https://developers.hubspot.com). Query contacts, companies, deals, tickets, products, line items, meetings, calls, tasks, notes, emails, pipelines, owners, and properties from HubSpot CRM directly in Grafana.

## Features

- **All CRM objects** — contacts, companies, deals, tickets, products, line items
- **Engagement types** — meetings, calls, tasks, notes, emails
- **Utility queries** — pipelines, owners, properties
- **Dynamic property filter builder** with all CRM Search API operators (EQ, NEQ, CONTAINS_ALL, CONTAINS_ANY, GT, GTE, LT, LTE, BETWEEN, IN, NOT_IN, HAS_PROPERTY, NOT_HAS_PROPERTY)
- **Dynamic property dropdowns** — property definitions fetched live from HubSpot
- **Pipeline and stage filtering** for deals and tickets
- **Date range filters** — dashboard time range or custom before/after
- **Property selection** — choose which columns to return
- **Sort** by any property (ascending/descending)
- **Auto-pagination** — fetches all matching records (up to 10,000 per query)
- **Raw REST mode** — custom GET or POST requests to any HubSpot API endpoint
- **US and EU data residency** — configure `api.hubapi.com` or `api.hubapi.eu`
- **Authentication** — private app tokens or OAuth2

## Authentication

Supports two authentication methods:

| Method | Description |
|---|---|
| **Private App Token** | Create a private app in HubSpot under **Settings → Integrations → Private Apps**. Copy the access token and paste it in the datasource config. |
| **OAuth2** | Use a HubSpot OAuth2 app. The token (access + refresh) is stored as a secure credential. |

Both methods send the token as `Authorization: Bearer <token>` to HubSpot's API.

## Required OAuth Scopes

Your HubSpot app (private or OAuth) must be granted the **read** scopes for every object type you plan to query. If a scope is missing, HubSpot returns HTTP 403 with `MISSING_SCOPES`.

| Query Type | Required HubSpot Scope |
|---|---|
| Contacts | `crm.objects.contacts.read` |
| Companies | `crm.objects.companies.read` |
| Deals | `crm.objects.deals.read` |
| Tickets | `crm.objects.tickets.read` |
| Products | `crm.objects.products.read` |
| Line Items | `crm.objects.line_items.read` |
| Meetings, Calls, Tasks, Notes, Emails | `crm.objects.engagements.read` |
| Pipelines | `crm.objects.deals.read` or `crm.objects.tickets.read` (depending on object) |
| Owners | `crm.objects.owners.read` |
| Properties | `crm.schemas.custom.read` |

For a private app that uses every query type, grant **all** of the following scopes:

```
crm.objects.contacts.read
crm.objects.companies.read
crm.objects.deals.read
crm.objects.tickets.read
crm.objects.products.read
crm.objects.line_items.read
crm.objects.engagements.read
crm.schemas.custom.read
crm.objects.owners.read
```

> **Tip:** You can narrow scopes to only the objects you query. The health check uses `/crm/v3/owners`, so `crm.objects.owners.read` is needed for the connection test to pass.

## Quick Start

```bash
# Install dependencies (from monorepo root)
yarn install

# Build the plugin
yarn build

# Run with a local Grafana instance
HUBSPOT_ACCESS_TOKEN=pat-xxxxxx docker compose up
```

Grafana runs at http://localhost:3000 with anonymous admin access.

## Configuration

| Field | Description |
|---|---|
| **API URL** | HubSpot API base URL. Default: `https://api.hubapi.com`. Use `https://api.hubapi.eu` for EU data residency. |
| **Auth Method** | `Private App Token` or `OAuth2`. |
| **Access Token** | The private app token or OAuth access token. Stored securely and never exposed to the browser. |

## Query Editor

| Section | Description |
|---|---|
| **Query Type** | Select the CRM object or utility to query. |
| **Filters** | Build filter groups with property, operator, and value. Multiple filters in one group are AND'd; groups are OR'd. |
| **Sort By** | Choose a property and direction to order results. |
| **Properties** | Select which properties to include in results (default: all). |
| **Pipeline / Stage** | (Deals & Tickets only) Filter by pipeline and/or stage. |
| **Date Filters** | Filter by `createdate` and/or `hs_lastmodifieddate` using dashboard time range or custom dates. |
| **Limit** | Maximum records to return (default: 200, max: 10,000). |
| **Raw Mode** | Execute custom GET or POST requests with a path and optional JSON body. |

## API Limits

HubSpot rate-limits private apps and OAuth apps to **100 requests per 10 seconds** per app. The plugin relies on the Grafana SDK's HTTP client for retry/backoff behavior.

## Local Development

```bash
# Type-check
yarn typecheck

# Lint
yarn lint

# Lint with auto-fix
yarn lint:fix

# Run backend tests
go test ./pkg/...

# Run frontend tests
yarn test

# Rebuild golden test snapshots
UPDATE_GOLDEN=true go test ./pkg/plugin/ -run TestGolden_Frames
```

## Troubleshooting

| Symptom | Likely Cause |
|---|---|
| 403 MISSING_SCOPES | The private app/OAuth app lacks required scopes for the endpoint. See [Required OAuth Scopes](#required-oauth-scopes). |
| 401 Unauthorized | Token is invalid or expired. Regenerate in HubSpot. |
| Empty results | Filter criteria match no records, or properties field is empty. |
| Timeout | Large result sets with many properties. Reduce limit or select fewer properties. |
