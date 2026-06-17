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

// Datasource is the Linear backend data source instance. One instance exists per
// configured data source and is reused across queries until settings change.
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
	// Pass the panel time range so date filters set to "dashboard" mode can use it.
	qm.TimeRange = query.TimeRange

	records, err := d.client.ListRecords(ctx, qm)
	if err != nil {
		log.DefaultLogger.Error("linear query failed", "refID", query.RefID, "error", err)
		return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
	}
	frame := recordsToFrame(query.RefID, records)
	return backend.DataResponse{Frames: []*data.Frame{frame}}
}

// CheckHealth validates connectivity and credentials.
func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settings.BaseURL == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Linear GraphQL URL is not configured",
		}, nil
	}
	if token, _ := d.settings.credential(); token == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Linear credential (API key or OAuth token) is not configured",
		}, nil
	}

	if err := d.client.Ping(ctx); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to connect to Linear: " + err.Error(),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to Linear",
	}, nil
}

// CallResource routes resource calls (used by the frontend to fetch teams and
// workflow states for the query editor dropdowns).
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resourceHandler.CallResource(ctx, req, sender)
}

func (d *Datasource) newResourceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/teams", d.handleTeams)
	mux.HandleFunc("/states", d.handleStates)
	mux.HandleFunc("/labels", d.handleLabels)
	mux.HandleFunc("/projects", d.handleProjects)
	mux.HandleFunc("/users", d.handleUsers)
	mux.HandleFunc("/issuefields", d.handleIssueFields)
	return mux
}

// handleTeams serves GET /teams returning the workspace teams.
func (d *Datasource) handleTeams(rw http.ResponseWriter, r *http.Request) {
	teams, err := d.client.ListTeams(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"teams": teams})
}

// handleStates serves GET /states?teamId=... returning workflow states,
// optionally restricted to a team.
func (d *Datasource) handleStates(rw http.ResponseWriter, r *http.Request) {
	teamID := r.URL.Query().Get("teamId")
	states, err := d.client.ListStates(r.Context(), teamID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"states": states})
}

// handleLabels serves GET /labels?teamId=... returning labels, optionally
// restricted to a team.
func (d *Datasource) handleLabels(rw http.ResponseWriter, r *http.Request) {
	teamID := r.URL.Query().Get("teamId")
	labels, err := d.client.ListLabels(r.Context(), teamID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"labels": labels})
}

// handleProjects serves GET /projects returning the workspace projects.
func (d *Datasource) handleProjects(rw http.ResponseWriter, r *http.Request) {
	projects, err := d.client.ListProjects(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"projects": projects})
}

// handleUsers serves GET /users returning the workspace users.
func (d *Datasource) handleUsers(rw http.ResponseWriter, r *http.Request) {
	users, err := d.client.ListUsers(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"users": users})
}

// handleIssueFields serves GET /issuefields returning the selectable issue field
// names for the fields multi-select.
func (d *Datasource) handleIssueFields(rw http.ResponseWriter, _ *http.Request) {
	writeJSON(rw, http.StatusOK, map[string]any{"fields": IssueFieldNames()})
}

func writeJSON(rw http.ResponseWriter, status int, body any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	if err := json.NewEncoder(rw).Encode(body); err != nil {
		log.DefaultLogger.Error("failed to write resource response", "error", err)
	}
}
