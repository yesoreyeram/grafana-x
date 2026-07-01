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

type Datasource struct {
	settings Settings
	client   *Client

	resourceHandler backend.CallResourceHandler
}

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

	switch qm.QueryType {
	case queryTypeCount:
		count, err := d.client.CountStories(ctx, qm)
		if err != nil {
			log.DefaultLogger.Error("shortcut count query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "count failed: "+err.Error())
		}
		frame := countToFrame(query.RefID, count)
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	default:
		records, _, err := d.client.ListStories(ctx, qm)
		if err != nil {
			log.DefaultLogger.Error("shortcut stories query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
		}
		if qm.HideSystemFields {
			records = dropSystemFields(records)
		}
		frame := recordsToFrame(query.RefID, records, effectiveFields(qm.Fields))
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	}
}

func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settings.apiToken == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Shortcut API token is not configured",
		}, nil
	}

	if err := d.client.Ping(ctx); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to connect to Shortcut: " + err.Error(),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to Shortcut",
	}, nil
}

func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resourceHandler.CallResource(ctx, req, sender)
}

func (d *Datasource) newResourceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/projects", d.handleProjects)
	mux.HandleFunc("/epics", d.handleEpics)
	mux.HandleFunc("/iterations", d.handleIterations)
	mux.HandleFunc("/members", d.handleMembers)
	mux.HandleFunc("/teams", d.handleTeams)
	mux.HandleFunc("/labels", d.handleLabels)
	mux.HandleFunc("/workflows", d.handleWorkflows)
	mux.HandleFunc("/storyfields", d.handleStoryFields)
	mux.HandleFunc("/storytypes", d.handleStoryTypes)
	return mux
}

func (d *Datasource) handleProjects(rw http.ResponseWriter, r *http.Request) {
	items, err := d.client.ListProjects(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"projects": items})
}

func (d *Datasource) handleEpics(rw http.ResponseWriter, r *http.Request) {
	items, err := d.client.ListEpics(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"epics": items})
}

func (d *Datasource) handleIterations(rw http.ResponseWriter, r *http.Request) {
	items, err := d.client.ListIterations(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"iterations": items})
}

func (d *Datasource) handleMembers(rw http.ResponseWriter, r *http.Request) {
	items, err := d.client.ListMembers(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"members": items})
}

func (d *Datasource) handleTeams(rw http.ResponseWriter, r *http.Request) {
	items, err := d.client.ListTeams(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"teams": items})
}

func (d *Datasource) handleLabels(rw http.ResponseWriter, r *http.Request) {
	items, err := d.client.ListLabels(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"labels": items})
}

func (d *Datasource) handleWorkflows(rw http.ResponseWriter, r *http.Request) {
	states, err := d.client.ListWorkflows(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"workflows": states})
}

// handleStoryFields serves GET /storyfields returning the selectable story field
// names for the fields multi-select.
func (d *Datasource) handleStoryFields(rw http.ResponseWriter, _ *http.Request) {
	writeJSON(rw, http.StatusOK, map[string]any{"fields": StoryFieldNames()})
}

// handleStoryTypes serves GET /storytypes returning the known story types.
func (d *Datasource) handleStoryTypes(rw http.ResponseWriter, _ *http.Request) {
	writeJSON(rw, http.StatusOK, map[string]any{"storyTypes": StoryTypeOptions()})
}

func writeJSON(rw http.ResponseWriter, status int, body any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	if err := json.NewEncoder(rw).Encode(body); err != nil {
		log.DefaultLogger.Error("failed to write resource response", "error", err)
	}
}
