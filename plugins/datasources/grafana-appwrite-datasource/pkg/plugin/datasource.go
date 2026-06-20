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

// Datasource is the Appwrite backend data source instance. One instance exists
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
	case "documents", "":
		documents, err := d.client.ListDocuments(ctx, qm)
		if err != nil {
			log.DefaultLogger.Error("appwrite query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
		}
		frame := documentsToFrame(query.RefID, documents, qm.HideSystemFields)
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	case "count":
		count, err := d.client.CountDocuments(ctx, qm)
		if err != nil {
			log.DefaultLogger.Error("appwrite count query failed", "refID", query.RefID, "error", err)
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
	if d.settings.Endpoint == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Appwrite endpoint is not configured",
		}, nil
	}
	if strings.TrimSpace(d.settings.ProjectID) == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Appwrite project id is not configured",
		}, nil
	}
	if strings.TrimSpace(d.settings.apiKey) == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Appwrite API key is not configured (re-enter it; saved secrets are write-only)",
		}, nil
	}

	if err := d.client.Ping(ctx); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to connect to Appwrite: " + err.Error(),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to Appwrite",
	}, nil
}

// CallResource routes resource calls (used by the frontend to fetch
// database/collection/attribute lists).
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resourceHandler.CallResource(ctx, req, sender)
}

func (d *Datasource) newResourceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/databases", d.handleDatabases)
	mux.HandleFunc("/collections", d.handleCollections)
	mux.HandleFunc("/attributes", d.handleAttributes)
	return mux
}

// handleDatabases serves GET /databases returning every database in the project.
func (d *Datasource) handleDatabases(rw http.ResponseWriter, r *http.Request) {
	databases, err := d.client.ListDatabases(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"databases": databases})
}

// handleCollections serves GET /collections[?databaseId=...] returning the
// database's collections.
func (d *Datasource) handleCollections(rw http.ResponseWriter, r *http.Request) {
	databaseID := r.URL.Query().Get("databaseId")
	collections, err := d.client.ListCollections(r.Context(), databaseID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"collections": collections})
}

// handleAttributes serves GET /attributes?collectionId=...[&databaseId=...]
// returning the collection's attributes.
func (d *Datasource) handleAttributes(rw http.ResponseWriter, r *http.Request) {
	databaseID := r.URL.Query().Get("databaseId")
	collectionID := r.URL.Query().Get("collectionId")
	attributes, err := d.client.ListAttributes(r.Context(), databaseID, collectionID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"attributes": attributes})
}

func writeJSON(rw http.ResponseWriter, status int, body any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	if err := json.NewEncoder(rw).Encode(body); err != nil {
		log.DefaultLogger.Error("failed to write resource response", "error", err)
	}
}
