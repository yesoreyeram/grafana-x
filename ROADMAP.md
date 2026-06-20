<!-- markdownlint-configure-file {
  "MD013": false,
  "MD033": false
} -->

# Roadmap

Candidate Grafana **data source** plugins to add next, extending the SaaS / no-code
API themes already in this repo (no-code databases, project management, docs, CRM).

Every service listed here was **verified absent from
[`grafana.com/api/plugins`](https://grafana.com/api/plugins)** (a catalog of 345
plugins: 182 data source, 129 panel, 34 app) as of **2026-06-19**. Each match was
checked against both plugin slug and name.

Proposed plugin ids follow the repo convention `yesoreyeram-<service>-datasource`.
The `API / auth` column is a best-effort note to confirm during implementation.

## Excluded (already on grafana.com)

These adjacent services already have a published data source, so they are **not**
candidates: Jira (`grafana-jira-datasource`), Salesforce
(`grafana-salesforce-datasource`), Zendesk (`grafana-zendesk-datasource`), GitHub,
GitLab, Google Sheets, Google Analytics, Adobe Analytics, ServiceNow, Azure DevOps,
Atlassian Statuspage, Strava, Zoom, Vercel, Netlify, Sentry, PagerDuty, HackerOne,
Firestore.

> Note: **Matomo** has only a _tracking panel_ (`thiagoarrais-matomotracking-panel`),
> not a data source, so a Matomo data source is still novel.

## Waves

- **Wave 1** — best theme fit + simple personal-token REST APIs (recommended first).
- **Wave 2** — strong, on-theme follow-ups.
- **Wave 3** — broader / new categories, or APIs needing OAuth apps.

## No-code DB / Backend

Extends Baserow, NocoDB, Airtable, PocketBase, Appwrite.

| Service  | Proposed id                       | API / auth                         | Wave |
| -------- | --------------------------------- | ---------------------------------- | ---- |
| Directus | `yesoreyeram-directus-datasource` | REST, Bearer token (self-host)     | 1    |
| Grist    | `yesoreyeram-grist-datasource`    | REST, Bearer API key (self-host)   | 1    |
| SeaTable | `yesoreyeram-seatable-datasource` | REST, base API token               | 1    |
| Strapi   | `yesoreyeram-strapi-datasource`   | REST, Bearer API token (self-host) | 2    |
| Teable   | `yesoreyeram-teable-datasource`   | REST, Bearer token (self-host)     | 2    |
| Supabase | `yesoreyeram-supabase-datasource` | PostgREST, apikey/Bearer           | 2    |
| Budibase | `yesoreyeram-budibase-datasource` | REST, API key                      | 3    |
| Xano     | `yesoreyeram-xano-datasource`     | REST, Bearer                       | 3    |
| Rowy     | `yesoreyeram-rowy-datasource`     | REST (Firebase-backed)             | 3    |

> Supabase is Postgres-backed (Grafana already ships a Postgres data source); the
> value here is the PostgREST/REST layer.

## Project management / Issues

Extends Asana, Linear, Plane, Monday, ClickUp.

| Service      | Proposed id                           | API / auth                     | Wave |
| ------------ | ------------------------------------- | ------------------------------ | ---- |
| Trello       | `yesoreyeram-trello-datasource`       | REST, key + token              | 1    |
| Todoist      | `yesoreyeram-todoist-datasource`      | REST v2, Bearer                | 1    |
| Shortcut     | `yesoreyeram-shortcut-datasource`     | REST v3, token header          | 1    |
| YouTrack     | `yesoreyeram-youtrack-datasource`     | REST, Bearer permanent token   | 2    |
| Smartsheet   | `yesoreyeram-smartsheet-datasource`   | REST, Bearer                   | 2    |
| Wrike        | `yesoreyeram-wrike-datasource`        | REST, Bearer / permanent token | 2    |
| Height       | `yesoreyeram-height-datasource`       | REST, API key                  | 3    |
| Productboard | `yesoreyeram-productboard-datasource` | REST, Bearer                   | 3    |
| Aha!         | `yesoreyeram-aha-datasource`          | REST, Bearer                   | 3    |
| Basecamp     | `yesoreyeram-basecamp-datasource`     | REST, OAuth2                   | 3    |
| Teamwork     | `yesoreyeram-teamwork-datasource`     | REST, API key                  | 3    |

## Docs / Notes

Extends Notion.

| Service    | Proposed id                         | API / auth          | Wave |
| ---------- | ----------------------------------- | ------------------- | ---- |
| Coda       | `yesoreyeram-coda-datasource`       | REST, Bearer        | 1    |
| Confluence | `yesoreyeram-confluence-datasource` | REST, token/basic   | 2    |
| Slite      | `yesoreyeram-slite-datasource`      | REST, API key       | 3    |
| Slab       | `yesoreyeram-slab-datasource`       | REST/GraphQL, token | 3    |

## CRM / Sales

Extends HubSpot.

| Service    | Proposed id                         | API / auth            | Wave |
| ---------- | ----------------------------------- | --------------------- | ---- |
| Pipedrive  | `yesoreyeram-pipedrive-datasource`  | REST, api_token       | 1    |
| Attio      | `yesoreyeram-attio-datasource`      | REST, Bearer          | 2    |
| Zoho CRM   | `yesoreyeram-zohocrm-datasource`    | REST, OAuth2          | 3    |
| Close      | `yesoreyeram-close-datasource`      | REST, API key (basic) | 3    |
| Freshsales | `yesoreyeram-freshsales-datasource` | REST, token           | 3    |
| Copper     | `yesoreyeram-copper-datasource`     | REST, API key         | 3    |

## Support / Helpdesk

New category.

| Service    | Proposed id                        | API / auth            | Wave |
| ---------- | ---------------------------------- | --------------------- | ---- |
| Intercom   | `yesoreyeram-intercom-datasource`  | REST, Bearer          | 2    |
| Freshdesk  | `yesoreyeram-freshdesk-datasource` | REST, API key (basic) | 3    |
| Front      | `yesoreyeram-front-datasource`     | REST, Bearer          | 3    |
| Help Scout | `yesoreyeram-helpscout-datasource` | REST, OAuth2 / app    | 3    |

## Forms / Surveys

New category.

| Service      | Proposed id                           | API / auth    | Wave |
| ------------ | ------------------------------------- | ------------- | ---- |
| Typeform     | `yesoreyeram-typeform-datasource`     | REST, Bearer  | 3    |
| Tally        | `yesoreyeram-tally-datasource`        | REST, Bearer  | 3    |
| Jotform      | `yesoreyeram-jotform-datasource`      | REST, API key | 3    |
| SurveyMonkey | `yesoreyeram-surveymonkey-datasource` | REST, OAuth2  | 3    |

## Marketing / Email

New category.

| Service     | Proposed id                         | API / auth        | Wave |
| ----------- | ----------------------------------- | ----------------- | ---- |
| Mailchimp   | `yesoreyeram-mailchimp-datasource`  | REST, API key     | 3    |
| SendGrid    | `yesoreyeram-sendgrid-datasource`   | REST, Bearer      | 3    |
| Brevo       | `yesoreyeram-brevo-datasource`      | REST, api-key     | 3    |
| Klaviyo     | `yesoreyeram-klaviyo-datasource`    | REST, API key     | 3    |
| Customer.io | `yesoreyeram-customerio-datasource` | REST, app API key | 3    |
| ConvertKit  | `yesoreyeram-convertkit-datasource` | REST, API key     | 3    |
| Mailgun     | `yesoreyeram-mailgun-datasource`    | REST, API key     | 3    |

## Payments / Billing

New category.

| Service       | Proposed id                           | API / auth            | Wave |
| ------------- | ------------------------------------- | --------------------- | ---- |
| Stripe        | `yesoreyeram-stripe-datasource`       | REST, secret key      | 3    |
| Paddle        | `yesoreyeram-paddle-datasource`       | REST, Bearer          | 3    |
| Chargebee     | `yesoreyeram-chargebee-datasource`    | REST, API key (basic) | 3    |
| Lemon Squeezy | `yesoreyeram-lemonsqueezy-datasource` | REST, Bearer          | 3    |
| Recurly       | `yesoreyeram-recurly-datasource`      | REST, API key         | 3    |

## Product analytics

New category.

| Service   | Proposed id                        | API / auth             | Wave |
| --------- | ---------------------------------- | ---------------------- | ---- |
| PostHog   | `yesoreyeram-posthog-datasource`   | REST, personal API key | 3    |
| Mixpanel  | `yesoreyeram-mixpanel-datasource`  | REST, service account  | 3    |
| Amplitude | `yesoreyeram-amplitude-datasource` | REST, API key + secret | 3    |
| Plausible | `yesoreyeram-plausible-datasource` | REST, Bearer           | 3    |
| Umami     | `yesoreyeram-umami-datasource`     | REST, token            | 3    |
| Matomo    | `yesoreyeram-matomo-datasource`    | REST, token            | 3    |

## Scheduling

New category.

| Service  | Proposed id                       | API / auth         | Wave |
| -------- | --------------------------------- | ------------------ | ---- |
| Cal.com  | `yesoreyeram-calcom-datasource`   | REST v2, API key   | 3    |
| Calendly | `yesoreyeram-calendly-datasource` | REST, Bearer / PAT | 3    |
| SavvyCal | `yesoreyeram-savvycal-datasource` | REST, Bearer       | 3    |

## How to add a plugin

There is no generator yet, so scaffold a new plugin by copying the structure of an
existing one (e.g. [`grafana-asana-datasource`](./plugins/datasources/yesoreyeram-asana-datasource)):

1. Create `plugins/datasources/grafana-<service>-datasource/` mirroring an existing plugin
   (TypeScript/React frontend + Go backend; `src/`, `pkg/`, `provisioning/`,
   `docker-compose.yaml`, `Magefile.go`).
2. Set the plugin id to `yesoreyeram-<service>-datasource` in `src/plugin.json`
   and the package name in `package.json`.
3. Implement the REST client in `pkg/plugin/` (paginate, flatten nested objects to
   scalars, convert to data frames); keep secrets in `secureJsonData`.
4. Follow that plugin's `AGENTS.md` for architecture conventions and the golden
   data-frame tests.
5. Register the plugin: add a row to the [README](./README.md) Plugins table and
   mount it in the root [`docker-compose.yaml`](./docker-compose.yaml).
6. Run the quality gates: `yarn typecheck && yarn lint && yarn test && go test ./pkg/...`.
