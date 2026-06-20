package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Make sure Datasource implements the required interfaces.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ backend.CallResourceHandler   = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// Datasource is the Airtable backend data source instance. One instance exists
// per configured data source and is reused across queries until settings change.
type Datasource struct {
	settings Settings
	client   *Client

	resourceHandler backend.CallResourceHandler
}

// NewDatasource creates a new Datasource instance.
func NewDatasource(ctx context.Context, s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	settings, err := LoadSettings(s)
	if err != nil {
		return nil, err
	}

	opts, err := s.HTTPClientOptions(ctx)
	if err != nil {
		return nil, err
	}
	httpClient, err := httpclient.New(opts)
	if err != nil {
		return nil, err
	}

	client, err := NewClient(settings, httpClient)
	if err != nil {
		return nil, err
	}

	ds := &Datasource{
		settings: settings,
		client:   client,
	}
	ds.resourceHandler = httpadapter.New(ds.newResourceMux())
	return ds, nil
}

// Dispose is called before creating a new instance to allow plugin authors to
// clean up resources. The SDK-managed http client needs no explicit teardown.
func (d *Datasource) Dispose() {}

// QueryData handles multiple queries and returns multiple responses.
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response := backend.NewQueryDataResponse()
	for _, q := range req.Queries {
		response.Responses[q.RefID] = d.query(ctx, q)
	}
	return response, nil
}

func (d *Datasource) query(ctx context.Context, query backend.DataQuery) backend.DataResponse {
	qm, err := LoadQuery(query.JSON)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "invalid query: "+err.Error())
	}

	switch qm.QueryType {
	case "records", "":
		records, err := d.client.ListRecords(ctx, qm)
		if err != nil {
			log.DefaultLogger.Error("airtable query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
		}
		frame := recordsToFrame(query.RefID, records)
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	case "count":
		count, err := d.client.CountRecords(ctx, qm)
		if err != nil {
			log.DefaultLogger.Error("airtable count query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
		}
		frame := countToFrame(query.RefID, count)
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, "unsupported query type: "+qm.QueryType)
	}
}

// CheckHealth validates connectivity and credentials.
func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settings.BaseURL == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Airtable base URL is not configured",
		}, nil
	}
	if strings.TrimSpace(d.settings.apiToken) == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Airtable API token is not configured (re-enter it; saved secrets are write-only)",
		}, nil
	}

	if err := d.client.Ping(ctx); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to connect to Airtable: " + err.Error(),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to Airtable",
	}, nil
}

// CallResource routes resource calls (used by the frontend to fetch base/table
// lists).
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resourceHandler.CallResource(ctx, req, sender)
}

func (d *Datasource) newResourceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/bases", d.handleBases)
	mux.HandleFunc("/tables", d.handleTables)
	mux.HandleFunc("/fields", d.handleFields)
	mux.HandleFunc("/views", d.handleViews)
	return mux
}

// handleBases serves GET /bases returning every base accessible to the token.
func (d *Datasource) handleBases(rw http.ResponseWriter, r *http.Request) {
	bases, err := d.client.ListBases(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"bases": bases})
}

// handleTables serves GET /tables[?baseId=...] returning the base's tables.
func (d *Datasource) handleTables(rw http.ResponseWriter, r *http.Request) {
	baseID := r.URL.Query().Get("baseId")
	tables, err := d.client.ListTables(r.Context(), baseID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"tables": tables})
}

// handleFields serves GET /fields?tableId=...[&baseId=...] returning the table's fields.
func (d *Datasource) handleFields(rw http.ResponseWriter, r *http.Request) {
	baseID := r.URL.Query().Get("baseId")
	tableID := r.URL.Query().Get("tableId")
	fields, err := d.client.ListFields(r.Context(), baseID, tableID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"fields": fields})
}

// handleViews serves GET /views?tableId=...[&baseId=...] returning the table's views.
func (d *Datasource) handleViews(rw http.ResponseWriter, r *http.Request) {
	baseID := r.URL.Query().Get("baseId")
	tableID := r.URL.Query().Get("tableId")
	views, err := d.client.ListViews(r.Context(), baseID, tableID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"views": views})
}

func writeJSON(rw http.ResponseWriter, status int, body any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	if err := json.NewEncoder(rw).Encode(body); err != nil {
		log.DefaultLogger.Error("failed to write resource response", "error", err)
	}
}
