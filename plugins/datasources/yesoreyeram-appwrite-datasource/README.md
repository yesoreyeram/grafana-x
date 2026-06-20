# Appwrite data source for Grafana

A Grafana **data source plugin with a Go backend** for
[Appwrite](https://appwrite.io). Query your Appwrite **Databases** —
collections and documents — directly from Grafana, with a type-aware filter
builder, sorting, attribute selection and counts.

> Plugin id: `yesoreyeram-appwrite-datasource`

## Features

- **Query types**
  - **Documents** — list documents (rows) from a collection.
  - **Count** — the number of documents matching the filters.
- **Pickers** for databases, collections and attributes, populated live from the
  Appwrite API.
- **Type-aware filters** — a structured filter builder with nested AND/OR
  groups. Operators adapt to each attribute's type (string/number/boolean/
  datetime) and are compiled server-side into
  [Appwrite query strings](https://appwrite.io/docs/products/databases/queries).
- **Raw queries** — an advanced escape hatch to paste Appwrite query strings
  directly.
- **Sorting** by one or more attributes, **attribute selection** (`select`), and
  a **limit** with transparent cursor-based pagination.
- **Data-plane-compliant frames** with automatic type inference and ISO 8601
  time parsing (great for time series and tables).

## Data model mapping

| Appwrite        | This plugin            |
| --------------- | ---------------------- |
| Database        | Database picker        |
| Collection      | Collection picker      |
| Document        | Row in the data frame  |
| Attribute       | Column / filter field  |
| Query           | Filters / sort / limit |

The Appwrite system attributes `$id`, `$createdAt` and `$updatedAt` are promoted
to the front of every result frame.

## Configuration

Create an **API key** in the Appwrite console
(Overview → Integrations → API keys) with the scopes:

- `databases.read`
- `collections.read`
- `attributes.read`
- `documents.read`

Then configure the data source:

| Field                | Description                                                                                                  |
| -------------------- | ------------------------------------------------------------------------------------------------------------ |
| **Endpoint**         | Appwrite API endpoint including `/v1`. Cloud: `https://cloud.appwrite.io/v1` (or a regional/self-hosted URL). |
| **Project ID**       | Your Appwrite project id (sent as `X-Appwrite-Project`).                                                      |
| **API Key**          | Your Appwrite API key (sent as `X-Appwrite-Key`; stored as a secret).                                         |
| **Default Database** | Optional. When set, the query editor lists this database's collections directly.                             |

## Local development

This plugin is a workspace in the `grafana-x` Yarn 4 monorepo.

```bash
# from the monorepo root
yarn install

# build frontend + backend
yarn workspace @yesoreyeram/grafana-appwrite-datasource build

# run a local Grafana with the plugin and datasource provisioned
APPWRITE_PROJECT_ID=... APPWRITE_API_KEY=... APPWRITE_DATABASE_ID=... \
  docker compose -f plugins/datasources/yesoreyeram-appwrite-datasource/docker-compose.yaml up
```

Grafana runs with anonymous admin at <http://localhost:3000>.

## License

Apache-2.0. See [LICENSE](./LICENSE).
