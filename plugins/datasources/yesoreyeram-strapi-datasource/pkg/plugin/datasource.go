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
	case "records", "":
		records, err := d.client.ListRecords(ctx, qm)
		if err != nil {
			log.DefaultLogger.Error("strapi query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
		}
		if qm.HideSystemFields {
			records = dropSystemFields(records)
		}
		frame := recordsToFrame(query.RefID, records)
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	case "count":
		count, err := d.client.CountRecords(ctx, qm)
		if err != nil {
			log.DefaultLogger.Error("strapi count query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
		}
		frame := countToFrame(query.RefID, count)
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, "unsupported query type: "+qm.QueryType)
	}
}

func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settings.BaseURL == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Strapi base URL is not configured",
		}, nil
	}
	if strings.TrimSpace(d.settings.apiToken) == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Strapi API token is not configured (re-enter it; saved secrets are write-only)",
		}, nil
	}

	if err := d.client.Ping(ctx, d.settings.DefaultContentTypeID); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to connect to Strapi: " + err.Error(),
		}, nil
	}

	msg := "Successfully connected to Strapi"
	if d.settings.DefaultContentTypeID == "" {
		msg += " (base URL reachable; set a Default Content Type to fully validate the API token)"
	}
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: msg,
	}, nil
}

func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resourceHandler.CallResource(ctx, req, sender)
}

func (d *Datasource) newResourceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/content-types", d.handleContentTypes)
	mux.HandleFunc("/fields", d.handleFields)
	return mux
}

type contentTypeDisplay struct {
	UID         string `json:"uid"`
	APIID       string `json:"apiID"`
	DisplayName string `json:"displayName"`
	PluralName  string `json:"pluralName"`
}

// handleContentTypes serves GET /content-types. Discovery uses the
// content-type-builder admin endpoint, which requires an admin JWT and is NOT
// reachable with a regular API token; the handler therefore degrades gracefully
// (logs and returns an empty list) so the query editor stays usable via
// free-text entry of the content type plural API id.
func (d *Datasource) handleContentTypes(rw http.ResponseWriter, r *http.Request) {
	contentTypes, err := d.client.ListContentTypes(r.Context())
	if err != nil {
		log.DefaultLogger.Debug("strapi content-type discovery unavailable (expected with an API token)", "error", err)
		writeJSON(rw, http.StatusOK, map[string]any{"contentTypes": []contentTypeDisplay{}})
		return
	}

	out := make([]contentTypeDisplay, 0, len(contentTypes))
	for _, ct := range contentTypes {
		out = append(out, contentTypeDisplay{
			UID:         ct.UID,
			APIID:       ct.Schema.SingularName,
			DisplayName: ct.Schema.DisplayName,
			PluralName:  ct.Schema.PluralName,
		})
	}
	writeJSON(rw, http.StatusOK, map[string]any{"contentTypes": out})
}

// handleFields serves GET /fields?contentTypeId=... It degrades gracefully like
// handleContentTypes, returning an empty list when schema discovery is
// unavailable so users can still type field names directly.
func (d *Datasource) handleFields(rw http.ResponseWriter, r *http.Request) {
	contentTypeID := r.URL.Query().Get("contentTypeId")
	fields, err := d.client.ListFields(r.Context(), contentTypeID)
	if err != nil {
		log.DefaultLogger.Debug("strapi field discovery unavailable (expected with an API token)", "error", err)
		writeJSON(rw, http.StatusOK, map[string]any{"fields": []FieldInfo{}})
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
