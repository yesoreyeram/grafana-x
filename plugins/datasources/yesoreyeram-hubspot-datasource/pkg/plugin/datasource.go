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

var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ backend.CallResourceHandler   = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// Datasource is the HubSpot backend data source instance.
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

func (d *Datasource) Dispose() {}

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
	qm.TimeRange = query.TimeRange

	records, err := d.client.ListRecords(ctx, qm)
	if err != nil {
		log.DefaultLogger.Error("hubspot query failed", "refID", query.RefID, "error", err)
		return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
	}
	frame := recordsToFrame(query.RefID, records, qm.Properties)
	return backend.DataResponse{Frames: []*data.Frame{frame}}
}

func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settings.BaseURL == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "HubSpot API URL is not configured",
		}, nil
	}
	if d.settings.credential() == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "HubSpot access token is not configured",
		}, nil
	}
	if err := d.client.Ping(ctx); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to connect to HubSpot: " + err.Error(),
		}, nil
	}
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to HubSpot",
	}, nil
}

func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resourceHandler.CallResource(ctx, req, sender)
}

func (d *Datasource) newResourceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/properties", d.handleProperties)
	mux.HandleFunc("/pipelines", d.handlePipelines)
	mux.HandleFunc("/owners", d.handleOwners)
	mux.HandleFunc("/search_operators", d.handleSearchOperators)
	mux.HandleFunc("/object_types", d.handleObjectTypes)
	return mux
}

// handleProperties serves GET /properties?objectType=contacts returning property definitions.
func (d *Datasource) handleProperties(rw http.ResponseWriter, r *http.Request) {
	objectType := r.URL.Query().Get("objectType")
	props, err := d.client.ListPropertiesForObject(r.Context(), objectType)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"properties": props})
}

// handlePipelines serves GET /pipelines?objectType=deals returning pipeline definitions.
func (d *Datasource) handlePipelines(rw http.ResponseWriter, r *http.Request) {
	objectType := r.URL.Query().Get("objectType")
	pipelines, err := d.client.ListPipelinesForObject(r.Context(), objectType)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"pipelines": pipelines})
}

// handleOwners serves GET /owners returning HubSpot account owners.
func (d *Datasource) handleOwners(rw http.ResponseWriter, r *http.Request) {
	owners, err := d.client.ListOwners(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"owners": owners})
}

// handleSearchOperators serves GET /search_operators returning supported operators.
func (d *Datasource) handleSearchOperators(rw http.ResponseWriter, _ *http.Request) {
	writeJSON(rw, http.StatusOK, map[string]any{"operators": SearchOperators()})
}

// handleObjectTypes serves GET /object_types returning the list of CRM object types.
func (d *Datasource) handleObjectTypes(rw http.ResponseWriter, _ *http.Request) {
	types := make([]string, 0, len(objectTypeToAPIPath))
	for k := range objectTypeToAPIPath {
		types = append(types, k)
	}
	writeJSON(rw, http.StatusOK, map[string]any{"objectTypes": types})
}

func writeJSON(rw http.ResponseWriter, status int, body any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	if err := json.NewEncoder(rw).Encode(body); err != nil {
		log.DefaultLogger.Error("failed to write resource response", "error", err)
	}
}
