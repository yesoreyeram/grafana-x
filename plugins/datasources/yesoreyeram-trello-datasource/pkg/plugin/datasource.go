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
	settings        Settings
	client          *Client
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

	cq := CardsQuery{
		BoardId:       qm.BoardId,
		ListId:        qm.ListId,
		CardFilter:    qm.CardFilter,
		MemberIds:     qm.MemberIds,
		LabelIds:      qm.LabelIds,
		Fields:        qm.Fields,
		Limit:         qm.Limit,
		CreatedMode:   qm.CreatedMode,
		CreatedAfter:  qm.CreatedAfter,
		CreatedBefore: qm.CreatedBefore,
		TimeRange:     qm.TimeRange,
	}

	switch qm.QueryType {
	case queryTypeCards:
		cards, err := d.client.ListCards(ctx, cq)
		if err != nil {
			log.DefaultLogger.Error("trello cards query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
		}
		frame := cardsToFrame(query.RefID, cards)
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	case queryTypeCount:
		count, err := d.client.CountCards(ctx, cq)
		if err != nil {
			log.DefaultLogger.Error("trello count query failed", "refID", query.RefID, "error", err)
			return backend.ErrDataResponse(backend.StatusInternal, "query failed: "+err.Error())
		}
		frame := countToFrame(query.RefID, count)
		return backend.DataResponse{Frames: []*data.Frame{frame}}
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, "unsupported query type: "+qm.QueryType)
	}
}

func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if d.settings.apiKey == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Trello API key is not configured",
		}, nil
	}
	if d.settings.apiToken == "" {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Trello API token is not configured",
		}, nil
	}

	if err := d.client.Ping(ctx); err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Failed to connect to Trello: " + err.Error(),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to Trello",
	}, nil
}

func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	return d.resourceHandler.CallResource(ctx, req, sender)
}

func (d *Datasource) newResourceMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/boards", d.handleBoards)
	mux.HandleFunc("/lists", d.handleLists)
	mux.HandleFunc("/members", d.handleMembers)
	mux.HandleFunc("/labels", d.handleLabels)
	return mux
}

func (d *Datasource) handleBoards(rw http.ResponseWriter, r *http.Request) {
	boards, err := d.client.ListBoards(r.Context())
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"boards": boards})
}

func (d *Datasource) handleLists(rw http.ResponseWriter, r *http.Request) {
	boardID := r.URL.Query().Get("boardId")
	if boardID == "" {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "boardId is required"})
		return
	}
	lists, err := d.client.ListLists(r.Context(), boardID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"lists": lists})
}

func (d *Datasource) handleMembers(rw http.ResponseWriter, r *http.Request) {
	boardID := r.URL.Query().Get("boardId")
	if boardID == "" {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "boardId is required"})
		return
	}
	members, err := d.client.ListMembers(r.Context(), boardID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"members": members})
}

func (d *Datasource) handleLabels(rw http.ResponseWriter, r *http.Request) {
	boardID := r.URL.Query().Get("boardId")
	if boardID == "" {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "boardId is required"})
		return
	}
	labels, err := d.client.ListLabels(r.Context(), boardID)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]any{"labels": labels})
}

func writeJSON(rw http.ResponseWriter, status int, body any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	if err := json.NewEncoder(rw).Encode(body); err != nil {
		log.DefaultLogger.Error("failed to write resource response", "error", err)
	}
}
