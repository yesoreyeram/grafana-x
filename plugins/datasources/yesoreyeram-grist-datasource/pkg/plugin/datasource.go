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

	switch qm.QueryType {
	case QueryTypeRecords, "":
		records, dateCols, err := d.client.ListRecords(ctx, qm)
		if err != nil {
			log.DefaultLogger.Error("grist query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
		}
		frame := recordsToFrame(query.RefID, records, dateCols)
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	case QueryTypeCount:
		count, err := d.client.CountRecords(ctx, qm)
		if err != nil {
			log.DefaultLogger.Error("grist count query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
		}
		frame := countToFrame(query.RefID, count)
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	case QueryTypeSQL:
		records, err := d.client.RunSQL(ctx, qm)
		if err != nil {
			log.DefaultLogger.Error("grist sql query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
		}
		// Raw SQL has no column metadata, so Date/DateTime columns surface as
		// epoch-seconds numbers (documented limitation).
		frame := recordsToFrame(query.RefID, records, nil)
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, "unsupported query type: "+qm.QueryType)
	}
}

func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settings.BaseURL == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Grist base URL is not configured",
		}, nil
	}
	if d.settings.apiKey == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Grist API key is not configured (re-enter it; saved secrets are write-only)",
		}, nil
	}

	if err := d.client.Ping(ctx); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to connect to Grist: " + err.Error(),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to Grist",
	}, nil
}

func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resourceHandler.CallResource(ctx, req, sender)
}

func (d *Datasource) newResourceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/docs", d.handleDocs)
	mux.HandleFunc("/tables", d.handleTables)
	mux.HandleFunc("/fields", d.handleFields)
	return mux
}

func (d *Datasource) handleDocs(rw http.ResponseWriter, r *http.Request) {
	docs, err := d.client.ListDocs(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"docs": docs})
}

func (d *Datasource) handleTables(rw http.ResponseWriter, r *http.Request) {
	docID := r.URL.Query().Get("docId")
	tables, err := d.client.ListTables(r.Context(), docID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"tables": tables})
}

func (d *Datasource) handleFields(rw http.ResponseWriter, r *http.Request) {
	docID := r.URL.Query().Get("docId")
	tableID := r.URL.Query().Get("tableId")
	fields, err := d.client.ListFields(r.Context(), docID, tableID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"fields": fields})
}

func writeJSON(rw http.ResponseWriter, status int, body any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	if err := json.NewEncoder(rw).Encode(body); err != nil {
		log.DefaultLogger.Error("failed to write resource response", "error", err)
	}
}
