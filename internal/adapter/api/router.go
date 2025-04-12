package api

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/go-payment-gateway/internal/core/service"
	"github.com/go-payment-gateway/internal/core/model"
	"github.com/go-payment-gateway/internal/core/erro"

	"github.com/eliezerraj/go-core/coreJson"

	go_core_observ "github.com/eliezerraj/go-core/observability"
	go_core_tools "github.com/eliezerraj/go-core/tools"
)

var childLogger = log.With().Str("component", "go-payment-gateway").Str("package", "internal.adapter.api").Logger()

var core_json 		coreJson.CoreJson
var core_apiError 	coreJson.APIError
var core_tools 		go_core_tools.ToolsCore
var tracerProvider 	go_core_observ.TracerProvider

type HttpRouters struct {
	workerService 	*service.WorkerService
}

// Above create routers
func NewHttpRouters(workerService *service.WorkerService) HttpRouters {
	childLogger.Info().Str("func","NewHttpRouters").Send()

	return HttpRouters{
		workerService: workerService,
	}
}

// About return a health
func (h *HttpRouters) Health(rw http.ResponseWriter, req *http.Request) {
	childLogger.Info().Str("func","Health").Send()

	json.NewEncoder(rw).Encode(model.MessageRouter{Message: "true"})
}

// About return a live
func (h *HttpRouters) Live(rw http.ResponseWriter, req *http.Request) {
	childLogger.Info().Str("func","Live").Send()

	json.NewEncoder(rw).Encode(model.MessageRouter{Message: "true"})
}

// About show all header received
func (h *HttpRouters) Header(rw http.ResponseWriter, req *http.Request) {
	childLogger.Info().Str("func","Header").Interface("trace-resquest-id", req.Context().Value("trace-request-id")).Send()
	
	json.NewEncoder(rw).Encode(req.Header)
}

// About do payment
func (h *HttpRouters) AddPayment(rw http.ResponseWriter, req *http.Request) error {
	childLogger.Info().Str("func","AddPayment").Interface("trace-resquest-id", req.Context().Value("trace-request-id")).Send()
	
	span := tracerProvider.Span(req.Context(), "adapter.api.AddPayment")
	defer span.End()

	payment := model.Payment{}
	err := json.NewDecoder(req.Body).Decode(&payment)
    if err != nil {
		core_apiError = core_apiError.NewAPIError(err, http.StatusBadRequest)
		return &core_apiError
    }
	defer req.Body.Close()

	res, err := h.workerService.AddPayment(req.Context(), payment)
	if err != nil {
		switch err {
		case erro.ErrNotFound:
			core_apiError = core_apiError.NewAPIError(err, http.StatusNotFound)
		default:
			core_apiError = core_apiError.NewAPIError(err, http.StatusInternalServerError)
		}
		return &core_apiError
	}
	
	return core_json.WriteJSON(rw, http.StatusOK, res)
}

// About add a pix moviment transaction
func (h *HttpRouters) PixTransaction(rw http.ResponseWriter, req *http.Request) error {
	childLogger.Info().Str("func","PixTransaction").Interface("trace-resquest-id", req.Context().Value("trace-request-id")).Send()
	
	span := tracerProvider.Span(req.Context(), "adapter.api.PixTransaction")
	defer span.End()

	pixTransaction := model.PixTransaction{}
	err := json.NewDecoder(req.Body).Decode(&pixTransaction)
    if err != nil {
		core_apiError = core_apiError.NewAPIError(err, http.StatusBadRequest)
		return &core_apiError
    }
	defer req.Body.Close()

	res, err := h.workerService.PixTransaction(req.Context(), pixTransaction)
	if err != nil {
		switch err {
		case erro.ErrNotFound:
			core_apiError = core_apiError.NewAPIError(err, http.StatusNotFound)
		default:
			core_apiError = core_apiError.NewAPIError(err, http.StatusInternalServerError)
		}
		return &core_apiError
	}
	
	return core_json.WriteJSON(rw, http.StatusOK, res)
}