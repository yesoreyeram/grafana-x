---
"yesoreyeram-vizard-panel": minor
---

Add a Grafana panel plugin that renders any data frame as a Vega-Lite
visualization through a visual builder (frontend only). Covers the single-view
Vega-Lite grammar — all marks, every encoding channel, a transform pipeline, and
config/spec-override JSON escape hatches for full-grammar coverage. Converts all
data-plane formats (time series wide/multi/long, numeric wide/multi/long, logs,
tables) to inline Vega-Lite data, with smart defaults from the detected shape,
Grafana theming, responsive sizing, and a hardened render core (remote loading
blocked, `url`/`href`/`usermeta` stripped, CSP-safe `ast` interpreter,
export-only actions). The spec pipeline is mode-pluggable so raw-grammar JSON and
the vega-lite-api can be added later without changing data/theme/security/sizing.
