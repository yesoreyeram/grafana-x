package plugin

import (
	"context"
	"encoding/json"
	"net/http"

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

// Datasource is the Pipedrive backend data source instance. One instance exists
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

// Dispose is called before creating a new instance. The SDK-managed http client
// needs no explicit teardown.
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

	switch {
	case qm.QueryType == queryTypeCount:
		return d.queryCount(ctx, query.RefID, qm)
	case isEntityQuery(qm.QueryType):
		return d.queryEntity(ctx, query.RefID, qm)
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, "unsupported query type: "+qm.QueryType)
	}
}

func (d *Datasource) queryEntity(ctx context.Context, refID string, qm QueryModel) backend.DataResponse {
	records, err := d.client.ListRecords(ctx, qm)
	if err != nil {
		log.DefaultLogger.Error("pipedrive query failed", "refID", refID, "queryType", qm.QueryType, "error", err)
		return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
	}
	records = applyFilters(records, qm.FilterGroups)
	if qm.HideSystemFields {
		records = dropSystemFields(records)
	}
	frame := recordsToFrame(refID, records, qm.Fields)
	return backend.DataResponse{Frames: []*data.Frame{frame}}
}

func (d *Datasource) queryCount(ctx context.Context, refID string, qm QueryModel) backend.DataResponse {
	count, err := d.client.CountRecords(ctx, qm)
	if err != nil {
		log.DefaultLogger.Error("pipedrive count query failed", "refID", refID, "error", err)
		return backend.ErrDataResponse(backend.StatusInternal, "count query failed: "+err.Error())
	}
	frame := countToFrame(refID, count)
	return backend.DataResponse{Frames: []*data.Frame{frame}}
}

// CheckHealth validates connectivity and credentials.
func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settings.CompanyDomain == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Pipedrive company domain is not configured",
		}, nil
	}
	if d.settings.credential() == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Pipedrive credential (API token or OAuth token) is not configured",
		}, nil
	}
	if err := d.client.Ping(ctx); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to connect to Pipedrive: " + err.Error(),
		}, nil
	}
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to Pipedrive",
	}, nil
}

// CallResource routes resource calls (used by the frontend to populate the
// pipeline/stage/user dropdowns).
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resourceHandler.CallResource(ctx, req, sender)
}

func (d *Datasource) newResourceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/pipelines", d.handlePipelines)
	mux.HandleFunc("/stages", d.handleStages)
	mux.HandleFunc("/users", d.handleUsers)
	return mux
}

func (d *Datasource) handlePipelines(rw http.ResponseWriter, r *http.Request) {
	pipelines, err := d.client.ListPipelines(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"pipelines": pipelines})
}

func (d *Datasource) handleStages(rw http.ResponseWriter, r *http.Request) {
	pipelineID := r.URL.Query().Get("pipelineId")
	stages, err := d.client.ListStages(r.Context(), pipelineID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"stages": stages})
}

func (d *Datasource) handleUsers(rw http.ResponseWriter, r *http.Request) {
	users, err := d.client.ListUsers(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"users": users})
}

func writeJSON(rw http.ResponseWriter, status int, body any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	if err := json.NewEncoder(rw).Encode(body); err != nil {
		log.DefaultLogger.Error("failed to write resource response", "error", err)
	}
}
